package contract

import (
	"regexp"
	"strings"
)

// A block is one fenced block in a Markdown file.
type block struct {
	info string // the first word after the opening fence
	line int    // the line of the opening fence
	body []bodyLine
}

type bodyLine struct {
	text string // trimmed
	line int
}

// A liveLine is a line outside any fence and any HTML comment, kept so the caller can look for
// obligations and headings without a second pass that would have to repeat the fence rules.
type liveLine struct {
	text string
	line int
}

var (
	fenceOpen  = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	fenceClose = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})[ \t]*$")
	atx        = regexp.MustCompile(`^ {0,3}#{1,6}(?:[ \t]+(.*))?$`)
	atxClose   = regexp.MustCompile(`(?:^|[ \t]+)#+[ \t]*$`)
	setext     = regexp.MustCompile(`^ {0,3}(?:=+|-+)[ \t]*$`)
)

// scan reads the little Markdown the Contract needs, with no Markdown library: fenced blocks under
// CommonMark's fence rules, and the lines outside them. A fence inside an HTML comment is dead, so
// commenting out a block retires it.
func scan(text string) (blocks []block, live []liveLine) {
	var (
		inComment bool
		open      *block
		openChar  byte
		openLen   int
	)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, raw := range lines {
		no := i + 1
		line := strings.TrimSuffix(raw, "\r")

		if open != nil {
			if m := fenceClose.FindStringSubmatch(line); m != nil && m[1][0] == openChar && len(m[1]) >= openLen {
				blocks = append(blocks, *open)
				open = nil
				continue
			}
			if t := strings.TrimSpace(line); t != "" && !strings.HasPrefix(t, "#") {
				open.body = append(open.body, bodyLine{text: t, line: no})
			}
			continue
		}

		var ok bool
		line, inComment, ok = outsideComments(line, inComment)
		if !ok {
			continue
		}
		live = append(live, liveLine{text: line, line: no})

		if m := fenceOpen.FindStringSubmatch(line); m != nil && !(m[1][0] == '`' && strings.Contains(m[2], "`")) {
			info := ""
			if f := strings.Fields(m[2]); len(f) > 0 {
				info = f[0]
			}
			open = &block{info: info, line: no}
			openChar, openLen = m[1][0], len(m[1])
		}
	}
	// CommonMark runs an unclosed fence to the end of the document.
	if open != nil {
		blocks = append(blocks, *open)
	}
	return blocks, live
}

// reasons gives the reason a declaration in each block carries, keyed by the line of the block's
// opening fence: the nearest heading above the block plus the prose between them, or "" when no
// heading is above it. It reads the live lines scan returned, so fences and comments hide prose
// and headings here the same way they hide obligations.
func reasons(blocks []block, live []liveLine) map[int]string {
	opens := map[int]bool{}
	for _, b := range blocks {
		opens[b.line] = true
	}
	why := map[int]string{}
	var (
		heading    string
		hasHeading bool
		prose      []string
		proseLine  int // the line of the last prose read, 0 before any
	)
	for _, ll := range live {
		// A setext underline needs prose on the line right above it.
		afterProse := proseLine > 0 && proseLine == ll.line-1
		switch m := atx.FindStringSubmatch(ll.text); {
		case opens[ll.line]:
			why[ll.line] = reason(hasHeading, heading, prose)
			// Prose read before a block explains that block, not the next one.
			prose = nil
		case afterProse && setext.MatchString(ll.text):
			heading, hasHeading = prose[len(prose)-1], true
			prose = nil
		case m != nil:
			heading, hasHeading = strings.TrimSpace(atxClose.ReplaceAllString(m[1], "")), true
			prose = nil
		default:
			if t := strings.TrimSpace(ll.text); t != "" {
				prose = append(prose, t)
				proseLine = ll.line
			}
		}
	}
	return why
}

func reason(hasHeading bool, heading string, prose []string) string {
	if !hasHeading {
		return ""
	}
	return strings.Join(strings.Fields(heading+" "+strings.Join(prose, " ")), " ")
}

// outsideComments returns the part of a line that is live Markdown, whether an HTML comment is still
// open after it, and false when the whole line sits inside a comment.
func outsideComments(line string, inComment bool) (string, bool, bool) {
	if inComment {
		end := strings.Index(line, "-->")
		if end < 0 {
			return "", true, false
		}
		line = line[end+3:]
	}
	for {
		start := strings.Index(line, "<!--")
		if start < 0 {
			return line, false, true
		}
		end := strings.Index(line[start+4:], "-->")
		if end < 0 {
			return line[:start], true, true
		}
		line = line[:start] + line[start+4+end+3:]
	}
}
