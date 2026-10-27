package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/mintlify/mintlify-go-extractor/internal/extract"
)

const toolName = "mintlify-go-extractor"

func run(args []string, dir string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(toolName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("o", "", "write the artifact to `file` instead of stdout")
	exclude := flags.String("exclude", "", "comma-separated package `patterns` to skip")
	excludeSymbols := flags.String("exclude-symbols", "", "comma-separated symbol `patterns` to skip, such as Name, Type.Member, or dir.Name")
	comments := flags.String("comments", string(extract.CommentsMarkdown), "doc comment `syntax`: markdown or godoc")
	goos := flags.String("goos", "linux", "GOOS used to select files")
	goarch := flags.String("goarch", "amd64", "GOARCH used to select files")
	tags := flags.String("tags", "", "comma-separated build `tags` used to select files")
	internal := flags.Bool("internal", false, "include internal packages")
	noParent := flags.Bool("no-parent", false, "require go.mod in the target directory instead of searching parent directories")
	showVersion := flags.Bool("version", false, "print the version and exit")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "usage: %s [flags] [packages]\n\nPackage patterns are relative to the module root and default to ./...\n\nFlags:\n", toolName)
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if *showVersion {
		fmt.Fprintf(stdout, "%s %s\n", toolName, version())
		return 0
	}

	syntax, err := extract.ParseCommentSyntax(*comments)
	if err != nil {
		return fail(stderr, err)
	}

	artifact, err := extract.Extract(extract.Options{
		Dir:            dir,
		Patterns:       flags.Args(),
		Exclude:        splitList(*exclude),
		ExcludeSymbols: splitList(*excludeSymbols),
		Comments:       syntax,
		GOOS:           *goos,
		GOARCH:         *goarch,
		Tags:           splitList(*tags),
		Internal:       *internal,
		NoParent:       *noParent,
		Private:        privatePatterns(),
		Generator:      extract.Generator{Name: toolName, Version: version(), Go: runtime.Version()},
	})
	if err != nil {
		return fail(stderr, err)
	}

	data, err := extract.Marshal(artifact)
	if err != nil {
		return fail(stderr, err)
	}

	if *output == "" {
		_, err = stdout.Write(data)
	} else {
		target := *output
		if !filepath.IsAbs(target) {
			target = filepath.Join(dir, target)
		}
		err = os.WriteFile(target, data, 0o644)
	}
	if err != nil {
		return fail(stderr, err)
	}
	return 0
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s: %v\n", toolName, err)
	return 1
}

// buildVersion is set at link time (-ldflags "-X main.buildVersion=v1.2.3") for
// prebuilt release binaries, which are built from a source checkout where Go
// records no module version.
var buildVersion string

func version() string {
	if buildVersion != "" {
		return buildVersion
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "devel"
	}
	return info.Main.Version
}

func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func privatePatterns() []string {
	if value, ok := os.LookupEnv("GOPRIVATE"); ok {
		return splitList(value)
	}
	return splitList(goEnvFileValue("GOPRIVATE"))
}

func goEnvFileValue(key string) string {
	file := os.Getenv("GOENV")
	if file == "off" {
		return ""
	}
	if file == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return ""
		}
		file = filepath.Join(configDir, "go", "env")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		name, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if found && name == key {
			return value
		}
	}
	return ""
}
