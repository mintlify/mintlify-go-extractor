package extract

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func findPackage(t *testing.T, artifact *Artifact, importPath string) Package {
	t.Helper()
	for _, pkg := range artifact.Packages {
		if pkg.ImportPath == importPath {
			return pkg
		}
	}
	t.Fatalf("missing package %s", importPath)
	return Package{}
}

func findType(t *testing.T, pkg Package, name string) Type {
	t.Helper()
	for _, typ := range pkg.Types {
		if typ.Name == name {
			return typ
		}
	}
	t.Fatalf("missing type %s", name)
	return Type{}
}

func funcList(funcs []Func) []string {
	var names []string
	for _, fn := range funcs {
		names = append(names, fn.Name)
	}
	return names
}

func fieldList(fields []Field) []string {
	var names []string
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return names
}

func valueNames(values []Value) []string {
	var names []string
	for _, value := range values {
		names = append(names, value.Names...)
	}
	return names
}

func extractExcluding(t *testing.T, patterns ...string) *Artifact {
	t.Helper()
	opts := testOptions("basic")
	opts.ExcludeSymbols = patterns
	return mustExtract(t, opts)
}

func TestExcludeSymbols(t *testing.T) {
	full := mustExtract(t, testOptions("basic"))
	root := findPackage(t, full, "example.com/acme")
	client := findType(t, root, "Client")
	color := findType(t, root, "Color")
	if len(color.Consts) == 0 || len(client.Fields) == 0 || len(client.Methods) == 0 || len(client.Funcs) == 0 {
		t.Fatal("the basic fixture no longer has the symbols this test excludes")
	}

	t.Run("type names with a wildcard", func(t *testing.T) {
		got := typeNames(extractExcluding(t, "Widget*"), "example.com/acme")
		want := slices.DeleteFunc(typeNames(full, "example.com/acme"), func(name string) bool {
			return strings.HasPrefix(name, "Widget")
		})
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("methods, fields, and constructors", func(t *testing.T) {
		pkg := findPackage(t, extractExcluding(t, "Client.Do", "Client.BaseURL", "NewClient"), "example.com/acme")
		got := findType(t, pkg, "Client")
		if slices.Contains(funcList(got.Methods), "Do") || len(got.Methods) != len(client.Methods)-1 {
			t.Errorf("methods = %v", funcList(got.Methods))
		}
		if slices.Contains(fieldList(got.Fields), "BaseURL") || len(got.Fields) != len(client.Fields)-1 {
			t.Errorf("fields = %v", fieldList(got.Fields))
		}
		if slices.Contains(funcList(got.Funcs), "NewClient") {
			t.Errorf("constructors = %v", funcList(got.Funcs))
		}
	})

	t.Run("interface methods leave the declaration too", func(t *testing.T) {
		declText := func(typ Type) string {
			var text strings.Builder
			for _, token := range typ.Decl {
				text.WriteString(token.Text)
			}
			return text.String()
		}
		if !strings.Contains(declText(findType(t, root, "Store")), "Save(") {
			t.Fatal("the basic fixture's Store no longer declares Save")
		}
		store := findType(t, findPackage(t, extractExcluding(t, "Store.Save"), "example.com/acme"), "Store")
		if slices.Contains(funcList(store.Methods), "Save") || strings.Contains(declText(store), "Save(") {
			t.Errorf("Save is still documented: methods=%v decl=%q", funcList(store.Methods), declText(store))
		}
		if !strings.Contains(declText(store), "io.Closer") {
			t.Errorf("the embedded io.Closer was removed: %q", declText(store))
		}
	})

	t.Run("struct fields leave the declaration too", func(t *testing.T) {
		dir := t.TempDir()
		files := map[string]string{
			"go.mod":    "module example.com/fields\n\ngo 1.22\n",
			"fields.go": "package fields\n\n// Point is a point.\ntype Point struct {\n\t// X and Y are coordinates.\n\tX, Y int\n\t// Secret is internal.\n\tSecret string\n\tLabel  string\n}\n",
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		opts := testOptions("basic")
		opts.Dir = dir
		opts.ExcludeSymbols = []string{"Point.Secret", "Point.Y"}
		point := findType(t, findPackage(t, mustExtract(t, opts), "example.com/fields"), "Point")
		var decl strings.Builder
		for _, token := range point.Decl {
			decl.WriteString(token.Text)
		}
		if !slices.Equal(fieldList(point.Fields), []string{"X", "Label"}) {
			t.Errorf("fields = %v", fieldList(point.Fields))
		}
		for _, gone := range []string{"Secret", "Y,", ", Y", "internal"} {
			if strings.Contains(decl.String(), gone) {
				t.Errorf("decl still contains %q: %q", gone, decl.String())
			}
		}
		if !strings.Contains(decl.String(), "\tX ") || !strings.Contains(decl.String(), "\tLabel ") {
			t.Errorf("decl lost kept fields: %q", decl.String())
		}
	})

	t.Run("a package-qualified name only matches that package", func(t *testing.T) {
		artifact := extractExcluding(t, "option.WithAPIKey", "option.Client")
		if slices.Contains(funcNames(artifact, "example.com/acme/option"), "WithAPIKey") {
			t.Error("option.WithAPIKey was not excluded")
		}
		if !slices.Contains(typeNames(artifact, "example.com/acme"), "Client") {
			t.Error("option.Client excluded the root package's Client")
		}
	})

	t.Run("a bare name matches in every package", func(t *testing.T) {
		if slices.Contains(funcNames(extractExcluding(t, "WithAPIKey"), "example.com/acme/option"), "WithAPIKey") {
			t.Error("WithAPIKey was not excluded from the option package")
		}
	})

	t.Run("a const group goes only when every name in it matches", func(t *testing.T) {
		names := valueNames(color.Consts)
		partial := findType(t, findPackage(t, extractExcluding(t, names[0]), "example.com/acme"), "Color")
		if !slices.Equal(valueNames(partial.Consts), names) {
			t.Errorf("excluding %s changed the group to %v", names[0], valueNames(partial.Consts))
		}
		all := findType(t, findPackage(t, extractExcluding(t, names...), "example.com/acme"), "Color")
		if len(all.Consts) != 0 {
			t.Errorf("excluding every name kept %v", valueNames(all.Consts))
		}
	})

	t.Run("bad patterns are rejected", func(t *testing.T) {
		opts := testOptions("basic")
		opts.ExcludeSymbols = []string{"Client["}
		if _, err := Extract(opts); err == nil || !strings.Contains(err.Error(), "-exclude-symbols") {
			t.Errorf("err = %v", err)
		}
	})
}
