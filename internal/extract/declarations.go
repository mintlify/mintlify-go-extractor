package extract

import (
	"bytes"
	"go/ast"
	"go/doc"
	"go/printer"
	"go/token"
	"strconv"
	"strings"
)

type extractor struct {
	*docRenderer
	fset       *token.FileSet
	importPath string
	modulePath string
	private    []string
	imports    map[string]map[string]string
	exported   map[string]bool
}

func newExtractor(src *source, dpkg *doc.Package, modulePath string, names map[string]string, opts Options) *extractor {
	e := &extractor{
		docRenderer: newDocRenderer(dpkg, src.importPath, modulePath, opts.Private, opts.Comments),
		fset:        src.fset,
		importPath:  src.importPath,
		modulePath:  modulePath,
		private:     opts.Private,
		imports:     map[string]map[string]string{},
		exported:    map[string]bool{},
	}
	for _, file := range src.files {
		filename := src.fset.File(file.Pos()).Name()
		e.imports[filename] = fileImports(file, names)
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						e.markExported(s.Name)
					case *ast.ValueSpec:
						for _, name := range s.Names {
							e.markExported(name)
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil {
					e.markExported(d.Name)
				}
			}
		}
	}
	return e
}

func fileImports(file *ast.File, names map[string]string) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name, known := names[importPath]
		if !known {
			name = guessPackageName(importPath)
		}
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		imports[name] = importPath
	}
	return imports
}

func (e *extractor) markExported(name *ast.Ident) {
	if name.IsExported() {
		e.exported[name.Name] = true
	}
}

func (e *extractor) values(values []*doc.Value) []Value {
	var out []Value
	for _, value := range values {
		docText, deprecated := e.render(value.Doc)
		out = append(out, Value{
			Names:      value.Names,
			Decl:       e.code(value.Decl, value.Decl),
			Doc:        docText,
			Deprecated: deprecated,
		})
	}
	return out
}

func (e *extractor) function(fn *doc.Func) Func {
	docText, deprecated := e.render(fn.Doc)
	return Func{
		Name:       fn.Name,
		Recv:       fn.Recv,
		Decl:       e.code(fn.Decl, fn.Decl),
		Doc:        docText,
		Deprecated: deprecated,
		TypeParams: e.typeParams(fn.Decl.Type.TypeParams, fn.Decl),
		Params:     e.params(fn.Decl.Type.Params, fn.Decl),
		Results:    e.params(fn.Decl.Type.Results, fn.Decl),
		Examples:   e.examples(fn.Examples),
	}
}

func (e *extractor) typ(typ *doc.Type) Type {
	docText, deprecated := e.render(typ.Doc)
	out := Type{
		Name:       typ.Name,
		Kind:       KindDefined,
		Doc:        docText,
		Deprecated: deprecated,
		Consts:     e.values(typ.Consts),
		Vars:       e.values(typ.Vars),
		Examples:   e.examples(typ.Examples),
	}
	for _, fn := range typ.Funcs {
		out.Funcs = append(out.Funcs, e.function(fn))
	}
	for _, method := range typ.Methods {
		out.Methods = append(out.Methods, e.function(method))
	}

	spec := typeSpec(typ)
	if spec == nil {
		out.Decl = e.code(typ.Decl, typ.Decl)
		return out
	}
	out.TypeParams = e.typeParams(spec.TypeParams, typ.Decl)
	var removedLines []lineRange
	switch t := spec.Type.(type) {
	case *ast.StructType:
		out.Kind = KindStruct
		for _, field := range t.Fields.List {
			out.Fields = append(out.Fields, e.fields(field, typ.Decl)...)
		}
		removedLines = e.stripFieldComments(t.Fields)
	case *ast.InterfaceType:
		out.Kind = KindInterface
		for _, field := range t.Methods.List {
			if len(field.Names) == 0 {
				if embeddedName(field.Type) != "" {
					out.Fields = append(out.Fields, e.fields(field, typ.Decl)...)
				}
				continue
			}
			if method, ok := e.interfaceMethod(typ.Name, field, typ.Decl); ok {
				out.Methods = append(out.Methods, method)
			}
		}
		removedLines = e.stripFieldComments(t.Methods)
	}
	if spec.Assign.IsValid() {
		out.Kind = KindAlias
	}
	out.Decl = e.codeWith(e.withoutLines(typ.Decl.Pos(), removedLines), typ.Decl, typ.Decl)
	return out
}

func typeSpec(typ *doc.Type) *ast.TypeSpec {
	for _, spec := range typ.Decl.Specs {
		if s, ok := spec.(*ast.TypeSpec); ok && s.Name.Name == typ.Name {
			return s
		}
	}
	return nil
}

type lineRange struct {
	after int
	last  int
}

func (e *extractor) stripFieldComments(fields *ast.FieldList) []lineRange {
	if fields == nil {
		return nil
	}
	var removed []lineRange
	for _, field := range fields.List {
		if field.Doc != nil {
			docLine := e.fset.Position(field.Doc.Pos()).Line
			fieldLine := e.fset.Position(field.Pos()).Line
			if fieldLine > docLine {
				removed = append(removed, lineRange{after: docLine, last: fieldLine})
			}
		}
		field.Doc = nil
		field.Comment = nil
	}
	return removed
}

