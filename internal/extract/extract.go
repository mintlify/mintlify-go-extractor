package extract

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Options struct {
	Dir      string
	Patterns []string
	Exclude  []string
	Comments CommentSyntax
	GOOS     string
	GOARCH   string
	Tags     []string
	Internal bool
	// NoParent requires go.mod in Dir itself. Without it the module root is
	// found the way the go command finds it, by walking up from Dir, which a
	// shared worker must not do: the first go.mod above a checkout could belong
	// to someone else and would put every sibling checkout in the artifact.
	NoParent  bool
	Private   []string
	Generator Generator
}

type source struct {
	dir        string
	relDir     string
	importPath string
	name       string
	fset       *token.FileSet
	files      []*ast.File
}

var modulePattern = regexp.MustCompile(`(?m)^\s*module\s+("[^"]+"|\S+)`)

func Extract(opts Options) (*Artifact, error) {
	if opts.Comments == "" {
		opts.Comments = CommentsMarkdown
	}
	root, modulePath, err := findModule(opts.Dir, opts.NoParent)
	if err != nil {
		return nil, err
	}

	ctx, err := buildContext(opts)
	if err != nil {
		return nil, err
	}

	patterns := opts.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	include := compilePatterns(patterns, modulePath)
	exclude := compilePatterns(opts.Exclude, modulePath)

	dirs, err := packageDirs(root)
	if err != nil {
		return nil, err
	}

	names := map[string]string{}
	var sources []*source
	for _, dir := range dirs {
		relDir, importPath := locate(root, dir, modulePath)
		if !include(importPath) || exclude(importPath) || (!opts.Internal && isInternal(relDir)) {
			if name := packageName(ctx, dir); name != "" {
				names[importPath] = name
			}
			continue
		}
		src, err := loadSource(ctx, dir, relDir, importPath)
		if err != nil {
			return nil, err
		}
		if src == nil {
			continue
		}
		names[importPath] = src.name
		if src.name != "main" {
			sources = append(sources, src)
		}
	}

	artifact := &Artifact{
		SchemaVersion: SchemaVersion,
		Generator:     opts.Generator,
		Module:        modulePath,
		GOOS:          opts.GOOS,
		GOARCH:        opts.GOARCH,
		CommentSyntax: opts.Comments,
		Packages:      []Package{},
	}
	for _, src := range sources {
		pkg, err := extractPackage(src, modulePath, names, opts)
		if err != nil {
			return nil, err
		}
		artifact.Packages = append(artifact.Packages, pkg)
	}
	return artifact, nil
}

func findModule(dir string, noParent bool) (string, string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}
	for current := abs; ; current = filepath.Dir(current) {
		goMod := filepath.Join(current, "go.mod")
		data, ok, err := readRegular(goMod, maxGoModBytes)
		if noParent && errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("no go.mod in %s (-no-parent)", abs)
		}
		// A go.mod that exists but is a symlink, device, FIFO, hard link, or
		// oversized is refused outright rather than skipped: walking past it
		// could land on a go.mod above the checkout.
		if err == nil && !ok {
			return "", "", fmt.Errorf("%s: not a regular file of at most %d bytes", goMod, maxGoModBytes)
		}
		if err == nil {
			match := modulePattern.FindSubmatch(data)
			if match == nil {
				return "", "", fmt.Errorf("%s: no module directive", goMod)
			}
			modulePath := string(match[1])
			if unquoted, err := strconv.Unquote(modulePath); err == nil {
				modulePath = unquoted
			}
			return current, modulePath, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", "", err
		}
		if filepath.Dir(current) == current {
			return "", "", fmt.Errorf("no go.mod found in %s or any parent directory", abs)
		}
	}
}

func packageDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if current != root {
			name := entry.Name()
			if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
				return filepath.SkipDir
			}
		}
		dirs = append(dirs, current)
		return nil
	})
	return dirs, err
}

func isInternal(relDir string) bool {
	for _, element := range strings.Split(relDir, "/") {
		if element == "internal" {
			return true
		}
	}
	return false
}

func locate(root, dir, modulePath string) (string, string) {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return ".", modulePath
	}
	rel = filepath.ToSlash(rel)
	return rel, path.Join(modulePath, rel)
}

// releaseTagHorizon bounds the go1.N release tags the build context satisfies.
// Pinning the list (instead of inheriting the running toolchain's) keeps file
// selection the same whatever Go built the extractor, and includes files gated
// on newer releases, which are still part of the module's API.
const releaseTagHorizon = 60

var knownGOOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true,
	"illumos": true, "ios": true, "js": true, "linux": true, "netbsd": true,
	"openbsd": true, "plan9": true, "solaris": true, "wasip1": true, "windows": true,
}

var knownGOARCH = map[string]bool{
	"386": true, "amd64": true, "arm": true, "arm64": true, "loong64": true,
	"mips": true, "mips64": true, "mips64le": true, "mipsle": true, "ppc64": true,
	"ppc64le": true, "riscv64": true, "s390x": true, "wasm": true,
}

