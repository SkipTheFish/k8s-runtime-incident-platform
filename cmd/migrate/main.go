package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"observability-platform/internal/database"
)

func main() {
	command := flag.String("command", "up", "migration command: up or status")
	timeout := flag.Duration("timeout", 30*time.Second, "database operation timeout")
	flag.Parse()
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		exitError(fmt.Errorf("POSTGRES_DSN is required"))
	}
	db, err := database.Open(dsn)
	if err != nil {
		exitError(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	switch *command {
	case "up":
		err = database.Up(ctx, db)
	case "status":
		err = database.Status(ctx, db)
	default:
		err = fmt.Errorf("unsupported migration command %q", *command)
	}
	if err != nil {
		exitError(err)
	}
}

func exitError(err error) {
	fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
	os.Exit(1)
}
