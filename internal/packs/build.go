// Package packs builds the read-only SQLite content packs the app downloads for offline use.
//
// One pack per Bible translation (bible-<CODE>), EGW edition (egw-<lang>-<BOOK>) and hymnal (hymnal-<CODE>).
// A new version is published only when the content hash (over the rows, not the file bytes) changes.
package packs

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	_ "modernc.org/sqlite" // pure-Go driver "sqlite": keeps CGO_ENABLED=0 builds working

	"github.com/PatzPaul/Neno-API/internal/store"
)

// Schemas shipped to clients. Changing them is a breaking change for the app's pack reader.
var (
	metaSchema = `CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT)`

	bibleSchema = []string{
		metaSchema,
		`CREATE TABLE books (osis TEXT PRIMARY KEY, ord INT, testament TEXT, name TEXT, abbr TEXT, chapters INT)`,
		`CREATE TABLE verses (osis_ref TEXT PRIMARY KEY, book TEXT, chapter INT, verse INT, text TEXT)`,
		`CREATE INDEX verses_chapter_idx ON verses (book, chapter, verse)`,
	}
	egwSchema = []string{
		metaSchema,
		`CREATE TABLE paragraphs (refcode TEXT PRIMARY KEY, chapter INT, chapter_title TEXT, ord INT, page INT, text TEXT)`,
		`CREATE INDEX paragraphs_chapter_idx ON paragraphs (chapter, ord)`,
	}
	hymnalSchema = []string{
		metaSchema,
		`CREATE TABLE hymns (number INT PRIMARY KEY, title TEXT, original_title TEXT, category TEXT)`,
		`CREATE TABLE stanzas (number INT, idx INT, kind TEXT, text TEXT, PRIMARY KEY (number, idx))`,
	}
)

// Builder reads published content through q and writes pack files into Out.
type Builder struct {
	Q       *store.Queries
	Out     string // directory the API serves at /packs/
	BaseURL string // public API origin, e.g. http://159.65.58.51:8090
	Now     func() time.Time
}

// Result describes one pack after a build.
type Result struct {
	Slug    string
	Lang    string
	Version int32
	File    string
	URL     string
	Bytes   int64
	Sha256  string
	// Status: "new" (version published), "unchanged", or "rewritten" (same content, file was missing/corrupt).
	Status string
}

type table struct {
	name string
	cols []string
	rows [][]any
}

type spec struct {
	slug   string
	lang   string
	meta   map[string]string // stable metadata (hashed); version/built_at/content_hash are added at write time
	schema []string
	tables []table
}

// Build builds every pack, or only the one named by `only` when non-empty.
func (b *Builder) Build(ctx context.Context, only string) ([]Result, error) {
	if b.Now == nil {
		b.Now = time.Now
	}
	if err := os.MkdirAll(b.Out, 0o755); err != nil {
		return nil, err
	}
	specs, err := b.collect(ctx, only)
	if err != nil {
		return nil, err
	}
	if only != "" && len(specs) == 0 {
		return nil, fmt.Errorf("no pack source for slug %q", only)
	}
	out := make([]Result, 0, len(specs))
	for _, s := range specs {
		r, err := b.publish(ctx, s)
		if err != nil {
			return out, fmt.Errorf("%s: %w", s.slug, err)
		}
		out = append(out, r)
	}
	return out, nil
}

