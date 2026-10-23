package extract

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"
)

const docsSource = `package acme

import "io"

type Client struct{}

func (c *Client) Do(r io.Reader) error { return nil }

func NewClient() *Client { return nil }
`

func testRenderer(t *testing.T, syntax CommentSyntax) *docRenderer {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "acme.go", docsSource, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	dpkg, err := doc.NewFromFiles(fset, []*ast.File{file}, "example.com/acme")
	if err != nil {
		t.Fatal(err)
	}
	return newDocRenderer(dpkg, "example.com/acme", "example.com/acme", []string{"corp.example"}, syntax)
}

func TestResolveLinks(t *testing.T) {
	r := testRenderer(t, CommentsMarkdown)
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"local type", "Use [Client].", "Use [Client](go:example.com/acme#Client)."},
		{"method", "Call [Client.Do].", "Call [Client.Do](go:example.com/acme#Client.Do)."},
		{"pointer", "Returns [*Client].", "Returns [*Client](go:example.com/acme#Client)."},
		{"imported", "Reads an [io.Reader].", "Reads an [io.Reader](go:io#Reader)."},
		{"private", "Wraps [corp.example/sdk/models.Model] and [corp.example/sdk/models].", "Wraps [corp.example/sdk/models.Model] and [corp.example/sdk/models]."},
		{"package", "See [io].", "See [io](go:io)."},
		{"unknown", "See [Missing].", "See [Missing]."},
		{"inline link", "See [Client](https://acme.dev).", "See [Client](https://acme.dev)."},
		{"reference link", "See [Client][docs].", "See [Client][docs]."},
		{"link destination", "See [guide](https://acme.dev/?q=[Client]) and [Client].", "See [guide](https://acme.dev/?q=[Client]) and [Client](go:example.com/acme#Client)."},
		{"escaped link destination", `See [guide](https://acme.dev/a\(b) and [Client].`, `See [guide](https://acme.dev/a\(b) and [Client](go:example.com/acme#Client).`},
		{"nested link label", "See [the [Client] docs](https://acme.dev) and [Client].", "See [the [Client] docs](https://acme.dev) and [Client](go:example.com/acme#Client)."},
		{"unclosed link destination is text", "See [guide](docs and [Client].", "See [guide](docs and [Client](go:example.com/acme#Client)."},
		{"image", "![Client](client.png) shows [Client].", "![Client](client.png) shows [Client](go:example.com/acme#Client)."},
		{"code span", "Use `[Client]` or ``x [Client] y``.", "Use `[Client]` or ``x [Client] y``."},
		{"multiline code span", "Use `x\n[Client] y` then [Client].", "Use `x\n[Client] y` then [Client](go:example.com/acme#Client)."},
		{"unmatched backtick is literal", "A ` tick [Client]\n\n[Client]", "A ` tick [Client](go:example.com/acme#Client)\n\n[Client](go:example.com/acme#Client)"},
		{"backticks in a link destination", "See [Client](https://acme.dev/`x`path) and [Client].", "See [Client](https://acme.dev/`x`path) and [Client](go:example.com/acme#Client)."},
		{"doc link beside the same name in a code span", "[Client] uses `[Client]` syntax.", "[Client](go:example.com/acme#Client) uses `[Client]` syntax."},
		{"link label beside a code span", "[a](b) `[c](d)` [Client]", "[a](b) `[c](d)` [Client](go:example.com/acme#Client)"},
		{"bracket in a code span does not close a label", "[foo`](/uri)` [Client]", "[foo`](/uri)` [Client](go:example.com/acme#Client)"},
		{"fence line with text does not close", "```go\n```json\n[Client]\n```\n[Client]", "```go\n```json\n[Client]\n```\n[Client](go:example.com/acme#Client)"},
		{"longer closing fence closes", "```go\n[Client]\n````  \n[Client]", "```go\n[Client]\n````  \n[Client](go:example.com/acme#Client)"},
		{"escaped backticks are literal", "Use \\`[Client]\\` here.", "Use \\`[Client](go:example.com/acme#Client)\\` here."},
		{"backslash inside a span is literal", "Use `a\\` then [Client].", "Use `a\\` then [Client](go:example.com/acme#Client)."},
		{"closing run must match length", "Use ``x ` [Client]`` then [Client].", "Use ``x ` [Client]`` then [Client](go:example.com/acme#Client)."},
		{"fence ends the paragraph", "Open ` here\n```go\n[Client]\n```\n[Client]", "Open ` here\n```go\n[Client]\n```\n[Client](go:example.com/acme#Client)"},
		{"list continuation", "- see\n    - nested [Client]", "- see\n    - nested [Client](go:example.com/acme#Client)"},
		{"indented code after blank", "Text\n\n    [Client]", "Text\n\n    [Client]"},
		{"autolink", "See <https://acme.dev/[Client]> and [Client].", "See <https://acme.dev/[Client]> and [Client](go:example.com/acme#Client)."},
		{"raw url", "See https://acme.dev/[Client] and [Client].", "See https://acme.dev/[Client] and [Client](go:example.com/acme#Client)."},
		{"inline html", "A <a href=\"[Client]\">x</a> and [Client].", "A <a href=\"[Client]\">x</a> and [Client](go:example.com/acme#Client)."},
		{"self-closing html", "A <br/> then [Client].", "A <br/> then [Client](go:example.com/acme#Client)."},
		{"html attribute with angle bracket", "A <a title=\"x > [Client]\" data-x='[Client]'>y</a> and [Client].", "A <a title=\"x > [Client]\" data-x='[Client]'>y</a> and [Client](go:example.com/acme#Client)."},
		{"html attribute without quotes", "A <a href=[Client]>y</a> and [Client].", "A <a href=[Client]>y</a> and [Client](go:example.com/acme#Client)."},
		{"html comment", "A <!-- [Client] > --> then [Client].", "A <!-- [Client] > --> then [Client](go:example.com/acme#Client)."},
		{"email autolink", "Mail <dev@acme.dev> about [Client].", "Mail <dev@acme.dev> about [Client](go:example.com/acme#Client)."},
		{"angle brackets as text", "See <[Client]> and a < b > [Client].", "See <[Client](go:example.com/acme#Client)> and a < b > [Client](go:example.com/acme#Client)."},
		{"heading then indented code", "# Usage\n    [Client]\n[Client]", "# Usage\n    [Client]\n[Client](go:example.com/acme#Client)"},
		{"heading links", "## See [Client]\nText [Client]", "## See [Client](go:example.com/acme#Client)\nText [Client](go:example.com/acme#Client)"},
		{"heading interrupts paragraph", "Text [Client]\n# Usage\n    [Client]", "Text [Client](go:example.com/acme#Client)\n# Usage\n    [Client]"},
		{"hashes without space are text", "#tag [Client]\n    [Client]", "#tag [Client](go:example.com/acme#Client)\n    [Client](go:example.com/acme#Client)"},
		{"escaped backslash", `Use \\[Client].`, `Use \\[Client](go:example.com/acme#Client).`},
		{"escaped", `Use \[Client].`, `Use \[Client].`},
		{"markdown kept", "A **bold** [Client] and _em_.", "A **bold** [Client](go:example.com/acme#Client) and _em_."},
		{"defined reference", "See [Client].\n\n[Client]: https://acme.dev", "See [Client].\n\n[Client]: https://acme.dev"},
		{"fenced code", "```go\nvar c [Client]\n```\n[Client]", "```go\nvar c [Client]\n```\n[Client](go:example.com/acme#Client)"},
		{"indented code", "Example:\n\n\tc := [NewClient]()", "Example:\n\n\tc := [NewClient]()"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.markdown(tc.in); got != tc.want {
				t.Errorf("markdown(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveLinksIsLinear(t *testing.T) {
	r := testRenderer(t, CommentsMarkdown)
	const n = 100000
	cases := map[string]string{
		"unmatched brackets":     strings.Repeat("[", n) + " [Client]",
		"unclosed destinations":  strings.Repeat("[a](", n) + " [Client]",
		"many links":             strings.Repeat("[a](b) ", n) + "[Client]",
		"long url":               "https://acme.dev/" + strings.Repeat("[x]", n) + " [Client]",
		"unmatched angle":        strings.Repeat("<", n) + " [Client]",
		"nested reference links": strings.Repeat("[a][", n) + " [Client]",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			got := r.markdown(in)
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Fatalf("took %v", elapsed)
			}
			if !strings.HasSuffix(got, "[Client](go:example.com/acme#Client)") {
				t.Errorf("trailing [Client] was not resolved: %q", got[len(got)-60:])
			}
		})
	}
}

