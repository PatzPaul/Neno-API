// Package usfm extracts plain verse text from USFM 3 Scripture files.
//
// Only the canonical reading text is kept: footnotes, cross references, headings, introductions,
// alternate verse numbers and word-level attributes are dropped; character styles (\wj, \nd, \add, \w …)
// keep their text. The output is one Verse per \v marker.
package usfm

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type Verse struct {
	Book    string // USFM book code from \id, e.g. JHN
	Chapter int
	Verse   int
	Text    string
}

var (
	// Notes and other spans removed together with their content. Matching is per kind so nested
	// character styles inside a note (\fr … \ft …) go with it.
	noteRe = regexp.MustCompile(`(?s)\\(f|fe|ef|x|ex|fig|va|vp|ca|cp|rq)\s.*?\\(?:f|fe|ef|x|ex|fig|va|vp|ca|cp|rq)\*`)
	// \w word|lemma="…" strong="…"\w* → keep "word".
	wordAttrRe = regexp.MustCompile(`\|[^\\|]*?(\\\+?w\*)`)
	// Closing character markers (\wj*, \+nd*, …).
	closeRe = regexp.MustCompile(`\\\+?[a-z]+[0-9]*\*`)
	// Any opening marker token.
	markerRe = regexp.MustCompile(`\\\+?([a-z]+[0-9]*) ?`) // the one space after a marker is syntax, not text
	spaceRe  = regexp.MustCompile(`\s+`)
	verseRe  = regexp.MustCompile(`^\s*([0-9]+)(?:[-–][0-9]+)?[a-z]?\s?`)
)

// Line-level markers whose whole line is not reading text.
var skipLine = map[string]bool{
	"h": true, "toc1": true, "toc2": true, "toc3": true, "toca1": true, "toca2": true, "toca3": true,
	"rem": true, "sts": true, "restore": true, "usfm": true, "ide": true,
	"mt": true, "mt1": true, "mt2": true, "mt3": true, "mt4": true, "mte": true, "mte1": true, "mte2": true,
	"ms": true, "ms1": true, "ms2": true, "ms3": true, "mr": true,
	"s": true, "s1": true, "s2": true, "s3": true, "s4": true, "sr": true, "r": true, "sp": true, "sd": true,
	"d": true, "cl": true, "cd": true,
	"imt": true, "imt1": true, "imt2": true, "imt3": true, "is": true, "is1": true, "is2": true,
	"ip": true, "ipi": true, "im": true, "imi": true, "ipq": true, "imq": true, "ipr": true,
	"iq": true, "iq1": true, "iq2": true, "ib": true, "ili": true, "ili1": true, "ili2": true,
	"iot": true, "io": true, "io1": true, "io2": true, "io3": true, "iex": true, "imte": true, "ie": true,
}

// Paragraph / poetry / list / table markers separate words; character markers (\w, \wj, \nd …) don't,
// so "C’\w était\w*" stays "C’était".
func breaksWords(name string) bool {
	base := strings.TrimRight(name, "0123456789")
	switch base {
	case "p", "m", "pi", "pm", "pmo", "pmc", "pmr", "pc", "pr", "mi", "nb", "cls", "ph", "b",
		"q", "qr", "qc", "qa", "qm", "qd", "li", "lh", "lf", "lim", "lik", "liv",
		"tr", "tc", "tcr", "th", "thr", "v", "c":
		return true
	}
	return false
}

// Parse reads one USFM book. Verses before the first \c or without an \id are an error.
func Parse(r io.Reader) ([]Verse, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	src := strings.ReplaceAll(string(raw), "\r\n", "\n")
	src = noteRe.ReplaceAllString(src, "") // notes attach directly to the preceding word
	src = wordAttrRe.ReplaceAllString(src, "$1")
	src = closeRe.ReplaceAllString(src, "")

	var (
		book    string
		chapter int
		out     []Verse
		cur     *strings.Builder
	)
	flush := func() {
		if cur != nil && len(out) > 0 {
			// "¶" is a paragraph sign some editions (KJV) carry inside verse text.
			out[len(out)-1].Text = strings.TrimSpace(spaceRe.ReplaceAllString(strings.ReplaceAll(cur.String(), "¶", " "), " "))
		}
	}

	sc := bufio.NewScanner(strings.NewReader(src))
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for ln := 1; sc.Scan(); ln++ {
		line := sc.Text()
		if first := markerRe.FindStringSubmatch(strings.TrimSpace(line)); first != nil && strings.HasPrefix(strings.TrimSpace(line), `\`) {
			name := first[1]
			switch {
			case name == "id":
				fields := strings.Fields(strings.TrimSpace(line)[len(`\id`):])
				if len(fields) == 0 {
					return nil, fmt.Errorf("line %d: empty \\id", ln)
				}
				book = strings.ToUpper(fields[0])
				continue
			case skipLine[name]:
				continue
			}
		}

		// Walk the line: text segments between markers.
		pos := 0
		for _, m := range markerRe.FindAllStringSubmatchIndex(line, -1) {
			name := line[m[2]:m[3]]
			if cur != nil {
				cur.WriteString(line[pos:m[0]])
				if breaksWords(name) {
					cur.WriteByte(' ')
				}
			}
			pos = m[1]
			switch name {
			case "c":
				flush()
				cur = nil
				rest := strings.TrimSpace(line[pos:])
				n, err := strconv.Atoi(strings.Fields(rest + " x")[0])
				if err != nil {
					return nil, fmt.Errorf("line %d: bad chapter %q", ln, rest)
				}
				chapter = n
				pos = len(line) // nothing else on a \c line is reading text
			case "v":
				if book == "" || chapter == 0 {
					return nil, fmt.Errorf("line %d: verse before \\id/\\c", ln)
				}
				vm := verseRe.FindStringSubmatch(line[pos:])
				if vm == nil {
					return nil, fmt.Errorf("line %d: bad verse number", ln)
				}
				n, _ := strconv.Atoi(vm[1])
				flush()
				cur = &strings.Builder{}
				out = append(out, Verse{Book: book, Chapter: chapter, Verse: n})
				pos += len(vm[0])
			}
		}
		if cur != nil && pos < len(line) {
			cur.WriteString(line[pos:])
			cur.WriteByte(' ')
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	flush()
	return out, nil
}