// buildContext is a fixed context that depends only on the options, never on
// the host: no cgo, no host ToolTags, and a pinned ReleaseTags list.
func buildContext(opts Options) (build.Context, error) {
	if !knownGOOS[opts.GOOS] {
		return build.Context{}, fmt.Errorf("unknown GOOS %q", opts.GOOS)
	}
	if !knownGOARCH[opts.GOARCH] {
		return build.Context{}, fmt.Errorf("unknown GOARCH %q", opts.GOARCH)
	}
	ctx := build.Default
	ctx.GOOS = opts.GOOS
	ctx.GOARCH = opts.GOARCH
	ctx.CgoEnabled = false
	ctx.BuildTags = append([]string(nil), opts.Tags...)
	ctx.ToolTags = nil
	ctx.ReleaseTags = make([]string, 0, releaseTagHorizon)
	for minor := 1; minor <= releaseTagHorizon; minor++ {
		ctx.ReleaseTags = append(ctx.ReleaseTags, fmt.Sprintf("go1.%d", minor))
	}
	return ctx, nil
}

func goFiles(ctx build.Context, dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasSuffix(name, ".go") {
			continue
		}
		match, err := ctx.MatchFile(dir, name)
		if err != nil {
			return nil, err
		}
		if match {
			files = append(files, name)
		}
	}
	return files, nil
}

func importsC(file *ast.File) bool {
	for _, imp := range file.Imports {
		if imp.Path.Value == `"C"` {
			return true
		}
	}
	return false
}

func packageName(ctx build.Context, dir string) string {
	files, err := goFiles(ctx, dir)
	if err != nil {
		return ""
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, ok, err := readRegular(filepath.Join(dir, name), maxSourceBytes)
		if err != nil || !ok {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, data, parser.PackageClauseOnly)
		if err == nil {
			return file.Name.Name
		}
	}
	return ""
}

func loadSource(ctx build.Context, dir, relDir, importPath string) (*source, error) {
	names, err := goFiles(ctx, dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var files, tests []*ast.File
	pkgName, pkgFile := "", ""
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, ok, err := readRegular(path, maxSourceBytes)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		file, err := parser.ParseFile(fset, path, data, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		if !ctx.CgoEnabled && importsC(file) {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			tests = append(tests, file)
			continue
		}
		if pkgName != "" && file.Name.Name != pkgName {
			return nil, fmt.Errorf("%s: found packages %s (%s) and %s (%s)", relDir, pkgName, pkgFile, file.Name.Name, name)
		}
		pkgName, pkgFile = file.Name.Name, name
		files = append(files, file)
	}
	if pkgName == "" {
		return nil, nil
	}
	for _, test := range tests {
		if test.Name.Name == pkgName || test.Name.Name == pkgName+"_test" {
			files = append(files, test)
		}
	}
	return &source{dir: dir, relDir: relDir, importPath: importPath, name: pkgName, fset: fset, files: files}, nil
}

// maxReadmeBytes bounds the README carried in the artifact; maxSourceBytes
// bounds a single Go file and maxGoModBytes the go.mod.
const (
	maxReadmeBytes = 1 << 20
	maxSourceBytes = 64 << 20
	maxGoModBytes  = 1 << 20
)

// readReadme returns the Markdown README in dir, if any, the way pkg.go.dev
// picks one: README.md first, then README.markdown, then README. Only plain
// files are read, never symlinks or hard links, so a README cannot point
// outside the module.
func readReadme(dir string) (string, error) {
	for _, name := range []string{"README.md", "README.markdown", "README"} {
		data, ok, err := readRegular(filepath.Join(dir, name), maxReadmeBytes)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		return strings.TrimRight(string(data), "\n"), nil
	}
	return "", nil
}

// readRegular reads a file of at most limit bytes that is a regular file, not
// a symlink, and not hard-linked elsewhere. ok is false when the file is not
// such a file and should be skipped. The checks run on the opened handle, so
// a path swapped between the check and the read is caught, and the size cap
// holds even if the file grows.
func readRegular(path string, limit int64) ([]byte, bool, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, false, err
	}
	if !before.Mode().IsRegular() {
		return nil, false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() || !os.SameFile(before, info) || linkCount(info) > 1 || info.Size() > limit {
		return nil, false, nil
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return nil, false, nil
	}
	return data, true, nil
}

func extractPackage(src *source, modulePath string, names map[string]string, opts Options) (Package, error) {
	dpkg, err := doc.NewFromFiles(src.fset, src.files, src.importPath)
	if err != nil {
		return Package{}, err
	}
	e := newExtractor(src, dpkg, modulePath, names, opts)
	pkgDoc, pkgDeprecated := e.render(dpkg.Doc)
	readme, err := readReadme(src.dir)
	if err != nil {
		return Package{}, err
	}
	pkg := Package{
		ImportPath: src.importPath,
		Name:       dpkg.Name,
		Dir:        src.relDir,
		Synopsis:   dpkg.Synopsis(dpkg.Doc),
		Doc:        pkgDoc,
		Readme:     readme,
		Deprecated: pkgDeprecated,
		Consts:     e.values(dpkg.Consts),
		Vars:       e.values(dpkg.Vars),
		Examples:   e.examples(dpkg.Examples),
	}
	for _, fn := range dpkg.Funcs {
		pkg.Funcs = append(pkg.Funcs, e.function(fn))
	}
	for _, typ := range dpkg.Types {
		pkg.Types = append(pkg.Types, e.typ(typ))
	}
	return pkg, nil
}