func (e *extractor) withoutLines(pos token.Pos, removed []lineRange) *token.FileSet {
	file := e.fset.File(pos)
	if file == nil || len(removed) == 0 {
		return e.fset
	}
	drop := map[int]bool{}
	for _, r := range removed {
		for line := r.after + 1; line <= r.last; line++ {
			drop[line] = true
		}
	}
	var lines []int
	for i, offset := range file.Lines() {
		if !drop[i+1] {
			lines = append(lines, offset)
		}
	}
	fset := token.NewFileSet()
	compact := fset.AddFile(file.Name(), file.Base(), file.Size())
	if !compact.SetLines(lines) {
		return e.fset
	}
	return fset
}

func fieldDoc(field *ast.Field) string {
	if text := field.Doc.Text(); text != "" {
		return text
	}
	return field.Comment.Text()
}

func (e *extractor) interfaceMethod(recv string, field *ast.Field, scope ast.Node) (Func, bool) {
	fnType, ok := field.Type.(*ast.FuncType)
	if !ok || !field.Names[0].IsExported() {
		return Func{}, false
	}
	docText, deprecated := e.render(fieldDoc(field))
	return Func{
		Name:       field.Names[0].Name,
		Recv:       recv,
		Decl:       e.code(&ast.FuncDecl{Name: field.Names[0], Type: fnType}, scope),
		Doc:        docText,
		Deprecated: deprecated,
		Params:     e.params(fnType.Params, scope),
		Results:    e.params(fnType.Results, scope),
	}, true
}

func (e *extractor) fields(field *ast.Field, scope ast.Node) []Field {
	docText, deprecated := e.render(fieldDoc(field))
	tag := ""
	if field.Tag != nil {
		tag = field.Tag.Value
		if unquoted, err := strconv.Unquote(tag); err == nil {
			tag = unquoted
		}
	}
	typeCode := e.code(field.Type, scope)
	if len(field.Names) == 0 {
		return []Field{{Name: embeddedName(field.Type), Type: typeCode, Doc: docText, Deprecated: deprecated, Embedded: true, Tag: tag}}
	}
	var out []Field
	for _, name := range field.Names {
		if name.IsExported() {
			out = append(out, Field{Name: name.Name, Type: typeCode, Doc: docText, Deprecated: deprecated, Tag: tag})
		}
	}
	return out
}

func embeddedName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return embeddedName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.IndexExpr:
		return embeddedName(e.X)
	case *ast.IndexListExpr:
		return embeddedName(e.X)
	case *ast.Ident:
		// A bare unexported identifier in an interface body is a predeclared
		// type term (int, string, ...), not an embedded interface, except for
		// the predeclared interfaces themselves.
		if !e.IsExported() && e.Name != "error" && e.Name != "comparable" && e.Name != "any" {
			return ""
		}
		return e.Name
	}
	return ""
}

func (e *extractor) typeParams(list *ast.FieldList, scope ast.Node) []TypeParam {
	if list == nil {
		return nil
	}
	var out []TypeParam
	for _, field := range list.List {
		constraint := e.code(field.Type, scope)
		for _, name := range field.Names {
			out = append(out, TypeParam{Name: name.Name, Constraint: constraint})
		}
	}
	return out
}

func (e *extractor) params(list *ast.FieldList, scope ast.Node) []Param {
	if list == nil {
		return nil
	}
	var out []Param
	for _, field := range list.List {
		typeCode := e.code(field.Type, scope)
		if len(field.Names) == 0 {
			out = append(out, Param{Type: typeCode})
			continue
		}
		for _, name := range field.Names {
			out = append(out, Param{Name: name.Name, Type: typeCode})
		}
	}
	return out
}

func (e *extractor) examples(examples []*doc.Example) []Example {
	var out []Example
	for _, example := range examples {
		code, ok := e.exampleCode(example)
		if !ok {
			continue
		}
		docText, _ := e.render(example.Doc)
		out = append(out, Example{Suffix: example.Suffix, Doc: docText, Code: code, Output: example.Output})
	}
	return out
}

func (e *extractor) exampleCode(example *doc.Example) (string, bool) {
	config := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	var buf bytes.Buffer
	var comments []*ast.CommentGroup
	for _, group := range example.Comments {
		text := group.Text()
		if !strings.HasPrefix(text, "Output:") && !strings.HasPrefix(text, "Unordered output:") {
			comments = append(comments, group)
		}
	}
	if err := config.Fprint(&buf, e.fset, &printer.CommentedNode{Node: example.Code, Comments: comments}); err != nil {
		return "", false
	}
	code := buf.String()
	if _, ok := example.Code.(*ast.BlockStmt); ok {
		code = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(code), "{"), "}")
		lines := strings.Split(strings.Trim(code, "\n"), "\n")
		for i, line := range lines {
			lines[i] = strings.TrimPrefix(line, "\t")
		}
		code = strings.Join(lines, "\n")
	}
	return strings.TrimSpace(code), true
}
