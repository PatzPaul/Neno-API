package server

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PatzPaul/Neno-API/internal/api"
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

func TestArrangeFeed(t *testing.T) {
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, feedTZ)
	mk := func(id string, kind api.FeedKind, at time.Time) api.FeedItem {
		return api.FeedItem{Id: uuid.MustParse("00000000-0000-4000-8000-0000000000" + id), Kind: kind, PublishAt: at}
	}
	yesterday := now.AddDate(0, 0, -1)
	in := []api.FeedItem{
		mk("01", api.FeedKindHymn, now), mk("02", api.FeedKindHymn, now), mk("03", api.FeedKindVerse, now),
		mk("04", api.FeedKindSabbathSchool, yesterday), mk("05", api.FeedKindSabbathSchool, now), mk("06", api.FeedKindVerse, now),
		mk("07", api.FeedKindEgwQuote, now),
	}
	out := arrangeFeed(in, true, now)
	ids := func(items []api.FeedItem) (s []string) {
		for _, it := range items {
			s = append(s, it.Id.String()[34:])
		}
		return s
	}
	if len(out) != len(in) {
		t.Fatalf("lost items: %v", ids(out))
	}
	if got := ids(out)[:2]; got[0] != "03" || got[1] != "05" {
		t.Fatalf("anchors = %v, want newest verse then today's Sabbath School", got)
	}
	for i := 1; i < len(out); i++ {
		if out[i].Kind == out[i-1].Kind {
			t.Fatalf("consecutive %s at %d: %v", out[i].Kind, i, ids(out))
		}
	}
	// Later pages keep order except for de-duplicating kinds; no anchors.
	if later := arrangeFeed(in, false, now); later[0].Id != in[0].Id || later[1].Kind == api.FeedKindHymn {
		t.Fatalf("later page = %v", ids(later))
	}
}
