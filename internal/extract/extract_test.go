package extract

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func testOptions(module string) Options {
	return Options{
		Dir:       filepath.Join("testdata", "modules", module),
		GOOS:      "linux",
		GOARCH:    "amd64",
		Generator: Generator{Name: "mintlify-go-extractor", Version: "test", Go: "go-test"},
	}
}

func indentJSON(t *testing.T, artifact *Artifact) []byte {
	t.Helper()
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func mustExtract(t *testing.T, opts Options) *Artifact {
	t.Helper()
	artifact, err := Extract(opts)
	if err != nil {
		t.Fatal(err)
	}
	return artifact
}

func TestGolden(t *testing.T) {
	cases := []struct {
		name   string
		module string
		modify func(*Options)
	}{
		{name: "basic", module: "basic"},
		{name: "basic-godoc", module: "basic", modify: func(o *Options) { o.Comments = CommentsGodoc }},
		{name: "generics", module: "generics"},
		{name: "privacy", module: "privacy", modify: func(o *Options) { o.Private = []string{"corp.example"} }},
		{name: "hostile", module: "hostile"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := testOptions(tc.module)
			if tc.modify != nil {
				tc.modify(&opts)
			}
			got := indentJSON(t, mustExtract(t, opts))
			golden := filepath.Join("testdata", "golden", tc.name+".json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("artifact differs from %s; run go test ./internal/extract -update and review the diff", golden)
			}
		})
	}
}

func importPaths(artifact *Artifact) []string {
	var paths []string
	for _, pkg := range artifact.Packages {
		paths = append(paths, pkg.ImportPath)
	}
	return paths
}

func TestPackageSelection(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*Options)
		want   []string
	}{
		{name: "default", want: []string{"example.com/acme", "example.com/acme/legacy", "example.com/acme/option"}},
		{name: "internal", modify: func(o *Options) { o.Internal = true }, want: []string{"example.com/acme", "example.com/acme/internal/secret", "example.com/acme/legacy", "example.com/acme/option"}},
		{name: "pattern", modify: func(o *Options) { o.Patterns = []string{"./option"} }, want: []string{"example.com/acme/option"}},
		{name: "import path pattern", modify: func(o *Options) { o.Patterns = []string{"example.com/acme/..."} }, want: []string{"example.com/acme", "example.com/acme/legacy", "example.com/acme/option"}},
		{name: "exclude", modify: func(o *Options) { o.Exclude = []string{"./legacy", "./option/..."} }, want: []string{"example.com/acme"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := testOptions("basic")
			if tc.modify != nil {
				tc.modify(&opts)
			}
			if got := importPaths(mustExtract(t, opts)); !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func typeNames(artifact *Artifact, importPath string) []string {
	var names []string
	for _, pkg := range artifact.Packages {
		if pkg.ImportPath != importPath {
			continue
		}
		for _, typ := range pkg.Types {
			names = append(names, typ.Name)
		}
	}
	return names
}

func funcNames(artifact *Artifact, importPath string) []string {
	var names []string
	for _, pkg := range artifact.Packages {
		if pkg.ImportPath != importPath {
			continue
		}
		for _, fn := range pkg.Funcs {
			names = append(names, fn.Name)
		}
	}
	return names
}

func TestBuildContext(t *testing.T) {
	linux := mustExtract(t, testOptions("basic"))
	if slices.Contains(funcNames(linux, "example.com/acme"), "WindowsOnly") {
		t.Error("linux artifact includes a windows-only function")
	}
	opts := testOptions("basic")
	opts.GOOS = "windows"
	windows := mustExtract(t, opts)
	if !slices.Contains(funcNames(windows, "example.com/acme"), "WindowsOnly") {
		t.Error("windows artifact is missing the windows-only function")
	}
	if windows.GOOS != "windows" {
		t.Errorf("goos = %q, want windows", windows.GOOS)
	}
	if slices.Contains(funcNames(linux, "example.com/acme"), "Experimental") {
		t.Error("untagged artifact includes a tag-gated function")
	}
	tagged := testOptions("basic")
	tagged.Tags = []string{"acme_experimental"}
	if !slices.Contains(funcNames(mustExtract(t, tagged), "example.com/acme"), "Experimental") {
		t.Error("tagged artifact is missing the tag-gated function")
	}
	bad := testOptions("basic")
	bad.GOOS = "linx"
	if _, err := Extract(bad); err == nil || !strings.Contains(err.Error(), `unknown GOOS "linx"`) {
		t.Errorf("unknown GOOS error = %v", err)
	}
}

func TestSymlinksAreNotFollowed(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/links\n\ngo 1.22\n")
	write("links.go", "// Package links has a symlinked README and source file.\npackage links\n\n// Real is declared in a regular file.\ntype Real struct{}\n")
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("# Secret\n\npackage links\n\n// Leaked must not appear.\ntype Leaked struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "leaked.go"} {
		if err := os.Symlink(outside, filepath.Join(dir, name)); err != nil {
			t.Skipf("symlinks unsupported: %v", err)
		}
	}
	opts := testOptions("")
	opts.Dir = dir
	artifact := mustExtract(t, opts)
	pkg := artifact.Packages[0]
	if pkg.Readme != "" {
		t.Errorf("readme followed a symlink: %q", pkg.Readme)
	}
	if names := typeNames(artifact, "example.com/links"); !slices.Equal(names, []string{"Real"}) {
		t.Errorf("types = %v, want [Real]", names)
	}
}

func TestHardLinksAreNotFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hard link counts are not read on windows")
	}
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/links\n\ngo 1.22\n")
	write("links.go", "// Package links has a hard-linked README and source file.\npackage links\n\n// Real is declared in a regular file.\ntype Real struct{}\n")
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("# Secret\n\npackage links\n\n// Leaked must not appear.\ntype Leaked struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "leaked.go"} {
		if err := os.Link(outside, filepath.Join(dir, name)); err != nil {
			t.Skipf("hard links unsupported: %v", err)
		}
	}
	opts := testOptions("")
	opts.Dir = dir
	artifact := mustExtract(t, opts)
	pkg := artifact.Packages[0]
	if pkg.Readme != "" {
		t.Errorf("readme followed a hard link: %q", pkg.Readme)
	}
	if names := typeNames(artifact, "example.com/links"); !slices.Equal(names, []string{"Real"}) {
		t.Errorf("types = %v, want [Real]", names)
	}
}

func TestNoParent(t *testing.T) {
	parent := t.TempDir()
	sub := filepath.Join(parent, "checkout")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(parent, "go.mod"), "module example.com/parent\n\ngo 1.22\n")
	write(filepath.Join(parent, "other.go"), "package parent\n\ntype Other struct{}\n")
	write(filepath.Join(sub, "sub.go"), "package checkout\n\ntype Sub struct{}\n")

	opts := testOptions("")
	opts.Dir = sub
	artifact := mustExtract(t, opts)
	if artifact.Module != "example.com/parent" || len(artifact.Packages) != 2 {
		t.Errorf("without NoParent: module %q, %d packages; want the parent module with 2 packages", artifact.Module, len(artifact.Packages))
	}

	opts.NoParent = true
	if _, err := Extract(opts); err == nil || !strings.Contains(err.Error(), "no go.mod in "+sub) {
		t.Errorf("NoParent error = %v, want it to name %s", err, sub)
	}

	write(filepath.Join(sub, "go.mod"), "module example.com/checkout\n\ngo 1.22\n")
	artifact = mustExtract(t, opts)
	if artifact.Module != "example.com/checkout" || len(artifact.Packages) != 1 {
		t.Errorf("with NoParent and a local go.mod: module %q, %d packages; want example.com/checkout with 1 package", artifact.Module, len(artifact.Packages))
	}
}

