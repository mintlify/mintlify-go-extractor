package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mintlify/mintlify-go-extractor/internal/extract"
)

var basicModule = filepath.Join("internal", "extract", "testdata", "modules", "basic")

func runCLI(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, dir, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestVersion(t *testing.T) {
	code, stdout, _ := runCLI(t, basicModule, "-version")
	if code != 0 || !strings.HasPrefix(stdout, "mintlify-go-extractor ") {
		t.Errorf("code = %d, stdout = %q", code, stdout)
	}
}

func TestHelp(t *testing.T) {
	code, _, stderr := runCLI(t, basicModule, "-h")
	if code != 0 || !strings.Contains(stderr, "usage: mintlify-go-extractor") {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
}

func TestInvalidFlags(t *testing.T) {
	cases := map[string][]string{
		"unknown flag":     {"-nope"},
		"invalid comments": {"-comments", "html"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if code, _, stderr := runCLI(t, basicModule, args...); code != 1 || stderr == "" {
				t.Errorf("code = %d, stderr = %q", code, stderr)
			}
		})
	}
}

func decode(t *testing.T, data []byte) extract.Artifact {
	t.Helper()
	var artifact extract.Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return artifact
}

func TestNoParent(t *testing.T) {
	code, _, stderr := runCLI(t, t.TempDir(), "-no-parent")
	if code != 1 || !strings.Contains(stderr, "-no-parent") {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
}

func TestStdout(t *testing.T) {
	t.Setenv("GOPRIVATE", "")
	code, stdout, stderr := runCLI(t, basicModule, "-comments", "godoc", "./option")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	artifact := decode(t, []byte(stdout))
	if artifact.CommentSyntax != extract.CommentsGodoc || artifact.Module != "example.com/acme" {
		t.Errorf("unexpected artifact header: %+v", artifact)
	}
	if len(artifact.Packages) != 1 || artifact.Packages[0].ImportPath != "example.com/acme/option" {
		t.Errorf("unexpected packages: %+v", artifact.Packages)
	}
}

func TestOutputFile(t *testing.T) {
	t.Setenv("GOPRIVATE", "")
	output := filepath.Join(t.TempDir(), "go.json")
	code, stdout, stderr := runCLI(t, basicModule, "-o", output, "-exclude", "./legacy,./option", "-goos", "windows")
	if code != 0 || stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	artifact := decode(t, data)
	if artifact.GOOS != "windows" || len(artifact.Packages) != 1 {
		t.Errorf("unexpected artifact: goos=%s packages=%d", artifact.GOOS, len(artifact.Packages))
	}
}

func TestPrivatePatterns(t *testing.T) {
	t.Setenv("GOPRIVATE", "corp.example, github.com/acme/*")
	got := privatePatterns()
	if len(got) != 2 || got[0] != "corp.example" || got[1] != "github.com/acme/*" {
		t.Errorf("privatePatterns() = %q", got)
	}
}

func TestPrivatePatternsFromGoEnvFile(t *testing.T) {
	previous, set := os.LookupEnv("GOPRIVATE")
	os.Unsetenv("GOPRIVATE")
	t.Cleanup(func() {
		if set {
			os.Setenv("GOPRIVATE", previous)
		}
	})
	envFile := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(envFile, []byte("GOPROXY=direct\nGOPRIVATE=corp.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", envFile)
	if got := privatePatterns(); len(got) != 1 || got[0] != "corp.example" {
		t.Errorf("privatePatterns() = %q", got)
	}
	t.Setenv("GOENV", "off")
	if got := privatePatterns(); len(got) != 0 {
		t.Errorf("privatePatterns() with GOENV=off = %q", got)
	}
}
