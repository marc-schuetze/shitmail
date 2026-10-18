// Package imappull drains a catch-all IMAP mailbox into shitmail storage.
//
// Why it exists: the public relay can only deliver SMTP to hosts it can reach.
// Where shitmail runs on an internal host with no inbound path, mail lands in a
// catch-all mailbox on the relay side and this puller fetches it outbound.
package imappull

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/marc-schuetze/shitmail/internal/config"
	"github.com/marc-schuetze/shitmail/internal/smtp"
)

// Puller polls one IMAP mailbox and hands every message to the SMTP backend.
type Puller struct {
	addr            string
	user            string
	pass            string
	mailbox         string
	envelopeHeaders []string
	interval        time.Duration
	backend         *smtp.Backend
}

// New returns nil when IMAP_HOST is unset (SMTP-only deployment).
func New(cfg *config.Config, backend *smtp.Backend) *Puller {
	if cfg.IMAPHost == "" {
		return nil
	}
	addr := cfg.IMAPHost
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "993")
	}
	mailbox := cfg.IMAPMailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}
	interval := cfg.IMAPPollInterval
	if interval <= 0 {
		interval = time.Minute
	}
	return &Puller{
		addr:            addr,
		user:            cfg.IMAPUser,
		pass:            cfg.IMAPPass,
		mailbox:         mailbox,
		envelopeHeaders: cfg.IMAPEnvelopeHeaders,
		interval:        interval,
		backend:         backend,
	}
}

// Run polls until ctx is cancelled. Connection errors are logged and retried on
// the next tick — a dead relay must not take the web UI down.
//
// ponytail: poll + reconnect per cycle instead of a long-lived IMAP IDLE
// session. Worst-case latency is one interval; switch to IDLE if that matters.
func (p *Puller) Run(ctx context.Context) {
	slog.Info("imappull: started", "addr", p.addr, "mailbox", p.mailbox, "interval", p.interval)
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		if n, err := p.drain(ctx); err != nil {
			slog.Error("imappull: drain failed", "error", err)
		} else if n > 0 {
			slog.Info("imappull: drained messages", "count", n)
		}
		select {
		case <-ctx.Done():
			slog.Info("imappull: stopped")
			return
		case <-t.C:
		}
	}
}

// drain fetches every message in the mailbox, delivers it, and deletes it.
// A message is deleted only after Deliver returned without error, so a storage
// failure leaves the mail on the server for the next cycle.
func (p *Puller) drain(ctx context.Context) (int, error) {
	c, err := imapclient.DialTLS(p.addr, nil)
	if err != nil {
		return 0, fmt.Errorf("dial %s: %w", p.addr, err)
	}
	defer c.Close()

	if err := c.Login(p.user, p.pass).Wait(); err != nil {
		return 0, fmt.Errorf("login %s: %w", p.user, err)
	}
	// defer c.Logout() would send LOGOUT immediately — the call is evaluated at
	// the defer statement, only Wait is deferred.
	defer func() { _ = c.Logout().Wait() }()

	box, err := c.Select(p.mailbox, nil).Wait()
	if err != nil {
		return 0, fmt.Errorf("select %s: %w", p.mailbox, err)
	}
	if box.NumMessages == 0 {
		return 0, nil
	}

	var seqSet imap.SeqSet
	seqSet.AddRange(1, box.NumMessages)
	msgs, err := c.Fetch(seqSet, &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{{Peek: true}},
	}).Collect()
	if err != nil {
		return 0, fmt.Errorf("fetch: %w", err)
	}

	var delivered imap.SeqSet
	for _, msg := range msgs {
		if ctx.Err() != nil {
			break
		}
		raw := msg.FindBodySection(&imap.FetchItemBodySection{Peek: true})
		if raw == nil {
			slog.Warn("imappull: message without body section", "seq", msg.SeqNum)
			continue
		}
		rcpts, from := envelope(raw, p.envelopeHeaders, p.backend.Domain())
		if len(rcpts) == 0 {
			// Nothing addressed at our domain: drop it, do not loop on it forever.
			slog.Info("imappull: no recipient at domain, discarding", "seq", msg.SeqNum)
			delivered.AddNum(msg.SeqNum)
			continue
		}
		if err := p.backend.Deliver(from, rcpts, bytes.NewReader(raw)); err != nil {
			slog.Error("imappull: deliver", "seq", msg.SeqNum, "error", err)
			continue
		}
		delivered.AddNum(msg.SeqNum)
	}

	nums, _ := delivered.Nums()
	if len(nums) == 0 {
		return 0, nil
	}
	if err := c.Store(delivered, &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagDeleted},
	}, nil).Close(); err != nil {
		return len(nums), fmt.Errorf("store deleted: %w", err)
	}
	if err := c.Expunge().Close(); err != nil {
		return len(nums), fmt.Errorf("expunge: %w", err)
	}
	return len(nums), nil
}

// envelope extracts the recipients at our domain plus the sender.
//
// A catch-all mailbox rewrites nothing in the body, so the original envelope
// recipient survives only in a trace header: Postfix writes X-Original-To,
// Dovecot writes Delivered-To (which on a catch-all is the catch-all mailbox
// itself, hence lower priority). To/Cc are the last resort and are wrong for
// Bcc'd mail — that is the cost of the pull model.
func envelope(raw []byte, headerNames []string, domain string) (rcpts []string, from string) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, ""
	}
	from = firstAddress(msg.Header.Get("From"))

	suffix := "@" + strings.ToLower(domain)
	seen := map[string]bool{}
	add := func(values []string) {
		for _, v := range values {
			for _, addr := range parseAddresses(v) {
				if strings.HasSuffix(addr, suffix) && !seen[addr] {
					seen[addr] = true
					rcpts = append(rcpts, addr)
				}
			}
		}
	}

	for _, name := range headerNames {
		add(msg.Header[textproto.CanonicalMIMEHeaderKey(name)])
		if len(rcpts) > 0 {
			return rcpts, from
		}
	}
	add(msg.Header["To"])
	add(msg.Header["Cc"])
	return rcpts, from
}

func parseAddresses(value string) []string {
	var out []string
	if list, err := mail.ParseAddressList(value); err == nil {
		for _, a := range list {
			out = append(out, strings.ToLower(a.Address))
		}
		return out
	}
	// Trace headers are often bare addresses, sometimes in angle brackets.
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, "<>")
		if part != "" && strings.Contains(part, "@") {
			out = append(out, strings.ToLower(part))
		}
	}
	return out
}

func firstAddress(value string) string {
	addrs := parseAddresses(value)
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0]
}
