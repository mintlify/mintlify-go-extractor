package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const SchemaVersion = 1

type CommentSyntax string

const (
	CommentsMarkdown CommentSyntax = "markdown"
	CommentsGodoc    CommentSyntax = "godoc"
)

func ParseCommentSyntax(value string) (CommentSyntax, error) {
	switch syntax := CommentSyntax(value); syntax {
	case CommentsMarkdown, CommentsGodoc:
		return syntax, nil
	}
	return "", fmt.Errorf("invalid -comments value %q: must be %q or %q", value, CommentsMarkdown, CommentsGodoc)
}

type Artifact struct {
	SchemaVersion int           `json:"schemaVersion"`
	Generator     Generator     `json:"generator"`
	Module        string        `json:"module"`
	GOOS          string        `json:"goos"`
	GOARCH        string        `json:"goarch"`
	CommentSyntax CommentSyntax `json:"commentSyntax"`
	Packages      []Package     `json:"packages"`
}

type Generator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Go      string `json:"go"`
}

type Ref struct {
	Pkg     string `json:"pkg,omitempty"`
	Name    string `json:"name"`
	Private bool   `json:"private,omitempty"`
}

type Token struct {
	Text string `json:"t"`
	Ref  *Ref   `json:"ref,omitempty"`
}

type Code []Token

type TypeParam struct {
	Name       string `json:"name"`
	Constraint Code   `json:"constraint"`
}

type Param struct {
	Name string `json:"name,omitempty"`
	Type Code   `json:"type"`
}

type Example struct {
	Suffix string `json:"suffix,omitempty"`
	Doc    string `json:"doc,omitempty"`
	Code   string `json:"code"`
	Output string `json:"output,omitempty"`
}

type Func struct {
	Name       string      `json:"name"`
	Recv       string      `json:"recv,omitempty"`
	Decl       Code        `json:"decl"`
	Doc        string      `json:"doc,omitempty"`
	Deprecated string      `json:"deprecated,omitempty"`
	TypeParams []TypeParam `json:"typeParams,omitempty"`
	Params     []Param     `json:"params,omitempty"`
	Results    []Param     `json:"results,omitempty"`
	Examples   []Example   `json:"examples,omitempty"`
}

type Field struct {
	Name       string `json:"name"`
	Type       Code   `json:"type"`
	Doc        string `json:"doc,omitempty"`
	Deprecated string `json:"deprecated,omitempty"`
	Embedded   bool   `json:"embedded,omitempty"`
	Tag        string `json:"tag,omitempty"`
}

type Value struct {
	Names      []string `json:"names"`
	Decl       Code     `json:"decl"`
	Doc        string   `json:"doc,omitempty"`
	Deprecated string   `json:"deprecated,omitempty"`
}

type TypeKind string

const (
	KindStruct    TypeKind = "struct"
	KindInterface TypeKind = "interface"
	KindDefined   TypeKind = "defined"
	KindAlias     TypeKind = "alias"
)

type Type struct {
	Name       string      `json:"name"`
	Kind       TypeKind    `json:"kind"`
	Decl       Code        `json:"decl"`
	Doc        string      `json:"doc,omitempty"`
	Deprecated string      `json:"deprecated,omitempty"`
	TypeParams []TypeParam `json:"typeParams,omitempty"`
	Fields     []Field     `json:"fields,omitempty"`
	Methods    []Func      `json:"methods,omitempty"`
	Funcs      []Func      `json:"funcs,omitempty"`
	Consts     []Value     `json:"consts,omitempty"`
	Vars       []Value     `json:"vars,omitempty"`
	Examples   []Example   `json:"examples,omitempty"`
}

type Package struct {
	ImportPath string    `json:"importPath"`
	Name       string    `json:"name"`
	Dir        string    `json:"dir"`
	Synopsis   string    `json:"synopsis,omitempty"`
	Doc        string    `json:"doc,omitempty"`
	Readme     string    `json:"readme,omitempty"`
	Deprecated string    `json:"deprecated,omitempty"`
	Consts     []Value   `json:"consts,omitempty"`
	Vars       []Value   `json:"vars,omitempty"`
	Funcs      []Func    `json:"funcs,omitempty"`
	Types      []Type    `json:"types,omitempty"`
	Examples   []Example `json:"examples,omitempty"`
}

func Marshal(artifact *Artifact) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(artifact); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
