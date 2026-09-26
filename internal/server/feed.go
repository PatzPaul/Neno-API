package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatzPaul/Neno-API/internal/api"
	"github.com/PatzPaul/Neno-API/internal/store"
)

// feedCursor is the keyset position (publish_at, id) of the last item served, plus the language served.
// v1 ranks purely by publish_at; when ranking gains a score, add it here — clients treat the cursor as opaque.
type feedCursor struct {
	Lang string    `json:"l"`
	At   time.Time `json:"t"`
	ID   uuid.UUID `json:"i"`
}

func encodeCursor(c feedCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (feedCursor, error) {
	var c feedCursor
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if !langRe.MatchString(c.Lang) || c.At.IsZero() || c.ID == uuid.Nil {
		return c, errors.New("incomplete cursor")
	}
	return c, nil
}

func (c feedCursor) pgAt() pgtype.Timestamptz { return pgtype.Timestamptz{Time: c.At, Valid: true} }
func (c feedCursor) nullID() uuid.NullUUID    { return uuid.NullUUID{UUID: c.ID, Valid: true} }

func parseKinds(p *string) ([]string, error) {
	kinds := []string{}
	if p == nil {
		return kinds, nil
	}
	for _, k := range strings.Split(*p, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if !store.FeedKind(k).Valid() {
			return nil, fmt.Errorf("unknown kind %q", k)
		}
		kinds = append(kinds, k)
	}
	return kinds, nil
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func toFeedItem(r store.ListFeedRow) api.FeedItem {
	it := api.FeedItem{
		Id: r.ID, Kind: api.FeedKind(r.Kind), Lang: r.Lang, Kicker: r.Kicker, Body: r.Body,
		Source: textPtr(r.Source), RefLabel: textPtr(r.RefLabel),
		AltLang: textPtr(r.AltLang), AltBody: textPtr(r.AltBody),
		CtaLabel: textPtr(r.CtaLabel), CtaTarget: textPtr(r.CtaTarget),
		PublishAt: r.PublishAt.Time,
	}
	likes := int(r.LikeCount)
	it.LikeCount = &likes
	if len(r.Audience) > 0 {
		aud := r.Audience
		it.Audience = &aud
	}
	it.Media = toMedia(r.MediaID, r.MediaKind, r.MediaUrl, r.MediaHlsUrl, r.MediaBytes, r.MediaDurationS)
	return it
}

// toMedia builds an API media object from a LEFT JOINed media row; nil when there is none.
func toMedia(id uuid.NullUUID, kind store.NullMediaKind, url, hls pgtype.Text, bytes pgtype.Int8, dur pgtype.Int4) *api.Media {
	if !id.Valid {
		return nil
	}
	m := &api.Media{Id: id.UUID, Kind: api.MediaKind(kind.MediaKind), Url: url.String, HlsUrl: textPtr(hls)}
	if bytes.Valid {
		m.Bytes = &bytes.Int64
	}
	if dur.Valid {
		d := int(dur.Int32)
		m.DurationS = &d
	}
	return m
}

// feedTZ decides what "today" means for daily anchors (East Africa, UTC+3).
var feedTZ = func() *time.Location {
	if loc, err := time.LoadLocation("Africa/Nairobi"); err == nil {
		return loc
	}
	return time.FixedZone("EAT", 3*60*60)
}()

// arrangeFeed reorders one page for display. Pagination stays keyset-based on the fetched batch, so this
// never skips or repeats items across pages. On the first page the newest verse ("Aya ya Siku") leads,
// followed by today's Sabbath School item; the rest is interleaved so no kind appears twice in a row when
// another kind is available.
func arrangeFeed(items []api.FeedItem, firstPage bool, now time.Time) []api.FeedItem {
	rest := append([]api.FeedItem(nil), items...)
	out := make([]api.FeedItem, 0, len(items))
	take := func(match func(api.FeedItem) bool) {
		for i, it := range rest {
			if match(it) {
				out = append(out, it)
				rest = append(rest[:i], rest[i+1:]...)
				return
			}
		}
	}
	if firstPage {
		today := now.In(feedTZ).Format(time.DateOnly)
		take(func(it api.FeedItem) bool { return it.Kind == api.FeedKindVerse })
		take(func(it api.FeedItem) bool {
			return it.Kind == api.FeedKindSabbathSchool && it.PublishAt.In(feedTZ).Format(time.DateOnly) == today
		})
	}
	for len(rest) > 0 {
		var prev api.FeedKind
		if len(out) > 0 {
			prev = out[len(out)-1].Kind
		}
		i := 0
		for j, it := range rest {
			if it.Kind != prev {
				i = j
				break
			}
		}
		out = append(out, rest[i])
		rest = append(rest[:i], rest[i+1:]...)
	}
	return out
}
