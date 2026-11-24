// Package delivery sends one-time codes to contacts: by email over SMTP, or to a
// local test file. There is no SMS provider; implement Sender to add one.
package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	ModeDisabled = "disabled"
	// ModeLocal writes messages to a private file. Refused in production by config.
	ModeLocal = "local"
	// ModeSMTP sends real email.
	ModeSMTP = "smtp"
)

type Message struct {
	Channel string `json:"channel"` // email | phone
	To      string `json:"to"`
	// Kind: register | already_registered | reset_password | add_contact
	Kind string `json:"kind"`
	Code string `json:"code,omitempty"`
	// Locale: kk | ru
	Locale string    `json:"locale"`
	At     time.Time `json:"at"`
}

type Sender interface {
	// Available reports whether the channel (email, phone) can be used.
	Available(channel string) bool
	// Send must return quickly and take the same time for every contact.
	Send(ctx context.Context, m Message) error
	// Close delivers what is queued until ctx ends.
	Close(ctx context.Context)
}

// New returns the sender for a validated mode; unknown modes are disabled.
func New(mode, dir string, cfg SMTPConfig, log *slog.Logger) Sender {
	switch mode {
	case ModeLocal:
		return &local{dir: dir}
	case ModeSMTP:
		return NewSMTP(cfg, log)
	}
	return disabled{}
}

type disabled struct{}

func (disabled) Available(string) bool { return false }
func (disabled) Close(context.Context) {}
func (disabled) Send(context.Context, Message) error {
	return fmt.Errorf("contact delivery is disabled")
}

type local struct {
	dir string
	mu  sync.Mutex
}

// OutboxFile is the JSON-lines file written by the local sender.
const OutboxFile = "outbox.jsonl"

func (*local) Available(string) bool { return true }
func (*local) Close(context.Context) {}

func (l *local) Send(_ context.Context, m Message) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(l.dir, OutboxFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	m.At = time.Now().UTC()
	if err := json.NewEncoder(f).Encode(m); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
