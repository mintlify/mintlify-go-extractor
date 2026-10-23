package extract

import (
	"path"
	"regexp"
	"strings"
)

var (
	majorVersionSuffix = regexp.MustCompile(`^v([2-9]|[1-9][0-9]+)$`)
	gopkgVersionSuffix = regexp.MustCompile(`\.v[0-9]+$`)
)

func compilePatterns(patterns []string, modulePath string) func(string) bool {
	var compiled []*regexp.Regexp
	for _, pattern := range patterns {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			compiled = append(compiled, patternRegexp(resolvePattern(pattern, modulePath)))
		}
	}
	return func(importPath string) bool {
		for _, re := range compiled {
			if re.MatchString(importPath) {
				return true
			}
		}
		return false
	}
}

func resolvePattern(pattern, modulePath string) string {
	if pattern == "." || pattern == "./" {
		return modulePath
	}
	if strings.HasPrefix(pattern, "./") {
		return path.Join(modulePath, pattern[2:])
	}
	return pattern
}

func patternRegexp(pattern string) *regexp.Regexp {
	expr := strings.ReplaceAll(regexp.QuoteMeta(pattern), `\.\.\.`, `.*`)
	if strings.HasSuffix(expr, `/.*`) {
		expr = strings.TrimSuffix(expr, `/.*`) + `(/.*)?`
	}
	return regexp.MustCompile(`^` + expr + `$`)
}

func matchesPrefixPattern(globs []string, target string) bool {
	for _, glob := range globs {
		glob = strings.TrimSuffix(strings.TrimSpace(glob), "/")
		if glob == "" {
			continue
		}
		elements := strings.Count(glob, "/") + 1
		parts := strings.SplitN(target, "/", elements+1)
		if len(parts) < elements {
			continue
		}
		if matched, err := path.Match(glob, strings.Join(parts[:elements], "/")); err == nil && matched {
			return true
		}
	}
	return false
}

func guessPackageName(importPath string) string {
	parts := strings.Split(importPath, "/")
	name := parts[len(parts)-1]
	if len(parts) > 1 && majorVersionSuffix.MatchString(name) {
		name = parts[len(parts)-2]
	}
	name = gopkgVersionSuffix.ReplaceAllString(name, "")
	name = strings.TrimPrefix(name, "go-")
	name = strings.TrimSuffix(name, "-go")
	return strings.Map(func(r rune) rune {
		if r == '-' || r == '.' {
			return -1
		}
		return r
	}, name)
}
