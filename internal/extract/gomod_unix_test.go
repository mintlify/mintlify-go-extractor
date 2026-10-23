//go:build unix

package extract

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestGoModMustBeRegular(t *testing.T) {
	targets := map[string]func(t *testing.T, path string){
		"symlink to /dev/zero": func(t *testing.T, path string) {
			if err := os.Symlink("/dev/zero", path); err != nil {
				t.Skip(err)
			}
		},
		"symlink to a module file outside": func(t *testing.T, path string) {
			outside := filepath.Join(t.TempDir(), "secret.rb")
			if err := os.WriteFile(outside, []byte("module InternalBillingSecrets\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Skip(err)
			}
		},
		"fifo": func(t *testing.T, path string) {
			if err := syscall.Mkfifo(path, 0o644); err != nil {
				t.Skip(err)
			}
		},
		"oversized": func(t *testing.T, path string) {
			data := append([]byte("module example.com/big\n"), bytes.Repeat([]byte("// x\n"), 1<<19)...)
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, plant := range targets {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			plant(t, filepath.Join(dir, "go.mod"))
			for _, noParent := range []bool{false, true} {
				opts := testOptions("")
				opts.Dir = dir
				opts.NoParent = noParent
				done := make(chan error, 1)
				go func() {
					_, err := Extract(opts)
					done <- err
				}()
				select {
				case err := <-done:
					if err == nil || !strings.Contains(err.Error(), "not a regular file") {
						t.Errorf("NoParent=%v: error = %v, want a refusal", noParent, err)
					}
				case <-time.After(10 * time.Second):
					t.Fatalf("NoParent=%v: extraction did not finish", noParent)
				}
			}
		})
	}
}
