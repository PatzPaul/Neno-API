package server

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	in := feedCursor{Lang: "sw", At: time.Date(2026, 9, 25, 18, 43, 0, 123456000, time.UTC), ID: uuid.New()}
	out, err := decodeCursor(encodeCursor(in))
	if err != nil {
		t.Fatal(err)
	}
	if out.Lang != in.Lang || !out.At.Equal(in.At) || out.ID != in.ID {
		t.Fatalf("round trip mismatch: %+v != %+v", out, in)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "!!!", "e30", encodeCursor(feedCursor{Lang: "sw"})} { // e30 = "{}"
		if _, err := decodeCursor(s); err == nil {
			t.Errorf("decodeCursor(%q) accepted", s)
		}
	}
}

func TestParseKinds(t *testing.T) {
	s := " verse, egw_quote ,,"
	got, err := parseKinds(&s)
	if err != nil || len(got) != 2 || got[0] != "verse" || got[1] != "egw_quote" {
		t.Fatalf("got %v, %v", got, err)
	}
	bad := "verse,tiktok"
	if _, err := parseKinds(&bad); err == nil {
		t.Fatal("unknown kind accepted")
	}
	if got, _ := parseKinds(nil); got == nil || len(got) != 0 {
		t.Fatalf("nil kinds should be empty non-nil slice, got %#v", got)
	}
}

func TestNormLang(t *testing.T) {
	cases := map[string]struct {
		want string
		ok   bool
	}{"": {"sw", true}, "sw-TZ": {"sw", true}, "EN": {"en", true}, "fr": {"fr", true}, "x": {"x", false}, "sw;drop": {"sw;drop", false}}
	for in, c := range cases {
		in := in
		got, ok := normLang(&in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("normLang(%q) = %q,%v want %q,%v", in, got, ok, c.want, c.ok)
		}
	}
	if got, ok := normLang(nil); got != "sw" || !ok {
		t.Errorf("nil lang should default to sw")
	}
}
