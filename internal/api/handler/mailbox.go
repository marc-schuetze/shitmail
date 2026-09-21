package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/marc-schuetze/shitmail/internal/api/middleware"
	"github.com/marc-schuetze/shitmail/internal/config"
	"github.com/marc-schuetze/shitmail/internal/domain"
)

// MailboxHandler groups all mailbox-related HTTP handlers.
type MailboxHandler struct {
	cfg       *config.Config
	mailboxes domain.MailboxRepository
	emails    domain.EmailRepository
}

// NewMailboxHandler constructs a MailboxHandler.
func NewMailboxHandler(
	cfg *config.Config,
	mailboxes domain.MailboxRepository,
	emails domain.EmailRepository,
) *MailboxHandler {
	return &MailboxHandler{cfg: cfg, mailboxes: mailboxes, emails: emails}
}

// allowedTTLs is the set of valid per-request TTL hours.
var allowedTTLs = map[int]bool{1: true, 6: true, 24: true, 168: true}

// TTLForever is the ttlHours value meaning "keep this mailbox" (subscriptions).
// Stored as now+100y so the RFC3339 string ordering used by DeleteExpired holds.
const TTLForever = -1

const foreverDuration = 100 * 365 * 24 * time.Hour

type createMailboxRequest struct {
	LocalPart string `json:"localPart"`
	TTLHours  int    `json:"ttlHours"` // 0 = server default; 1, 6, 24, 168, or -1 = forever
}

// resolveTTL maps a request ttlHours to a duration; 0 means the server default.
func resolveTTL(hours int, def time.Duration) (time.Duration, error) {
	switch {
	case hours == 0:
		return def, nil
	case hours == TTLForever:
		return foreverDuration, nil
	case allowedTTLs[hours]:
		return time.Duration(hours) * time.Hour, nil
	}
	return 0, errors.New("invalid ttlHours: must be 1, 6, 24, 168, or -1 (forever)")
}

// Create handles POST /api/v1/mailbox
// Accepts an optional JSON body { "localPart": "...", "ttlHours": 24 }.
// If localPart is omitted a random address is generated.
// If ttlHours is omitted the server-configured default is used.
func (h *MailboxHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no SSO identity")
		return
	}
	var req createMailboxRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	req.LocalPart = strings.TrimSpace(strings.ToLower(req.LocalPart))

	// Resolve TTL: per-request value overrides server default.
	ttl, err := resolveTTL(req.TTLHours, h.cfg.MailboxTTL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Every address carries the owner's tag: <name>-<tag>@domain. The name is
	// user-chosen or random; the tag makes it collision-free across users.
	base := req.LocalPart
	if base == "" {
		base = domain.RandomLocalPart()
	} else if err := domain.ValidateLocalPart(base); err != nil {
		if errors.Is(err, domain.ErrInvalidLocalPart) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	local := base + "-" + user.Tag

	existing, err := h.mailboxes.FindByAddress(r.Context(), local+"@"+h.cfg.SMTPDomain)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if existing != nil {
		if !existing.IsExpired() {
			writeError(w, http.StatusConflict, "address already taken")
			return
		}
		// Expired row still holds the unique address; drop it (cascades its mail).
		_ = h.mailboxes.Delete(r.Context(), existing.ID)
	}

	mb := domain.NewMailboxWithLocal(local, h.cfg.SMTPDomain, ttl)
	mb.Owner = user.UID

	if err := h.mailboxes.Create(r.Context(), mb); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create mailbox")
		return
	}
	writeJSON(w, http.StatusCreated, mb)
}

// List handles GET /api/v1/mailboxes — the caller's own live mailboxes.
func (h *MailboxHandler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no SSO identity")
		return
	}
	list, err := h.mailboxes.ListByOwner(r.Context(), user.UID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mailboxes": list})
}

// Me handles GET /api/v1/me — identity as seen through the proxy headers.
func (h *MailboxHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no SSO identity")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"uid":      user.UID,
		"username": user.Username,
		"tag":      user.Tag,
		"admin":    user.Admin,
		"domain":   h.cfg.SMTPDomain,
	})
}

// Get handles GET /api/v1/mailbox/{address}
func (h *MailboxHandler) Get(w http.ResponseWriter, r *http.Request) {
	mb, ok := ResolveOwnedMailbox(w, r, h.mailboxes)
	if !ok {
		return
	}

	count, _ := h.emails.CountByMailbox(r.Context(), mb.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"mailbox":    mb,
		"emailCount": count,
	})
}

// SetTTL handles PATCH /api/v1/mailbox/{address} with { "ttlHours": n }.
// Re-bases the expiry on now; -1 keeps the mailbox (subscriptions).
func (h *MailboxHandler) SetTTL(w http.ResponseWriter, r *http.Request) {
	mb, ok := ResolveOwnedMailbox(w, r, h.mailboxes)
	if !ok {
		return
	}
	var req struct {
		TTLHours int `json:"ttlHours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TTLHours == 0 {
		writeError(w, http.StatusBadRequest, "ttlHours required")
		return
	}
	ttl, err := resolveTTL(req.TTLHours, h.cfg.MailboxTTL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mb.ExpiresAt = time.Now().UTC().Add(ttl)
	if err := h.mailboxes.UpdateExpiry(r.Context(), mb.ID, mb.ExpiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update mailbox")
		return
	}
	writeJSON(w, http.StatusOK, mb)
}

// Delete handles DELETE /api/v1/mailbox/{address}
func (h *MailboxHandler) Delete(w http.ResponseWriter, r *http.Request) {
	mb, ok := ResolveOwnedMailbox(w, r, h.mailboxes)
	if !ok {
		return
	}
	if err := h.mailboxes.Delete(r.Context(), mb.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete mailbox")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// helpers ----------------------------------------------------------------

// ResolveOwnedMailbox loads the mailbox named in the URL and checks that the
// SSO user owns it (admins may see all). Unowned, foreign and expired
// mailboxes all answer 404 so addresses cannot be probed.
func ResolveOwnedMailbox(w http.ResponseWriter, r *http.Request, repo domain.MailboxRepository) (*domain.Mailbox, bool) {
	user, ok := middleware.UserFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no SSO identity")
		return nil, false
	}
	mb, err := repo.FindByAddress(r.Context(), urlParam(r, "address"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return nil, false
	}
	if mb == nil || mb.IsExpired() || (mb.Owner != user.UID && !user.Admin) {
		writeError(w, http.StatusNotFound, "mailbox not found or expired")
		return nil, false
	}
	return mb, true
}

func urlParam(r *http.Request, key string) string {
	v := chi.URLParam(r, key)
	if decoded, err := url.PathUnescape(v); err == nil {
		return decoded
	}
	return v
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
