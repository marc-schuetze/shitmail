package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

type APIToken struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt"`
}

type TokenStore struct{ db *sql.DB }

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Create returns the secret once. Only its SHA-256 hash is stored.
func (s *TokenStore) Create(ctx context.Context, owner, name string, lifetime time.Duration) (APIToken, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return APIToken{}, "", err
	}
	secret := "sm_" + hex.EncodeToString(b)
	now := time.Now().UTC()
	token := APIToken{uuid.NewString(), name, now.Format(time.RFC3339), now.Add(lifetime).Format(time.RFC3339)}
	_, err := s.db.ExecContext(ctx, `INSERT INTO api_tokens (id, owner, name, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`, token.ID, owner, name, tokenHash(secret), token.CreatedAt, token.ExpiresAt)
	return token, secret, err
}

func (s *TokenStore) Owner(ctx context.Context, secret string) (string, error) {
	var owner string
	err := s.db.QueryRowContext(ctx, `SELECT owner FROM api_tokens WHERE token_hash = ? AND expires_at > ?`, tokenHash(secret), time.Now().UTC().Format(time.RFC3339)).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return owner, err
}

func (s *TokenStore) List(ctx context.Context, owner string) ([]APIToken, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, created_at, expires_at FROM api_tokens WHERE owner = ? ORDER BY created_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens := []APIToken{}
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.ExpiresAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (s *TokenStore) Revoke(ctx context.Context, owner, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE owner = ? AND id = ?`, owner, id)
	return err
}
