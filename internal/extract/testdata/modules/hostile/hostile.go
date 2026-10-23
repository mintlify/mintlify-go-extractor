// Package hostile carries doc text that is special in MDX but ordinary in Go.
//
// Use map{a: b} and struct{}{} or <b>bold</b> and {props.x}.
//
// import x from "y"
//
// export GOPRIVATE=example.com/*
package hostile

// Record has tags and docs that downstream clients must escape, not interpret.
//
// A field like <Field/> or {value} must stay text.
type Record struct {
	// ID is `json:"id"` on the wire and uses {braces} in prose.
	ID string `json:"id"`
	// Raw has a multi-line tag.
	Raw string `json:"raw"
	 	 extra:"{alert(1)}"`
	// Fn is a function-typed field whose signature spans lines.
	Fn func(
		a int,
		b string,
	) error
}

// Do takes an argument named like a tag and returns <error>.
//
// <div>
// import fs from "node:fs"
// </div>
//
// <b>export</b> is the first word after a tag.
//
// [imp](javascript:alert(1))ort x from "y"
//
// See [one][r1].
//
// [r1]: javascript&colon;alert(1)
func Do(input map[string]struct{}) (result struct {
	Code int
}, err error) {
	return result, nil
}
