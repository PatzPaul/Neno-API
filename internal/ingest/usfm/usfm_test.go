package usfm

import (
	"strings"
	"testing"
)

// Fixture uses invented [SAMPLE] text; it only exercises USFM structure.
const fixture = `\id JHN [SAMPLE] fixture
\h Sample
\toc1 Sample Book
\mt1 Sample
\c 3
\s1 A heading that is not verse text
\p
\v 1 First [SAMPLE] verse\f + \fr 3:1 \ft a footnote that must vanish.\f* continues here.
\v 2 Second with \wj words of Jesus\wj* and \nd Lord\nd*.
\q1
\v 3 Poetry line one
\q2 poetry line two\x - \xo 3:3 \xt Isa 1:1\x*
\v 4 \w word|lemma="x" strong="G1"\w* done.
\b
\v 5-6 A bridged verse.
\v 7 \w C|strong="H1"\w*’\w était|strong="H2"\w* ¶ elided.
\c 4
\cl Chapter Four
\d A psalm title, skipped
\p
\v 1 Next chapter.
`

func TestParse(t *testing.T) {
	vs, err := Parse(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	want := []Verse{
		{"JHN", 3, 1, "First [SAMPLE] verse continues here."},
		{"JHN", 3, 2, "Second with words of Jesus and Lord."},
		{"JHN", 3, 3, "Poetry line one poetry line two"},
		{"JHN", 3, 4, "word done."},
		{"JHN", 3, 5, "A bridged verse."},
		{"JHN", 3, 7, "C’était elided."},
		{"JHN", 4, 1, "Next chapter."},
	}
	if len(vs) != len(want) {
		t.Fatalf("got %d verses: %+v", len(vs), vs)
	}
	for i := range want {
		if vs[i] != want[i] {
			t.Errorf("verse %d:\n got  %+v\n want %+v", i, vs[i], want[i])
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{`\c 1` + "\n" + `\v 1 no id`, `\id GEN` + "\n" + `\v 1 no chapter`} {
		if _, err := Parse(strings.NewReader(src)); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}
