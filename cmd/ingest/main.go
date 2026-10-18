// Command ingest imports licensed source content into the database, recording provenance.
//
//	ingest bible --code NENO --lang sw --name "…" --license "CC BY-SA 4.0" --license-url … \
//	             --attribution "…" --url https://ebible.org/Scriptures/swhonen_usfm.zip --zip content/…/swhonen_usfm.zip [--dry-run]
//
// A Bible import replaces all verses of that translation in one transaction, so re-running is safe.
package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PatzPaul/Neno-API/internal/ingest/usfm"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "bible" {
		fmt.Fprintln(os.Stderr, "usage: ingest bible [flags]")
		os.Exit(2)
	}
	if err := bible(os.Args[2:]); err != nil {
		log.Fatal(err)
	}
}

func bible(args []string) error {
	fs := flag.NewFlagSet("bible", flag.ExitOnError)
	code := fs.String("code", "", "translation code, e.g. NENO")
	lang := fs.String("lang", "", "language code (sw, en, fr)")
	name := fs.String("name", "", "display name")
	license := fs.String("license", "", "licence, e.g. \"CC BY-SA 4.0\" or \"public domain\"")
	licenseURL := fs.String("license-url", "", "licence URL")
	attribution := fs.String("attribution", "", "attribution text shown in the app")
	srcURL := fs.String("url", "", "where the archive was downloaded from")
	zipPath := fs.String("zip", "", "USFM zip archive (one book per .usfm/.sfm file)")
	dryRun := fs.Bool("dry-run", false, "parse and report, write nothing")
	show := fs.String("show", "", "with --dry-run: comma-separated refs to print, e.g. JHN.3.16,PSA.23.1")
	_ = fs.Parse(args)
	for flagName, v := range map[string]string{"code": *code, "lang": *lang, "name": *name, "license": *license, "attribution": *attribution, "url": *srcURL, "zip": *zipPath} {
		if v == "" {
			return fmt.Errorf("--%s is required", flagName)
		}
	}
	*code = strings.ToUpper(*code)

	sum, err := fileSHA256(*zipPath)
	if err != nil {
		return err
	}
	verses, err := parseZip(*zipPath)
	if err != nil {
		return err
	}

	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Only canonical books we know; deuterocanon/front matter are reported and skipped.
	known := map[string]bool{}
	rows, err := conn.Query(ctx, `SELECT osis FROM bible_books`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			return err
		}
		known[b] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	kept := verses[:0]
	skipped := map[string]int{}
	seen := map[string]bool{}
	dupes := 0
	for _, v := range verses {
		if !known[v.Book] {
			skipped[v.Book]++
			continue
		}
		ref := fmt.Sprintf("%s.%d.%d", v.Book, v.Chapter, v.Verse)
		if seen[ref] || v.Text == "" {
			dupes++ // bridged/duplicate numbering or empty verse: keep the first non-empty
			continue
		}
		seen[ref] = true
		kept = append(kept, v)
	}
	books := map[string]bool{}
	for _, v := range kept {
		books[v.Book] = true
		// Leftover markup means the parser missed a USFM construct: refuse rather than publish it.
		if strings.ContainsAny(v.Text, `\|`) {
			return fmt.Errorf("%s %d:%d still contains markup: %q", v.Book, v.Chapter, v.Verse, v.Text)
		}
	}
	log.Printf("%s: %d verses in %d books (sha256 %s); skipped %v; dropped %d duplicate/empty", *code, len(kept), len(books), sum[:12], skipped, dupes)
	if *dryRun {
		want := map[string]bool{}
		for _, r := range strings.Split(*show, ",") {
			want[strings.TrimSpace(r)] = true
		}
		for _, v := range kept {
			if ref := fmt.Sprintf("%s.%d.%d", v.Book, v.Chapter, v.Verse); want[ref] {
				fmt.Printf("%s %s\n", ref, v.Text)
			}
		}
		return nil
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var sourceID int
	err = tx.QueryRow(ctx, `
		INSERT INTO content_sources (kind, ref, url, sha256, license, attribution, license_url, fetched_at)
		VALUES ('bible', $1, $2, $3, $4, $5, NULLIF($6, ''), $7)
		ON CONFLICT (kind, ref, sha256) DO UPDATE SET ingested_at = now(), attribution = EXCLUDED.attribution
		RETURNING id`,
		*code, *srcURL, sum, *license, *attribution, *licenseURL, fileModTime(*zipPath)).Scan(&sourceID)
	if err != nil {
		return fmt.Errorf("content_sources: %w", err)
	}

	var trID int
	err = tx.QueryRow(ctx, `
		INSERT INTO bible_translations (code, lang, name, license, attribution, license_url, source_id)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
		ON CONFLICT (code) DO UPDATE SET lang = EXCLUDED.lang, name = EXCLUDED.name, license = EXCLUDED.license,
		  attribution = EXCLUDED.attribution, license_url = EXCLUDED.license_url, source_id = EXCLUDED.source_id,
		  version = bible_translations.version + 1
		RETURNING id`,
		*code, *lang, *name, *license, *attribution, *licenseURL, sourceID).Scan(&trID)
	if err != nil {
		return fmt.Errorf("bible_translations: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM verses WHERE translation_id = $1`, trID); err != nil {
		return err
	}
	n, err := tx.CopyFrom(ctx, pgx.Identifier{"verses"}, []string{"translation_id", "osis_ref", "book", "chapter", "verse", "text"},
		pgx.CopyFromSlice(len(kept), func(i int) ([]any, error) {
			v := kept[i]
			return []any{trID, fmt.Sprintf("%s.%d.%d", v.Book, v.Chapter, v.Verse), v.Book, v.Chapter, v.Verse, v.Text}, nil
		}))
	if err != nil {
		return fmt.Errorf("copy verses: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	log.Printf("%s: imported %d verses (translation id %d, source id %d)", *code, n, trID, sourceID)
	return nil
}

func parseZip(path string) ([]usfm.Verse, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	names := make([]string, 0, len(zr.File))
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		low := strings.ToLower(f.Name)
		if strings.HasSuffix(low, ".usfm") || strings.HasSuffix(low, ".sfm") {
			names = append(names, f.Name)
			files[f.Name] = f
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s: no .usfm files", path)
	}
	sort.Strings(names)
	var all []usfm.Verse
	for _, n := range names {
		rc, err := files[n].Open()
		if err != nil {
			return nil, err
		}
		vs, err := usfm.Parse(io.LimitReader(rc, 32<<20))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		all = append(all, vs...)
	}
	return all, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileModTime(path string) time.Time {
	if st, err := os.Stat(path); err == nil {
		return st.ModTime()
	}
	return time.Now()
}
