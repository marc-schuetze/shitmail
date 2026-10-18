package api

import (
	"context"
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marc-schuetze/shitmail/internal/config"
	"github.com/marc-schuetze/shitmail/internal/domain"
	"github.com/marc-schuetze/shitmail/internal/storage"
)

func TestAgentTokenLifecycleAndIsolation(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	cfg := &config.Config{SMTPDomain: "test.local", AdminGroup: "admins"}
	router := NewRouter(cfg, db.Mailboxes, db.Emails, nil, embed.FS{}, nil, db.Settings, db.Tokens)
	request := func(method, path, uid, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if uid != "" {
			req.Header.Set("X-Authentik-Uid", uid)
			req.Header.Set("X-Authentik-Groups", "admins")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	check := func(w *httptest.ResponseRecorder, want int) {
		t.Helper()
		if w.Code != want {
			t.Fatalf("status %d, want %d: %s", w.Code, want, w.Body.String())
		}
	}
	for _, owner := range []string{"alice", "bob"} {
		mb := domain.NewMailboxWithLocal(owner, "test.local", time.Hour)
		mb.Owner = owner
		if err := db.Mailboxes.Create(ctx, mb); err != nil {
			t.Fatal(err)
		}
	}
	check(request("POST", "/api/v1/tokens", "", "", `{"name":"test"}`), 401)
	check(request("POST", "/api/v1/tokens", "alice", "", `{"name":"test","expiresInDays":366}`), 400)
	w := request("POST", "/api/v1/tokens", "alice", "", `{"name":"agent","expiresInDays":7}`)
	check(w, 201)
	var created struct {
		Token  storage.APIToken `json:"token"`
		Secret string           `json:"secret"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Secret) != 67 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("invalid secret/response cache policy")
	}
	w = request("GET", "/api/v1/tokens", "alice", "", "")
	check(w, 200)
	if strings.Contains(w.Body.String(), created.Secret) {
		t.Fatal("list leaked token")
	}
	w = request("GET", "/api/v1/tokens", "bob", "", "")
	check(w, 200)
	if strings.Contains(w.Body.String(), created.Token.ID) {
		t.Fatal("list leaked other user's token")
	}
	check(request("GET", "/api/agent/v1/mailboxes", "alice", "", ""), 401)
	check(request("GET", "/api/agent/v1/mailboxes", "alice", "sm_"+strings.Repeat("a", 64), ""), 401)
	w = request("GET", "/api/agent/v1/mailboxes", "bob", created.Secret, "")
	check(w, 200)
	if strings.Contains(w.Body.String(), "bob@test.local") || !strings.Contains(w.Body.String(), "alice@test.local") {
		t.Fatal("bearer inherited forged SSO/admin identity")
	}
	check(request("GET", "/api/agent/v1/mailbox/bob@test.local/emails", "bob", created.Secret, ""), 404)
	check(request("DELETE", "/api/agent/v1/mailbox/alice@test.local", "", created.Secret, ""), 405)
	check(request("POST", "/api/v1/tokens", "", created.Secret, `{"name":"escalation"}`), 401)
	mb, _ := db.Mailboxes.FindByAddress(ctx, "alice@test.local")
	email := &domain.Email{ID: "mail-1", MailboxID: mb.ID, From: "login@example.com", To: mb.Address, Subject: "Login", BodyHTML: `<a href="https://example.com/login?token=abc">Sign in</a>`, Headers: map[string]string{}, ReceivedAt: time.Now()}
	if err := db.Emails.Save(ctx, email); err != nil {
		t.Fatal(err)
	}
	check(request("GET", "/api/agent/v1/mailbox/alice@test.local/emails/mail-1", "", created.Secret, ""), 200)
	persisted, _ := db.Emails.FindByID(ctx, email.ID)
	if persisted.IsRead {
		t.Fatal("agent read changed unread state")
	}
	check(request("DELETE", "/api/v1/tokens/"+created.Token.ID, "bob", "", ""), 204)
	check(request("GET", "/api/agent/v1/mailboxes", "", created.Secret, ""), 200)
	check(request("DELETE", "/api/v1/tokens/"+created.Token.ID, "alice", "", ""), 204)
	check(request("GET", "/api/agent/v1/mailboxes", "alice", created.Secret, ""), 401)
	_, expired, err := db.Tokens.Create(ctx, "alice", "expired", -time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	check(request("GET", "/api/agent/v1/mailboxes", "", expired, ""), http.StatusUnauthorized)
}
