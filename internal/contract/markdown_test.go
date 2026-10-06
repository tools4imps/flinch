package contract

import (
	"reflect"
	"strings"
	"testing"
)

// md turns a readable fixture into Markdown. Fixtures write three single quotes where the Markdown
// has three backticks, so they can sit inside Go raw strings.
func md(s string) string { return strings.ReplaceAll(s, "'''", "```") }

func infos(bs []block) []string {
	var out []string
	for _, b := range bs {
		out = append(out, b.info)
	}
	return out
}

func TestScanFollowsCommonMarkFences(t *testing.T) {
	for _, c := range []struct {
		name string
		text string
		want []string
	}{
		{"backticks", "'''covers\na\n'''\n", []string{"covers"}},
		{"tildes", "~~~covers\na\n~~~\n", []string{"covers"}},
		{"info string is the first word", "''' covers extra words\na\n'''\n", []string{"covers"}},
		{"up to three spaces of indent", "   '''covers\na\n   '''\n", []string{"covers"}},
		{"four spaces is code, not a fence", "    '''covers\na\n", nil},
		{"two backticks is no fence", "``covers\na\n``\n", nil},
		{"a longer closing fence closes", "'''covers\na\n`````\n'''unpromised\nb\n'''\n", []string{"covers", "unpromised"}},
		{"a shorter closing fence doesn't", "````covers\n'''\n'''unpromised\n````\n", []string{"covers"}},
		{"the other character doesn't close", "~~~covers\n'''\n~~~\n", []string{"covers"}},
		{"a closing fence takes no info string", "'''covers\n''' x\n'''\n", []string{"covers"}},
		{"backtick info can't hold a backtick", "'''covers`x\na\n", nil},
		{"tilde info can hold a backtick", "~~~covers`x\na\n~~~\n", []string{"covers`x"}},
		{"an unclosed fence runs to the end", "'''covers\na\n", []string{"covers"}},
		{"a fence in a comment is dead", "<!--\n'''covers\na\n'''\n-->\n", nil},
		{"a one-line comment is dead too", "<!-- '''covers -->\na\n", nil},
		{"text after a comment closes is live", "<!-- x --> '''covers\na\n'''\n", []string{"covers"}},
		{"a comment opener inside a fence is text", "'''covers\n<!--\n'''\n'''unpromised\nb\n'''\n", []string{"covers", "unpromised"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			bs, _ := scan(md(c.text))
			if got := infos(bs); !reflect.DeepEqual(got, c.want) {
				t.Errorf("blocks = %q, want %q", got, c.want)
			}
		})
	}
}

func TestScanSkipsBlankAndCommentLinesInsideBlocks(t *testing.T) {
	bs, _ := scan(md("'''covers\n\n  # why\n  a  \n#b\nc # stays\n'''\n"))
	want := []bodyLine{{"a", 4}, {"c # stays", 6}}
	if !reflect.DeepEqual(bs[0].body, want) {
		t.Errorf("body = %v, want %v", bs[0].body, want)
	}
}

func TestReasonsGiveEachBlockItsHeadingAndProse(t *testing.T) {
	text := md(`'''unpromised
x
'''

# First heading #

Some prose
over two lines.

'''unpromised
x
'''

Prose after a block.

'''equivalent
x
'''

Setext
======

'''equivalent
x
'''
<!-- a comment isn't prose -->
'''equivalent
x
'''
`)
	bs, live := scan(text)
	why := reasons(bs, live)
	var got []string
	for _, b := range bs {
		got = append(got, why[b.line])
	}
	want := []string{
		"",
		"First heading Some prose over two lines.",
		"First heading Prose after a block.",
		"Setext",
		"Setext",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reasons = %q, want %q", got, want)
	}
}

func TestScanReturnsLiveLinesOnly(t *testing.T) {
	_, live := scan(md("a\n'''x\nb\n'''\n<!-- c\nd -->e\n"))
	var got []string
	for _, l := range live {
		got = append(got, l.text)
	}
	// The line that opens a comment keeps its text before "<!--", which here is nothing.
	want := []string{"a", "```x", "", "e"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("live = %q, want %q", got, want)
	}
}