func (b *Builder) collect(ctx context.Context, only string) ([]spec, error) {
	var specs []spec
	want := func(slug string) bool { return only == "" || only == slug }

	trs, err := b.Q.ListTranslationsForPacks(ctx)
	if err != nil {
		return nil, err
	}
	for _, tr := range trs {
		slug := "bible-" + tr.Code
		if !want(slug) {
			continue
		}
		verses, err := b.Q.ListVersesForPack(ctx, tr.ID)
		if err != nil {
			return nil, err
		}
		if len(verses) == 0 {
			continue // never publish an empty pack
		}
		books, err := b.Q.ListBibleBooks(ctx, store.ListBibleBooksParams{TranslationID: tr.ID, Lang: tr.Lang})
		if err != nil {
			return nil, err
		}
		bt := table{name: "books", cols: []string{"osis", "ord", "testament", "name", "abbr", "chapters"}}
		for _, bk := range books {
			bt.rows = append(bt.rows, []any{bk.Osis, bk.Ord, bk.Testament, bk.Name, bk.Abbr, bk.Chapters})
		}
		vt := table{name: "verses", cols: []string{"osis_ref", "book", "chapter", "verse", "text"}}
		for _, v := range verses {
			vt.rows = append(vt.rows, []any{v.OsisRef, v.Book, v.Chapter, v.Verse, v.Text})
		}
		specs = append(specs, spec{
			slug: slug, lang: tr.Lang, schema: bibleSchema, tables: []table{bt, vt},
			meta: map[string]string{"slug": slug, "kind": "bible", "lang": tr.Lang, "translation": tr.Code},
		})
	}

	eds, err := b.Q.ListEgwEditionsForPacks(ctx)
	if err != nil {
		return nil, err
	}
	for _, ed := range eds {
		slug := "egw-" + ed.Lang + "-" + ed.BookCode
		if !want(slug) {
			continue
		}
		paras, err := b.Q.ListEgwParagraphsForPack(ctx, ed.ID)
		if err != nil {
			return nil, err
		}
		if len(paras) == 0 {
			continue
		}
		pt := table{name: "paragraphs", cols: []string{"refcode", "chapter", "chapter_title", "ord", "page", "text"}}
		for _, p := range paras {
			pt.rows = append(pt.rows, []any{p.Refcode, p.Chapter, text(p.ChapterTitle), p.Ord, int4(p.Page), p.Text})
		}
		specs = append(specs, spec{
			slug: slug, lang: ed.Lang, schema: egwSchema, tables: []table{pt},
			meta: map[string]string{
				"slug": slug, "kind": "egw", "lang": ed.Lang, "edition_id": strconv.Itoa(int(ed.ID)),
				"book_code": ed.BookCode, "title": ed.Title, "original_title": ed.OriginalTitle,
			},
		})
	}

	hymnals, err := b.Q.ListHymnalsForPacks(ctx)
	if err != nil {
		return nil, err
	}
	for _, h := range hymnals {
		slug := "hymnal-" + h.Code
		if !want(slug) {
			continue
		}
		hymns, err := b.Q.ListHymnsForPack(ctx, h.ID)
		if err != nil {
			return nil, err
		}
		if len(hymns) == 0 {
			continue
		}
		stanzas, err := b.Q.ListStanzasForPack(ctx, h.ID)
		if err != nil {
			return nil, err
		}
		ht := table{name: "hymns", cols: []string{"number", "title", "original_title", "category"}}
		for _, hy := range hymns {
			ht.rows = append(ht.rows, []any{hy.Number, hy.Title, text(hy.OriginalTitle), text(hy.Category)})
		}
		st := table{name: "stanzas", cols: []string{"number", "idx", "kind", "text"}}
		for _, s := range stanzas {
			st.rows = append(st.rows, []any{s.Number, s.Idx, s.Kind, s.Text})
		}
		specs = append(specs, spec{
			slug: slug, lang: h.Lang, schema: hymnalSchema, tables: []table{ht, st},
			meta: map[string]string{"slug": slug, "kind": "hymnal", "lang": h.Lang, "code": h.Code, "name": h.Name},
		})
	}
	return specs, nil
}

