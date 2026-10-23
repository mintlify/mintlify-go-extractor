package extract

import (
	"go/doc"
	"go/doc/comment"
	"regexp"
	"strings"
	"unicode"
)

const deprecatedPrefix = "Deprecated: "

var (
	linkCandidate  = regexp.MustCompile(`\[([^\[\]\n]+)\]`)
	linkDefinition = regexp.MustCompile(`^ {0,3}\[([^\[\]\n]+)\]:`)
	// A URI autolink, an email autolink, or an HTML open/closing tag, per
	// CommonMark (attribute values may be quoted and contain anything but their
	// quote); any other <...> is plain text.
	angleSpan = regexp.MustCompile(`^<(?:` +
		`[A-Za-z][A-Za-z0-9+.\-]{1,31}:[^\s<>]*` +
		`|[A-Za-z0-9.!#$%&'*+/=?^_{|}~\x60-]+@[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)*` +
		`|[A-Za-z][A-Za-z0-9-]*(?:\s+[A-Za-z_:][A-Za-z0-9_.:-]*(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=<>\x60]+))?)*\s*/?` +
		`|/[A-Za-z][A-Za-z0-9-]*\s*` +
		`)>`)
)

type docRenderer struct {
	importPath string
	modulePath string
	private    []string
	syntax     CommentSyntax
	parser     *comment.Parser
	printer    *comment.Printer
}

func newDocRenderer(dpkg *doc.Package, importPath, modulePath string, private []string, syntax CommentSyntax) *docRenderer {
	r := &docRenderer{
		importPath: importPath,
		modulePath: modulePath,
		private:    private,
		syntax:     syntax,
		parser:     dpkg.Parser(),
		printer:    dpkg.Printer(),
	}
	r.printer.HeadingLevel = 3
	r.printer.HeadingID = func(*comment.Heading) string { return "" }
	r.printer.DocLinkURL = r.docLinkURL
	return r
}

func (r *docRenderer) render(text string) (string, string) {
	body, deprecated := splitDeprecated(text)
	return r.markdown(body), r.markdown(deprecated)
}

func (r *docRenderer) markdown(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if r.syntax == CommentsGodoc {
		return strings.TrimSpace(string(r.printer.Markdown(r.parser.Parse(text))))
	}
	return r.resolveLinks(text)
}

// docLinkURL returns the go: URI for a doc link, or "" to leave a reference to
// a private dependency as plain text, since no public page can show it.
func (r *docRenderer) docLinkURL(link *comment.DocLink) string {
	importPath := link.ImportPath
	if importPath == "" {
		importPath = r.importPath
	}
	if r.isPrivate(importPath) {
		return ""
	}
	if link.Name == "" {
		return "go:" + importPath
	}
	name := link.Name
	if link.Recv != "" {
		name = link.Recv + "." + name
	}
	return "go:" + importPath + "#" + name
}

func (r *docRenderer) lookup(text string) (string, bool) {
	parsed := r.parser.Parse("[" + text + "]")
	if len(parsed.Content) != 1 {
		return "", false
	}
	paragraph, ok := parsed.Content[0].(*comment.Paragraph)
	if !ok || len(paragraph.Text) != 1 {
		return "", false
	}
	link, ok := paragraph.Text[0].(*comment.DocLink)
	if !ok {
		return "", false
	}
	url := r.docLinkURL(link)
	return url, url != ""
}

func (r *docRenderer) isPrivate(importPath string) bool {
	inModule := importPath == r.modulePath || strings.HasPrefix(importPath, r.modulePath+"/")
	return !inModule && matchesPrefixPattern(r.private, importPath)
}

// resolveLinks rewrites Go doc links in Markdown text to go: URIs. Fenced
// blocks, indented code, link reference definitions, code spans, existing
// links, autolinks, and raw URLs are left alone. Text is handled one paragraph
// at a time so code spans can follow the CommonMark rules: a backtick run is a
// span only when a run of exactly the same length closes it within the
// paragraph; otherwise it is literal text.
func (r *docRenderer) resolveLinks(text string) string {
	lines := strings.Split(strings.Trim(text, "\n"), "\n")
	defined := map[string]bool{}
	for _, line := range lines {
		if match := linkDefinition.FindStringSubmatch(line); match != nil {
			defined[strings.ToLower(match[1])] = true
		}
	}
	for i := 0; i < len(lines); {
		if end := fencedBlockEnd(lines, i); end > i {
			i = end
			continue
		}
		if !startsParagraph(lines[i]) {
			i++
			continue
		}
		end := i + 1
		for end < len(lines) && !atxHeading(lines[i]) && continuesParagraph(lines, end) {
			end++
		}
		resolved := strings.Split(r.resolveParagraph(strings.Join(lines[i:end], "\n"), defined), "\n")
		copy(lines[i:end], resolved)
		i = end
	}
	return strings.Join(lines, "\n")
}

