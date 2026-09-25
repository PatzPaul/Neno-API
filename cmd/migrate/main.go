// Command migrate runs goose migrations embedded from db/migrations.
//
//	go run ./cmd/migrate [up|down|status|version|redo|reset]
package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/PatzPaul/Neno-API/db"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	conn, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	goose.SetBaseFS(db.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal(err)
	}
	if err := goose.RunContext(context.Background(), cmd, conn, "migrations", os.Args[2:]...); err != nil {
		log.Fatal(err)
	}
}