// contentHash is a stable digest of the schema, metadata and rows (independent of file layout and build time).
func contentHash(s spec) string {
	h := sha256.New()
	w := func(parts ...string) {
		for _, p := range parts {
			io.WriteString(h, p)
			h.Write([]byte{0x1f})
		}
		h.Write([]byte{0x1e})
	}
	w(s.schema...)
	keys := make([]string, 0, len(s.meta))
	for k := range s.meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		w(k, s.meta[k])
	}
	for _, t := range s.tables {
		w(t.name)
		for _, r := range t.rows {
			vals := make([]string, len(r))
			for i, v := range r {
				if v == nil {
					vals[i] = "\x00"
				} else {
					vals[i] = fmt.Sprint(v)
				}
			}
			w(vals...)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func fileName(slug string, version int32) string { return fmt.Sprintf("%s-v%d.sqlite", slug, version) }

func (b *Builder) publish(ctx context.Context, s spec) (Result, error) {
	hash := contentHash(s)
	latest, err := b.Q.LatestPack(ctx, s.slug)
	hasLatest := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}

	res := Result{Slug: s.slug, Lang: s.lang}
	if hasLatest && latest.ContentHash.Valid && latest.ContentHash.String == hash {
		res.Version = latest.Version
		res.File = fileName(s.slug, latest.Version)
		path := filepath.Join(b.Out, res.File)
		if sum, size, err := fileDigest(path); err == nil && sum == latest.Sha256 {
			res.URL, res.Bytes, res.Sha256, res.Status = latest.Url, size, sum, "unchanged"
			return res, nil
		}
		// Same content but the file is missing or differs (new host, disk loss): rebuild in place.
		if err := b.write(s, latest.Version, hash, path); err != nil {
			return res, err
		}
		sum, size, err := fileDigest(path)
		if err != nil {
			return res, err
		}
		res.URL, res.Bytes, res.Sha256, res.Status = b.url(res.File), size, sum, "rewritten"
		return res, b.Q.UpdatePackFile(ctx, store.UpdatePackFileParams{ID: latest.ID, Url: res.URL, Bytes: size, Sha256: sum})
	}

	res.Version = 1
	if hasLatest {
		res.Version = latest.Version + 1
	}
	res.File = fileName(s.slug, res.Version)
	path := filepath.Join(b.Out, res.File)
	if err := b.write(s, res.Version, hash, path); err != nil {
		return res, err
	}
	sum, size, err := fileDigest(path)
	if err != nil {
		return res, err
	}
	res.URL, res.Bytes, res.Sha256, res.Status = b.url(res.File), size, sum, "new"
	if err := b.Q.InsertPack(ctx, store.InsertPackParams{
		Slug: s.slug, Lang: s.lang, Version: res.Version, Url: res.URL, Bytes: size, Sha256: sum,
		ContentHash: pgtype.Text{String: hash, Valid: true},
	}); err != nil {
		return res, err
	}
	b.prune(s.slug, res.Version)
	return res, nil
}

func (b *Builder) url(file string) string {
	return strings.TrimRight(b.BaseURL, "/") + "/packs/" + file
}

// prune keeps the latest two versions on disk so clients mid-download of the previous one can finish.
func (b *Builder) prune(slug string, latest int32) {
	for v := latest - 2; v >= 1; v-- {
		_ = os.Remove(filepath.Join(b.Out, fileName(slug, v)))
	}
}

// write builds the SQLite file at a temp path and atomically renames it into place.
func (b *Builder) write(s spec, version int32, hash, path string) error {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	_ = os.Remove(tmp)
	defer os.Remove(tmp)

	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range s.schema {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	meta := map[string]string{"version": strconv.Itoa(int(version)), "built_at": b.Now().UTC().Format(time.RFC3339), "content_hash": hash}
	for k, v := range s.meta {
		meta[k] = v
	}
	for k, v := range meta {
		if _, err := tx.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)`, k, v); err != nil {
			tx.Rollback()
			return err
		}
	}
	for _, t := range s.tables {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(t.cols)), ",")
		stmt, err := tx.Prepare(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", t.name, strings.Join(t.cols, ","), ph))
		if err != nil {
			tx.Rollback()
			return err
		}
		for _, r := range t.rows {
			if _, err := stmt.Exec(r...); err != nil {
				stmt.Close()
				tx.Rollback()
				return fmt.Errorf("%s: %w", t.name, err)
			}
		}
		stmt.Close()
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if _, err := db.Exec(`VACUUM`); err != nil {
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func fileDigest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func text(t pgtype.Text) any {
	if !t.Valid {
		return nil
	}
	return t.String
}

func int4(i pgtype.Int4) any {
	if !i.Valid {
		return nil
	}
	return i.Int32
}