func startsParagraph(line string) bool {
	return strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "\t") && lineIndent(line) < 4 && !linkDefinition.MatchString(line)
}

// An ATX heading is a single-line block of its own: indented code may follow it
// directly without a blank line, which a paragraph would absorb as a
// continuation.
func atxHeading(line string) bool {
	if lineIndent(line) >= 4 {
		return false
	}
	trimmed := strings.TrimLeft(line, " ")
	hashes := 0
	for hashes < len(trimmed) && trimmed[hashes] == '#' {
		hashes++
	}
	if hashes == 0 || hashes > 6 {
		return false
	}
	return len(trimmed) == hashes || trimmed[hashes] == ' ' || trimmed[hashes] == '\t'
}

// An indented line cannot start indented code inside a paragraph; it is a lazy
// continuation, which is how nested list items and wrapped list text appear. A
// fence or an ATX heading interrupts the paragraph instead.
func continuesParagraph(lines []string, i int) bool {
	line := lines[i]
	return strings.TrimSpace(line) != "" && fencedBlockEnd(lines, i) == i && !atxHeading(line) && !linkDefinition.MatchString(line)
}

func lineIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func fencedBlockEnd(lines []string, start int) int {
	if lineIndent(lines[start]) >= 4 {
		return start
	}
	fence := fenceMarker(strings.TrimLeft(lines[start], " "))
	if fence == "" {
		return start
	}
	for i := start + 1; i < len(lines); i++ {
		if closesFence(lines[i], fence) {
			return i + 1
		}
	}
	return len(lines)
}

// closesFence reports whether line closes a block opened by fence: at most
// three spaces of indent, a run of the same character at least as long, and
// nothing after it but spaces or tabs.
func closesFence(line, fence string) bool {
	if lineIndent(line) >= 4 {
		return false
	}
	trimmed := strings.TrimLeft(line, " ")
	run := 0
	for run < len(trimmed) && trimmed[run] == fence[0] {
		run++
	}
	return run >= len(fence) && strings.Trim(trimmed[run:], " \t") == ""
}

func fenceMarker(line string) string {
	for _, char := range []string{"`", "~"} {
		run := 0
		for run < len(line) && line[run] == char[0] {
			run++
		}
		if run >= 3 {
			return line[:run]
		}
	}
	return ""
}

// resolveParagraph finds code spans first, then links over the whole
// paragraph, the way a CommonMark parser meets them left to right: a backtick
// inside a link destination (already consumed when its "](" was reached)
// does not split the link, while a "]" inside a code span cannot close a
// link label. Candidates inside a span, a link, or a bare URL are left alone.
func (r *docRenderer) resolveParagraph(paragraph string, defined map[string]bool) string {
	spans := codeSpans(paragraph)
	inSpan := make([]bool, len(paragraph)+1)
	for _, span := range spans {
		for i := span[0]; i < span[1]; i++ {
			inSpan[i] = true
		}
	}
	links := markdownLinks(paragraph, inSpan)
	inURL := rawURLMask(paragraph)
	var out strings.Builder
	last, next := 0, 0
	for _, match := range linkCandidate.FindAllStringSubmatchIndex(paragraph, -1) {
		start, end := match[0], match[1]
		for next < len(links) && links[next][1] <= start {
			next++
		}
		if inSpan[start] || inSpan[end-1] || (next < len(links) && links[next][0] <= start) || escaped(paragraph, start) || inURL[start] {
			continue
		}
		text := paragraph[match[2]:match[3]]
		if defined[strings.ToLower(text)] {
			continue
		}
		url, ok := r.lookup(text)
		if !ok {
			continue
		}
		out.WriteString(paragraph[last:end])
		out.WriteString("(" + url + ")")
		last = end
	}
	out.WriteString(paragraph[last:])
	return out.String()
}

func codeSpans(text string) [][2]int {
	var spans [][2]int
	for i := 0; i < len(text); {
		if text[i] != '`' {
			i++
			continue
		}
		// An escaped backtick is literal and cannot open a span; the rest of
		// its run still can. Inside a span backslashes are literal, so closing
		// runs need no such check.
		if escaped(text, i) {
			i++
			continue
		}
		open := backtickRun(text, i)
		closing := -1
		for j := open; j < len(text); {
			if text[j] != '`' {
				j++
				continue
			}
			run := backtickRun(text, j)
			if run-j == open-i {
				closing = run
				break
			}
			j = run
		}
		if closing < 0 {
			i = open
			continue
		}
		spans = append(spans, [2]int{i, closing})
		i = closing
	}
	return spans
}

