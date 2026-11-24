package delivery

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	TLSStartTLS = "starttls"
	TLSImplicit = "tls"

	smtpTimeout = 20 * time.Second
	queueSize   = 200
)

// defaultRetries covers temporary failures; codes live 15 minutes.
var defaultRetries = []time.Duration{2 * time.Second, 10 * time.Second, 30 * time.Second}

type SMTPConfig struct {
	Host, Port, TLS    string
	Username, Password string
	From, FromName     string
	ReplyTo            string

	// rootCAs is set only by tests; nil means the system pool.
	rootCAs *x509.CertPool
	// retries overrides defaultRetries in tests.
	retries []time.Duration
}

// Validate reports configuration errors without ever quoting the password.
func (c SMTPConfig) Validate() error {
	var errs []error
	if c.Host == "" || strings.ContainsAny(c.Host, ": /\r\n") {
		errs = append(errs, errors.New("SMTP_HOST must be a host name"))
	}
	if p, err := strconv.Atoi(c.Port); err != nil || p < 1 || p > 65535 {
		errs = append(errs, errors.New("SMTP_PORT must be a port number"))
	}
	if c.TLS != TLSStartTLS && c.TLS != TLSImplicit {
		errs = append(errs, errors.New("SMTP_TLS must be starttls or tls (unencrypted SMTP is not supported)"))
	}
	if c.Username == "" || c.Password == "" {
		errs = append(errs, errors.New("SMTP_USERNAME and SMTP_PASSWORD are required"))
	}
	if !plainAddress(c.From) {
		errs = append(errs, errors.New("SMTP_FROM must be a plain email address"))
	}
	if c.ReplyTo != "" && !plainAddress(c.ReplyTo) {
		errs = append(errs, errors.New("SMTP_REPLY_TO must be a plain email address"))
	}
	if strings.ContainsAny(c.FromName, "\r\n") {
		errs = append(errs, errors.New("SMTP_FROM_NAME must be one line"))
	}
	return errors.Join(errs...)
}

func plainAddress(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s && !strings.ContainsAny(s, "\r\n")
}

// smtpSender queues messages in memory, so the response time of the API does not
// depend on the mail server and does not reveal which contacts exist. Codes are
// deliberately not persisted: after a crash the visitor requests a new one.
type smtpSender struct {
	cfg   SMTPConfig
	log   *slog.Logger
	queue chan Message
	done  chan struct{}

	ctx    context.Context // cancelled when shutdown runs out of time
	cancel context.CancelFunc

	mu     sync.Mutex
	closed bool
}

// NewSMTP starts the background worker. Call Close on shutdown.
func NewSMTP(cfg SMTPConfig, log *slog.Logger) Sender {
	s := &smtpSender{cfg: cfg, log: log, queue: make(chan Message, queueSize), done: make(chan struct{})}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	go s.work()
	return s
}

func (s *smtpSender) Available(channel string) bool { return channel == "email" }

func (s *smtpSender) Send(_ context.Context, m Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("mail queue is closed")
	}
	select {
	case s.queue <- m:
		return nil
	default:
		return errors.New("mail queue is full")
	}
}

// Close stops accepting messages and waits for the queue until ctx ends.
func (s *smtpSender) Close(ctx context.Context) {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.queue)
	}
	s.mu.Unlock()
	select {
	case <-s.done:
	case <-ctx.Done():
		s.log.Warn("mail queue not drained before shutdown", "left", len(s.queue))
	}
	s.cancel()
}

func (s *smtpSender) work() {
	defer close(s.done)
	retries := s.cfg.retries
	if retries == nil {
		retries = defaultRetries
	}
	for m := range s.queue {
		if s.ctx.Err() != nil {
			return // forced shutdown
		}
		for attempt := 0; ; attempt++ {
			err := SendSMTP(s.ctx, s.cfg, m)
			if err == nil {
				break
			}
			retry := attempt < len(retries) && temporary(err) && s.ctx.Err() == nil
			// No recipient and no server text: replies often quote the address.
			s.log.Error("mail delivery failed", "kind", m.Kind, "attempt", attempt+1, "will_retry", retry, "reason", SafeReason(err))
			if !retry {
				break
			}
			select {
			case <-time.After(retries[attempt]):
			case <-s.ctx.Done():
			}
		}
	}
}

func temporary(err error) bool {
	var te *textproto.Error
	if errors.As(err, &te) {
		return te.Code >= 400 && te.Code < 500
	}
	var ve *tls.CertificateVerificationError
	return !errors.As(err, &ve) && !errors.Is(err, errNoStartTLS)
}

var errNoStartTLS = errors.New("server does not offer STARTTLS")

// SafeReason describes a failure without addresses or credentials.
func SafeReason(err error) string {
	var te *textproto.Error
	var ve *tls.CertificateVerificationError
	var ne net.Error
	switch {
	case errors.As(err, &te):
		hint := ""
		if te.Code == 534 || te.Code == 535 {
			hint = " (login rejected: check SMTP_USERNAME and the app password)"
		}
		return fmt.Sprintf("smtp reply %d%s", te.Code, hint)
	case errors.As(err, &ve):
		return "TLS certificate verification failed"
	case errors.Is(err, errNoStartTLS):
		return errNoStartTLS.Error()
	case errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	default:
		return "connection failed"
	}
}

// SendSMTP delivers one message synchronously over a verified TLS connection.
func SendSMTP(ctx context.Context, cfg SMTPConfig, m Message) error {
	body, err := buildMail(cfg, m, time.Now())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, smtpTimeout)
	defer cancel()
	tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12, RootCAs: cfg.rootCAs}

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(cfg.Host, cfg.Port))
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	// A forced shutdown interrupts a stuck conversation.
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()

	if cfg.TLS == TLSImplicit {
		tc := tls.Client(conn, tlsCfg)
		if err := tc.HandshakeContext(ctx); err != nil {
			return err
		}
		conn = tc
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Hello("localhost"); err != nil {
		return err
	}
	if cfg.TLS == TLSStartTLS {
		// Never fall back to plain text, even if an attacker strips the offer.
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errNoStartTLS
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
		return err
	}
	if err := c.Mail(cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(m.To); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	// Accepted: a failing QUIT must not cause a duplicate.
	_ = c.Quit()
	return nil
}

func buildMail(cfg SMTPConfig, m Message, now time.Time) ([]byte, error) {
	if !plainAddress(m.To) {
		return nil, errors.New("invalid recipient")
	}
	subject, text := mailText(m)
	var b bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	header("From", (&mail.Address{Name: cfg.FromName, Address: cfg.From}).String())
	header("To", (&mail.Address{Address: m.To}).String())
	if cfg.ReplyTo != "" {
		header("Reply-To", (&mail.Address{Address: cfg.ReplyTo}).String())
	}
	header("Subject", mime.BEncoding.Encode("utf-8", subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", "<"+rand.Text()+cfg.From[strings.LastIndex(cfg.From, "@"):]+">")
	header("Auto-Submitted", "auto-generated")
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	q := quotedprintable.NewWriter(&b)
	if _, err := q.Write([]byte(strings.ReplaceAll(text, "\n", "\r\n"))); err != nil {
		return nil, err
	}
	return b.Bytes(), q.Close()
}
