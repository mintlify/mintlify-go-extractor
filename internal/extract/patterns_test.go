package extract

import "testing"

func TestPatterns(t *testing.T) {
	const module = "example.com/acme"
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"./...", "example.com/acme", true},
		{"./...", "example.com/acme/option", true},
		{".", "example.com/acme", true},
		{".", "example.com/acme/option", false},
		{"./option", "example.com/acme/option", true},
		{"./option", "example.com/acme/option/sub", false},
		{"./option/...", "example.com/acme/option", true},
		{"./option/...", "example.com/acme/option/sub", true},
		{"./option/...", "example.com/acme/optional", false},
		{"example.com/acme/...", "example.com/acme/legacy", true},
		{"./.../internal", "example.com/acme/a/internal", true},
	}
	for _, tc := range cases {
		if got := compilePatterns([]string{tc.pattern}, module)(tc.path); got != tc.want {
			t.Errorf("pattern %q on %q = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
	if compilePatterns(nil, module)("example.com/acme") {
		t.Error("empty pattern list matched a package")
	}
}

func TestMatchesPrefixPattern(t *testing.T) {
	cases := []struct {
		globs []string
		path  string
		want  bool
	}{
		{[]string{"corp.example"}, "corp.example/sdk/models", true},
		{[]string{"corp.example"}, "corp.example", true},
		{[]string{"corp.example"}, "corp.example.org/sdk", false},
		{[]string{"github.com/acme/*"}, "github.com/acme/private/pkg", true},
		{[]string{"github.com/acme/*"}, "github.com/other/pkg", false},
		{[]string{"*.corp.example"}, "git.corp.example/sdk", true},
		{[]string{" ", "github.com/acme/"}, "github.com/acme/x", true},
		{nil, "github.com/acme/x", false},
	}
	for _, tc := range cases {
		if got := matchesPrefixPattern(tc.globs, tc.path); got != tc.want {
			t.Errorf("matchesPrefixPattern(%q, %q) = %v, want %v", tc.globs, tc.path, got, tc.want)
		}
	}
}

func TestGuessPackageName(t *testing.T) {
	cases := map[string]string{
		"context":                        "context",
		"github.com/public/widgets/v2":   "widgets",
		"gopkg.in/yaml.v3":               "yaml",
		"github.com/public/go-thing":     "thing",
		"github.com/stripe/stripe-go":    "stripe",
		"github.com/acme/my-lib":         "mylib",
		"github.com/acme/sdk.client":     "sdkclient",
		"github.com/openai/openai-go/v3": "openai",
		"k8s.io/api/core/v1":             "v1",
		"gopkg.in/check.v1":              "check",
	}
	for path, want := range cases {
		if got := guessPackageName(path); got != want {
			t.Errorf("guessPackageName(%q) = %q, want %q", path, got, want)
		}
	}
}