func backtickRun(text string, start int) int {
	end := start
	for end < len(text) && text[end] == '`' {
		end++
	}
	return end
}

// escaped reports whether the character at offset is preceded by an odd number
// of backslashes, so \\[Client] (an escaped backslash) still links.
func escaped(segment string, offset int) bool {
	count := 0
	for i := offset - 1; i >= 0 && segment[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

// rawURLMask marks the offsets that fall inside a bare URL such as
// https://acme.dev/[Client], which GFM autolinks as a whole: every byte of a
// whitespace-delimited word after it has shown "://" or started with "www.".
func rawURLMask(segment string) []bool {
	mask := make([]bool, len(segment)+1)
	wordStart, inURL := 0, false
	for i := 0; i < len(segment); i++ {
		if unicode.IsSpace(rune(segment[i])) {
			wordStart, inURL = i+1, false
			continue
		}
		if !inURL {
			word := segment[wordStart : i+1]
			inURL = strings.HasSuffix(word, "://") || (len(word) == 4 && strings.EqualFold(word, "www."))
		}
		mask[i+1] = inURL
	}
	return mask
}

// markdownLinks returns the ranges of inline links, reference links, images,
// autolinks (<https://...>), and inline HTML, which never get rewritten.
func markdownLinks(segment string, inSpan []bool) [][2]int {
	var links [][2]int
	brackets := matchPairs(segment, '[', ']', inSpan)
	parens := matchPairs(segment, '(', ')', nil)
	for i := 0; i < len(segment); i++ {
		if inSpan[i] {
			continue
		}
		switch segment[i] {
		case '\\':
			i++
		case '<':
			if end := angleSpanEnd(segment, i); end > i {
				links = append(links, [2]int{i, end})
				i = end - 1
			}
		case '[':
			end := brackets[i]
			if end < 0 {
				continue
			}
			switch {
			case end < len(segment) && segment[end] == '(':
				// Without a closing paren this is not a link, just text.
				if end = parens[end]; end < 0 {
					continue
				}
			case end < len(segment) && segment[end] == '[':
				if end = brackets[end]; end < 0 {
					continue
				}
			default:
				continue
			}
			links = append(links, [2]int{i, end})
			i = end - 1
		}
	}
	return links
}

// matchPairs returns, for each unescaped open byte, the offset just past its
// matching close byte, or -1 when it has none, in one pass over the segment.
// Offsets marked in skip (code spans, for brackets) neither open nor close.
func matchPairs(segment string, open, close byte, skip []bool) []int {
	ends := make([]int, len(segment))
	for i := range ends {
		ends[i] = -1
	}
	var stack []int
	for i := 0; i < len(segment); i++ {
		if skip != nil && skip[i] {
			continue
		}
		switch segment[i] {
		case '\\':
			i++
		case open:
			stack = append(stack, i)
		case close:
			if len(stack) > 0 {
				ends[stack[len(stack)-1]] = i + 1
				stack = stack[:len(stack)-1]
			}
		}
	}
	return ends
}

// angleSpanEnd returns the end of the autolink, HTML tag, or HTML comment that
// opens at open, or -1 when the angle bracket is plain text such as <[Client]>.
func angleSpanEnd(segment string, open int) int {
	rest := segment[open:]
	if strings.HasPrefix(rest, "<!--") {
		if end := strings.Index(rest[4:], "-->"); end >= 0 {
			return open + 4 + end + 3
		}
		return -1
	}
	if loc := angleSpan.FindStringIndex(rest); loc != nil {
		return open + loc[1]
	}
	return -1
}

func splitDeprecated(text string) (string, string) {
	lines := strings.Split(text, "\n")
	var body, deprecated []string
	for start := 0; start < len(lines); {
		if end := fencedBlockEnd(lines, start); end > start {
			body = append(body, lines[start:end]...)
			start = end
			continue
		}
		end := start
		for end < len(lines) && strings.TrimSpace(lines[end]) != "" && fencedBlockEnd(lines, end) == end {
			end++
		}
		if end == start {
			body = append(body, lines[start])
			start++
			continue
		}
		paragraph := lines[start:end]
		if deprecated == nil && strings.HasPrefix(paragraph[0], deprecatedPrefix) {
			deprecated = append([]string{strings.TrimPrefix(paragraph[0], deprecatedPrefix)}, paragraph[1:]...)
		} else {
			body = append(body, paragraph...)
		}
		start = end
	}
	return collapseBlankLines(body), strings.TrimSpace(strings.Join(deprecated, "\n"))
}

func collapseBlankLines(lines []string) string {
	var out []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		if strings.TrimSpace(line) == "" {
			line = ""
		}
		out = append(out, line)
	}
	return strings.Trim(strings.Join(out, "\n"), "\n")
}
