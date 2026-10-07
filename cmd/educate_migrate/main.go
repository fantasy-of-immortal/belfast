// Local save migration. DSN is read from the environment, never logged.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"os"
)

func main() {
	apply := flag.Bool("apply", false, "archive and migrate; default is read-only dry-run")
	prepare := flag.Bool("prepare-schema", false, "explicitly apply formal schema migrations before inspection")
	schema := flag.String("schema", "azurlane_local", "existing database schema")
	commander := flag.Uint("commander", 0, "commander id")
	flag.Parse()
	if *commander == 0 {
		fmt.Fprintln(os.Stderr, "--commander is required")
		os.Exit(1)
	}
	if *prepare {
		store, err := db.InitDefaultStore(context.Background(), os.Getenv("EDUCATE_MIGRATION_DSN"), *schema)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		store.Pool.Close()
	}
	// Dry-run deliberately does not apply DDL or initialize a new schema.
	pool, err := db.OpenPostgresPool(context.Background(), os.Getenv("EDUCATE_MIGRATION_DSN"), *schema)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not connect to migration database")
		os.Exit(1)
	}
	defer pool.Close()
	db.DefaultStore = db.NewStore(pool)
	report, err := orm.MigrateEducateLegacy(uint32(*commander), *apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		panic(err)
	}
}
