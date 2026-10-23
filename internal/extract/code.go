package extract

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/scanner"
	"go/token"
	"strings"
)

type span struct {
	start int
	end   int
}

func (e *extractor) code(node ast.Node, scope ast.Node) Code {
	return e.codeWith(e.fset, node, scope)
}

func (e *extractor) codeWith(fset *token.FileSet, node ast.Node, scope ast.Node) Code {
	var buf bytes.Buffer
	config := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := config.Fprint(&buf, fset, node); err != nil {
		return Code{{Text: fmt.Sprintf("/* %v */", err)}}
	}
	text := buf.String()
	refs := e.identRefs(node, scope)

	file := token.NewFileSet().AddFile("", -1, len(text))
	var s scanner.Scanner
	s.Init(file, []byte(text), nil, 0)
	var idents []span
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.IDENT {
			offset := file.Offset(pos)
			idents = append(idents, span{offset, offset + len(lit)})
		}
	}
	if len(idents) != len(refs) {
		return Code{{Text: text}}
	}

	var out Code
	last := 0
	for i, ident := range idents {
		if refs[i] == nil {
			continue
		}
		if ident.start > last {
			out = append(out, Token{Text: text[last:ident.start]})
		}
		out = append(out, Token{Text: text[ident.start:ident.end], Ref: refs[i]})
		last = ident.end
	}
	if last < len(text) {
		out = append(out, Token{Text: text[last:]})
	}
	return out
}

func (e *extractor) identRefs(node ast.Node, scope ast.Node) []*Ref {
	imports := map[string]string{}
	if file := e.fset.File(scope.Pos()); file != nil {
		imports = e.imports[file.Name()]
	}
	shadowed := typeParamNames(scope)

	definitions := map[*ast.Ident]bool{}
	selectors := map[*ast.Ident]*Ref{}
	skipped := map[*ast.Ident]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Field:
			for _, name := range v.Names {
				definitions[name] = true
			}
		case *ast.TypeSpec:
			definitions[v.Name] = true
		case *ast.ValueSpec:
			for _, name := range v.Names {
				definitions[name] = true
			}
		case *ast.FuncDecl:
			definitions[v.Name] = true
		case *ast.KeyValueExpr:
			if key, ok := v.Key.(*ast.Ident); ok {
				definitions[key] = true
			}
		case *ast.SelectorExpr:
			if x, ok := v.X.(*ast.Ident); ok {
				if importPath, ok := imports[x.Name]; ok && !shadowed[x.Name] {
					skipped[x] = true
					if v.Sel.IsExported() {
						selectors[v.Sel] = e.ref(importPath, v.Sel.Name)
					} else {
						skipped[v.Sel] = true
					}
					return false
				}
			}
			skipped[v.Sel] = true
		}
		return true
	})

	var refs []*Ref
	ast.Inspect(node, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		switch {
		case selectors[ident] != nil:
			refs = append(refs, selectors[ident])
		case definitions[ident], skipped[ident], shadowed[ident.Name]:
			refs = append(refs, nil)
		case e.exported[ident.Name]:
			refs = append(refs, &Ref{Name: ident.Name})
		default:
			refs = append(refs, nil)
		}
		return true
	})
	return refs
}

func (e *extractor) ref(importPath, name string) *Ref {
	ref := &Ref{Pkg: importPath, Name: name}
	inModule := importPath == e.modulePath || strings.HasPrefix(importPath, e.modulePath+"/")
	if !inModule && matchesPrefixPattern(e.private, importPath) {
		ref.Private = true
	}
	return ref
}

func typeParamNames(scope ast.Node) map[string]bool {
	names := map[string]bool{}
	addFields := func(list *ast.FieldList) {
		if list == nil {
			return
		}
		for _, field := range list.List {
			for _, name := range field.Names {
				names[name.Name] = true
			}
		}
	}
	ast.Inspect(scope, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.TypeSpec:
			addFields(v.TypeParams)
		case *ast.FuncType:
			addFields(v.TypeParams)
		case *ast.FuncDecl:
			if v.Recv == nil {
				return true
			}
			for _, field := range v.Recv.List {
				ast.Inspect(field.Type, func(r ast.Node) bool {
					switch index := r.(type) {
					case *ast.IndexExpr:
						if ident, ok := index.Index.(*ast.Ident); ok {
							names[ident.Name] = true
						}
					case *ast.IndexListExpr:
						for _, expr := range index.Indices {
							if ident, ok := expr.(*ast.Ident); ok {
								names[ident.Name] = true
							}
						}
					}
					return true
				})
			}
		}
		return true
	})
	return names
}
