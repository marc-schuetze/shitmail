package smtp

import (
	"io"
	"strings"

	gosmtp "github.com/emersion/go-smtp"
)

// session handles a single SMTP client connection.
type session struct {
	backend *Backend
	from    string
	to      []string
}

func (s *session) AuthPlain(_, _ string) error {
	// shitmail is a receive-only MTA; we accept but do not verify credentials.
	return nil
}

func (s *session) Mail(from string, _ *gosmtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *session) Rcpt(to string, _ *gosmtp.RcptOptions) error {
	domain := s.backend.domain
	if !strings.HasSuffix(strings.ToLower(to), "@"+strings.ToLower(domain)) {
		return &gosmtp.SMTPError{
			Code:         550,
			EnhancedCode: gosmtp.EnhancedCode{5, 1, 1},
			Message:      "User not found",
		}
	}
	s.to = append(s.to, strings.ToLower(to))
	return nil
}

func (s *session) Data(r io.Reader) error {
	return s.backend.Deliver(s.from, s.to, r)
}

func (s *session) Reset() {
	s.from = ""
	s.to = nil
}

func (s *session) Logout() error { return nil }

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
