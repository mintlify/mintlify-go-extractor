package extract

import (
	"fmt"
	"path"
	"strings"
)

type symbolFilter []string

func compileSymbolPatterns(patterns []string) (symbolFilter, error) {
	var filter symbolFilter
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if _, err := path.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("-exclude-symbols: bad pattern %q: %w", pattern, err)
		}
		filter = append(filter, pattern)
	}
	return filter, nil
}

func (filter symbolFilter) matches(dir, name string) bool {
	candidates := []string{name}
	if dir != "." {
		candidates = append(candidates, dir+"."+name)
	}
	for _, pattern := range filter {
		for _, candidate := range candidates {
			if matched, _ := path.Match(pattern, candidate); matched {
				return true
			}
		}
	}
	return false
}

func (filter symbolFilter) apply(pkg *Package) {
	if len(filter) == 0 {
		return
	}
	match := func(name string) bool { return filter.matches(pkg.Dir, name) }
	pkg.Consts = filter.values(pkg.Consts, match)
	pkg.Vars = filter.values(pkg.Vars, match)
	pkg.Funcs = filter.funcs(pkg.Funcs, match)
	types := pkg.Types[:0]
	for _, typ := range pkg.Types {
		if match(typ.Name) {
			continue
		}
		member := func(name string) bool { return match(typ.Name + "." + name) }
		fields := typ.Fields[:0]
		for _, field := range typ.Fields {
			if !member(field.Name) {
				fields = append(fields, field)
			}
		}
		typ.Fields = fields
		typ.Methods = filter.funcs(typ.Methods, member)
		typ.Funcs = filter.funcs(typ.Funcs, match)
		typ.Consts = filter.values(typ.Consts, match)
		typ.Vars = filter.values(typ.Vars, match)
		types = append(types, typ)
	}
	pkg.Types = types
}

func (symbolFilter) funcs(funcs []Func, match func(string) bool) []Func {
	kept := funcs[:0]
	for _, fn := range funcs {
		if !match(fn.Name) {
			kept = append(kept, fn)
		}
	}
	return kept
}

func (symbolFilter) values(values []Value, match func(string) bool) []Value {
	kept := values[:0]
	for _, value := range values {
		for _, name := range value.Names {
			if !match(name) {
				kept = append(kept, value)
				break
			}
		}
	}
	return kept
}
