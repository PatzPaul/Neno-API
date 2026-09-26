package packs

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PatzPaul/Neno-API/internal/store"
)

// Builds packs from the seeded database inside a rolled-back transaction, so the real manifest is untouched.
//
//	NENO_TEST_DATABASE_URL=$DATABASE_URL go test ./internal/packs/
func TestBuildPacks(t *testing.T) {
	dbURL := os.Getenv("NENO_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("NENO_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	out := t.TempDir()
	b := &Builder{Q: store.New(tx), Out: out, BaseURL: "http://test.invalid/"}
	first, err := b.Build(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]Result{}
	for _, r := range first {
		bySlug[r.Slug] = r
	}
	for _, slug := range []string{"bible-SUV", "bible-KJV", "egw-sw-SC", "egw-en-SC", "hymnal-NZK", "hymnal-SDAH"} {
		r, ok := bySlug[slug]
		if !ok {
			t.Fatalf("missing pack %s (got %v)", slug, first)
		}
		if r.URL != "http://test.invalid/packs/"+r.File || r.Bytes == 0 || len(r.Sha256) != 64 {
			t.Fatalf("bad result %+v", r)
		}
	}

	// Row counts in the SUV pack match the source.
	var srcVerses int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM verses v JOIN bible_translations t ON t.id = v.translation_id WHERE t.code = 'SUV'`).Scan(&srcVerses); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(out, bySlug["bible-SUV"].File))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(q string) (n int) {
		t.Helper()
		if err := db.QueryRow(q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM verses`); n != srcVerses {
		t.Fatalf("pack verses = %d, source = %d", n, srcVerses)
	}
	if n := count(`SELECT count(*) FROM books`); n != 66 {
		t.Fatalf("books = %d, want 66", n)
	}
	if n := count(`SELECT count(*) FROM books WHERE osis = 'JHN' AND chapters >= 3 AND name = 'Yohana'`); n != 1 {
		t.Fatal("JHN book row missing or wrong")
	}
	var kind, hash string
	_ = db.QueryRow(`SELECT value FROM meta WHERE key = 'kind'`).Scan(&kind)
	_ = db.QueryRow(`SELECT value FROM meta WHERE key = 'content_hash'`).Scan(&hash)
	if kind != "bible" || len(hash) != 64 {
		t.Fatalf("meta kind=%q content_hash=%q", kind, hash)
	}

	hdb, err := sql.Open("sqlite", filepath.Join(out, bySlug["hymnal-NZK"].File))
	if err != nil {
		t.Fatal(err)
	}
	defer hdb.Close()
	var hymns, stanzas int
	_ = hdb.QueryRow(`SELECT count(*) FROM hymns`).Scan(&hymns)
	_ = hdb.QueryRow(`SELECT count(*) FROM stanzas`).Scan(&stanzas)
	if hymns == 0 || stanzas == 0 {
		t.Fatalf("hymnal pack hymns=%d stanzas=%d", hymns, stanzas)
	}

	// Unchanged content → no new version.
	second, err := b.Build(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range second {
		if r.Status != "unchanged" || r.Version != bySlug[r.Slug].Version {
			t.Fatalf("second build %s: status=%s v%d (first v%d)", r.Slug, r.Status, r.Version, bySlug[r.Slug].Version)
		}
	}

	// Lost file with the same content is rewritten under the same version.
	_ = os.Remove(filepath.Join(out, bySlug["egw-sw-SC"].File))
	third, err := b.Build(ctx, "egw-sw-SC")
	if err != nil {
		t.Fatal(err)
	}
	if len(third) != 1 || third[0].Status != "rewritten" || third[0].Version != bySlug["egw-sw-SC"].Version {
		t.Fatalf("third build = %+v", third)
	}

	// Changed content → version bump.
	if _, err := tx.Exec(ctx, `UPDATE hymns SET title = title || ' [edited]' WHERE hymnal_id = (SELECT id FROM hymnals WHERE code = 'NZK') AND number = 1`); err != nil {
		t.Fatal(err)
	}
	fourth, err := b.Build(ctx, "hymnal-NZK")
	if err != nil {
		t.Fatal(err)
	}
	if fourth[0].Status != "new" || fourth[0].Version != bySlug["hymnal-NZK"].Version+1 {
		t.Fatalf("fourth build = %+v", fourth[0])
	}

	if _, err := b.Build(ctx, "nope"); err == nil {
		t.Fatal("unknown slug should error")
	}
}
