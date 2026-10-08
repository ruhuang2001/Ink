package main

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruhuang/ink/server/internal/platform/config"
	"github.com/ruhuang/ink/server/internal/platform/seed"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "dev" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/seed dev")
		os.Exit(1)
	}

	if err := config.LoadDotEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "load .env: %v\n", err)
		os.Exit(1)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelConnect()

	db, err := pgxpool.New(connectCtx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(connectCtx); err != nil {
		fmt.Fprintf(os.Stderr, "ping database: %v\n", err)
		os.Exit(1)
	}

	serverDir := filepath.Dir(config.ResolveProjectPath(".env.example"))
	credentialsPath := cmp.Or(os.Getenv("INK_DEV_ADMIN_CREDENTIALS_PATH"), filepath.Join(serverDir, ".dev-admin-password"))
	seedCtx, cancelSeed := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelSeed()

	result, err := seed.EnsureDevAdmin(seedCtx, db, seed.DevAdminOptions{
		CredentialsPath: credentialsPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "run seed: %v\n", err)
		os.Exit(1)
	}

	if result.Created {
		fmt.Println("seeded development admin account")
		fmt.Printf("admin login: %s\n", result.Login)
		if result.CredentialsPath != "" {
			fmt.Printf("credentials saved to: %s\n", result.CredentialsPath)
			fmt.Println("read the initial password from the credentials file")
		}
		return
	}

	fmt.Printf("development admin account already exists: %s\n", result.Login)
	if result.CredentialsPath != "" && credentialsFileExists(result.CredentialsPath) {
		fmt.Printf("saved credentials: %s\n", result.CredentialsPath)
	}
}

func credentialsFileExists(path string) bool {
	if path == "" {
		return false
	}

	_, err := os.Stat(path)
	return err == nil
}
