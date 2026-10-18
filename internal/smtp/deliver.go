package smtp

import (
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/marc-schuetze/shitmail/internal/domain"
	"github.com/marc-schuetze/shitmail/internal/metrics"
)

// Deliver parses one raw message and stores it for every recipient that has a
// live mailbox. Unknown or expired recipients are dropped silently — the
// upstream relay must never see a bounce. Shared by the SMTP session and the
// IMAP puller.
func (b *Backend) Deliver(from string, to []string, r io.Reader) error {
	if len(to) == 0 {
		return errors.New("no recipients")
	}

	parsed, err := Parse(r, ParseOptions{
		MaxAttachmentBytes:      b.maxAttachmentBytes,
		MaxTotalAttachmentBytes: b.maxTotalAttachmentBytes,
		MaxBodyBytes:            b.maxBodyBytes,
		DropAttachments:         b.dropAttachments,
	})
	if err != nil {
		return err
	}

	ctx := b.ctx

	for _, addr := range to {
		mb, err := b.mailboxes.FindByAddress(ctx, addr)
		if err != nil {
			slog.Error("deliver: lookup mailbox", "addr", addr, "error", err)
			continue
		}
		if mb == nil || mb.IsExpired() {
			slog.Debug("deliver: no active mailbox for address", "addr", addr)
			continue
		}

		email := &domain.Email{
			ID:         uuid.NewString(),
			MailboxID:  mb.ID,
			From:       coalesce(parsed.From, from),
			To:         addr,
			Subject:    parsed.Subject,
			BodyText:   parsed.TextBody,
			BodyHTML:   parsed.HTMLBody,
			Headers:    parsed.Headers,
			Size:       parsed.Size,
			IsRead:     false,
			ReceivedAt: time.Now().UTC(),
		}

		if err := b.emails.Save(ctx, email); err != nil {
			slog.Error("deliver: save email", "error", err)
			continue
		}

		// Persist attachments (failures are non-fatal).
		for _, att := range parsed.Attachments {
			a, err := b.emails.SaveAttachment(ctx, email.ID, &domain.AttachmentInput{
				Filename:    att.Filename,
				ContentType: att.ContentType,
				Data:        att.Data,
			})
			if err != nil {
				slog.Warn("deliver: save attachment", "filename", att.Filename, "error", err)
				continue
			}
			email.Attachments = append(email.Attachments, a)
		}

		metrics.EmailsReceived.WithLabelValues(b.domain).Inc()
		b.hub.BroadcastNewEmail(addr, email)
		slog.Info("deliver: email stored",
			"to", addr,
			"from", email.From,
			"subject", email.Subject,
			"attachments", len(email.Attachments),
			"skipped_attachments", parsed.Skipped,
		)
	}

	return nil
}

// Domain reports the mail domain this backend accepts.
func (b *Backend) Domain() string { return b.domain }