func TestGodocRendering(t *testing.T) {
	r := testRenderer(t, CommentsGodoc)
	got := r.markdown("A **bold** [Client].\n\n# Usage\n\n\tc := NewClient()")
	want := "A \\*\\*bold\\*\\* [Client](go:example.com/acme#Client).\n\n### Usage\n\n\tc := NewClient()"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestGodocPrivateLinks(t *testing.T) {
	r := testRenderer(t, CommentsGodoc)
	got := r.markdown("See [corp.example/sdk/models.Model] and [Client].")
	want := "See corp.example/sdk/models.Model and [Client](go:example.com/acme#Client)."
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestSplitDeprecated(t *testing.T) {
	cases := []struct {
		name           string
		in             string
		wantBody       string
		wantDeprecated string
	}{
		{"none", "Does things.\n", "Does things.", ""},
		{"trailing", "Does things.\n\nDeprecated: Use [Other].\n", "Does things.", "Use [Other]."},
		{"multi-line", "Does things.\n\nDeprecated: Use [Other]\ninstead.\n\nMore text.\n", "Does things.\n\nMore text.", "Use [Other]\ninstead."},
		{"only", "Deprecated: gone.\n", "", "gone."},
		{"indented is code", "Example:\n\n\tDeprecated: not a notice\n", "Example:\n\n\tDeprecated: not a notice", ""},
		{"mid-paragraph", "Text that says\nDeprecated: inline.\n", "Text that says\nDeprecated: inline.", ""},
		{"fenced code", "Example:\n\n```text\nold\n\nDeprecated: old value\n```\n\nDeprecated: gone.\n", "Example:\n\n```text\nold\n\nDeprecated: old value\n```", "gone."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, deprecated := splitDeprecated(tc.in)
			if body != tc.wantBody || deprecated != tc.wantDeprecated {
				t.Errorf("splitDeprecated(%q) = (%q, %q), want (%q, %q)", tc.in, body, deprecated, tc.wantBody, tc.wantDeprecated)
			}
		})
	}
}
