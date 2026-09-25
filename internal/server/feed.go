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
	if len(r.Audience) > 0 {
		aud := r.Audience
		it.Audience = &aud
	}
	if r.MediaID.Valid {
		m := &api.Media{Id: r.MediaID.UUID, Kind: api.MediaKind(r.MediaKind.MediaKind), Url: r.MediaUrl.String, HlsUrl: textPtr(r.MediaHlsUrl)}
		if r.MediaBytes.Valid {
			m.Bytes = &r.MediaBytes.Int64
		}
		if r.MediaDurationS.Valid {
			d := int(r.MediaDurationS.Int32)
			m.DurationS = &d
		}
		it.Media = m
	}
	return it
}
