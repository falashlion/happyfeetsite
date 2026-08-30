// cmd/migrate/main.go — Database migration runner using golang-migrate.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/happyfeet/api/pkg/config"
)

func main() {
	direction := flag.String("direction", "up", "up | down | version | force")
	steps := flag.Int("steps", 0, "Number of migration steps (0 = all)")
	forceVersion := flag.Int("force", -1, "Force set version number")
	migrationsPath := flag.String("path", "migrations", "Path to migrations directory")
	flag.Parse()

	config.LoadDotenv(".")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://happyfeet:happyfeet_dev_secret@localhost:5432/happyfeet?sslmode=disable"
	}

	m, err := migrate.New("file://"+*migrationsPath, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: create: %v\n", err)
		os.Exit(1)
	}
	defer m.Close()

	switch *direction {
	case "up":
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
	case "down":
		if *steps > 0 {
			err = m.Steps(-*steps)
		} else {
			err = m.Down()
		}
	case "version":
		ver, dirty, vErr := m.Version()
		if vErr != nil {
			fmt.Fprintf(os.Stderr, "migrate: version: %v\n", vErr)
			os.Exit(1)
		}
		fmt.Printf("version=%d dirty=%v\n", ver, dirty)
		return
	case "force":
		if *forceVersion < 0 {
			fmt.Fprintln(os.Stderr, "migrate: -force must be >= 0")
			os.Exit(1)
		}
		err = m.Force(*forceVersion)
	default:
		fmt.Fprintf(os.Stderr, "migrate: unknown direction %q\n", *direction)
		os.Exit(1)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("migrate: done")
}
