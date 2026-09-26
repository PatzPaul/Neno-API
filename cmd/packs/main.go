// Command packs builds the offline SQLite content packs and records them in the `packs` manifest.
//
//	packs build --out /var/lib/neno-api/packs --base-url http://159.65.58.51:8090 [--only bible-SUV]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PatzPaul/Neno-API/internal/packs"
	"github.com/PatzPaul/Neno-API/internal/store"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "build" {
		fmt.Fprintln(os.Stderr, "usage: packs build --out DIR --base-url URL [--only SLUG]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	out := fs.String("out", envOr("PACKS_DIR", "/var/lib/neno-api/packs"), "output directory (served at /packs/)")
	base := fs.String("base-url", os.Getenv("PACKS_BASE_URL"), "public API origin used in pack URLs")
	only := fs.String("only", "", "build a single pack slug")
	_ = fs.Parse(os.Args[2:])

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if *base == "" {
		log.Fatal("--base-url (or PACKS_BASE_URL) is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	b := &packs.Builder{Q: store.New(pool), Out: *out, BaseURL: *base}
	results, err := b.Build(ctx, *only)
	for _, r := range results {
		fmt.Printf("%-10s %-20s v%-3d %8d bytes  %s\n", r.Status, r.Slug, r.Version, r.Bytes, r.URL)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