func TestDeterministic(t *testing.T) {
	first := indentJSON(t, mustExtract(t, testOptions("basic")))
	second := indentJSON(t, mustExtract(t, testOptions("basic")))
	if !bytes.Equal(first, second) {
		t.Error("repeated extraction produced different output")
	}
}

func TestEmbeddedFields(t *testing.T) {
	artifact := mustExtract(t, testOptions("generics"))
	embedded := func(name string) []string {
		t.Helper()
		for _, pkg := range artifact.Packages {
			for _, typ := range pkg.Types {
				if typ.Name != name {
					continue
				}
				var names []string
				for _, field := range typ.Fields {
					if field.Embedded {
						names = append(names, field.Name)
					}
				}
				return names
			}
		}
		t.Fatalf("type %s not found", name)
		return nil
	}
	if got := embedded("Resource"); !slices.Equal(got, []string{"Base", "Meta", "Page"}) {
		t.Errorf("Resource embeds %v, want [Base Meta Page]", got)
	}
	if got := embedded("Constraint"); !slices.Equal(got, []string{"error", "any", "Store"}) {
		t.Errorf("Constraint embeds %v, want [error any Store]; int and int | string are type terms", got)
	}
}

func TestDefaultCommentSyntax(t *testing.T) {
	artifact := mustExtract(t, testOptions("generics"))
	if artifact.CommentSyntax != CommentsMarkdown {
		t.Errorf("commentSyntax = %q, want %q", artifact.CommentSyntax, CommentsMarkdown)
	}
}

func TestExtractErrors(t *testing.T) {
	cases := []struct {
		name string
		dir  string
		want string
	}{
		{name: "no module", dir: t.TempDir(), want: "no go.mod found"},
		{name: "parse error", dir: filepath.Join("testdata", "modules", "broken"), want: "broken.go:3:"},
		{name: "conflicting packages", dir: filepath.Join("testdata", "modules", "conflict"), want: "found packages a (a.go) and b (b.go)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := testOptions("basic")
			opts.Dir = tc.dir
			_, err := Extract(opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestParseCommentSyntax(t *testing.T) {
	for _, value := range []string{"markdown", "godoc"} {
		if _, err := ParseCommentSyntax(value); err != nil {
			t.Errorf("ParseCommentSyntax(%q) = %v", value, err)
		}
	}
	if _, err := ParseCommentSyntax("html"); err == nil {
		t.Error("ParseCommentSyntax accepted an invalid value")
	}
}
