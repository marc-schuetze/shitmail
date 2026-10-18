package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTokensPersistAsHashes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	token, secret, err := db.Tokens.Create(ctx, "owner", "agent", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	var hash string
	if err := db.sql.QueryRow(`SELECT token_hash FROM api_tokens WHERE id = ?`, token.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == secret || strings.Contains(hash, secret) || len(hash) != 64 {
		t.Fatal("secret not hashed")
	}
	db.Close()
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	owner, err := db.Tokens.Owner(ctx, secret)
	if err != nil || owner != "owner" {
		t.Fatal("token did not survive restart")
	}
}
