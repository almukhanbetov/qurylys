package delivery

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"mime"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"

	"mime/quotedprintable"
)

// fakeServer is a minimal SMTP server on 127.0.0.1.
type fakeServer struct {
	ln       net.Listener
	cert     tls.Certificate
	pool     *x509.CertPool
	implicit bool
	noTLS    bool
	rcpt     []string // replies to RCPT, one per connection; empty = 250

	mu          sync.Mutex
	mails       []string
	conns       int
	authInClear bool
}

func newServer(t *testing.T, mod func(*fakeServer)) (*fakeServer, SMTPConfig) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"},
		IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	s := &fakeServer{cert: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool: x509.NewCertPool()}
	s.pool.AddCert(leaf)
	if mod != nil {
		mod(s)
	}
	if s.ln, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.ln.Close() })
	go func() {
		for {
			c, err := s.ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	_, port, _ := net.SplitHostPort(s.ln.Addr().String())
	cfg := SMTPConfig{Host: "localhost", Port: port, TLS: TLSStartTLS, Username: "box@gmail.com", Password: "app-secret",
		From: "box@gmail.com", FromName: "QURYLYS B2B", rootCAs: s.pool}
	if s.implicit {
		cfg.TLS = TLSImplicit
	}
	return s, cfg
}

func (s *fakeServer) serve(c net.Conn) {
	defer func() { _ = c.Close() }()
	s.mu.Lock()
	n := s.conns
	s.conns++
	s.mu.Unlock()
	tc := &tls.Config{Certificates: []tls.Certificate{s.cert}}
	secure := s.implicit
	if secure {
		c = tls.Server(c, tc)
	}
	r := bufio.NewReader(c)
	say := func(l string) { fmt.Fprintf(c, "%s\r\n", l) }
	say("220 fake")
	for {
		l, err := r.ReadString('\n')
		if err != nil {
			return
		}
		l = strings.TrimSpace(l)
		switch verb := strings.ToUpper(strings.SplitN(l, " ", 2)[0]); verb {
		case "EHLO":
			say("250-fake")
			if !secure && !s.noTLS {
				say("250-STARTTLS")
			}
			say("250 AUTH PLAIN")
		case "STARTTLS":
			say("220 go")
			c = tls.Server(c, tc)
			r, secure = bufio.NewReader(c), true
		case "AUTH":
			if !secure {
				s.mu.Lock()
				s.authInClear = true
				s.mu.Unlock()
			}
			b, _ := base64.StdEncoding.DecodeString(l[strings.LastIndex(l, " ")+1:])
			if string(b) == "\x00box@gmail.com\x00app-secret" {
				say("235 ok")
			} else {
				say("535 5.7.8 rejected box@gmail.com")
			}
		case "MAIL":
			say("250 ok")
		case "RCPT":
			if n < len(s.rcpt) && s.rcpt[n] != "" {
				say(s.rcpt[n])
			} else {
				say("250 ok")
			}
		case "DATA":
			say("354 go")
			var b strings.Builder
			for {
				d, err := r.ReadString('\n')
				if err != nil || d == ".\r\n" {
					break
				}
				b.WriteString(d)
			}
			s.mu.Lock()
			s.mails = append(s.mails, b.String())
			s.mu.Unlock()
			say("250 ok")
		case "QUIT":
			say("221 bye")
			return
		default:
			say("500 ?")
		}
	}
}

func (s *fakeServer) connections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conns
}

func (s *fakeServer) leaked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authInClear
}

func (s *fakeServer) got() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.mails...)
}

func TestSMTPDelivers(t *testing.T) {
	for _, implicit := range []bool{false, true} {
		s, cfg := newServer(t, func(s *fakeServer) { s.implicit = implicit })
		if err := SendSMTP(context.Background(), cfg, Message{To: "a@example.kz", Kind: KindRegister, Code: "012345", Locale: "ru"}); err != nil {
			t.Fatal(err)
		}
		msg, err := mail.ReadMessage(strings.NewReader(s.got()[0]))
		if err != nil {
			t.Fatal(err)
		}
		subj, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
		body, _ := io.ReadAll(quotedprintable.NewReader(msg.Body))
		if subj != "QURYLYS B2B: подтверждение регистрации" || !strings.Contains(string(body), "012345") ||
			msg.Header.Get("To") != "<a@example.kz>" || !strings.Contains(msg.Header.Get("From"), "<box@gmail.com>") || s.leaked() {
			t.Fatalf("subject %q headers %v body %s", subj, msg.Header, body)
		}
	}
}

func TestSMTPRequiresTrustedTLS(t *testing.T) {
	send := func(cfg SMTPConfig) error {
		return SendSMTP(context.Background(), cfg, Message{To: "a@example.kz", Kind: KindTest})
	}
	s, cfg := newServer(t, func(s *fakeServer) { s.noTLS = true })
	if err := send(cfg); err == nil || temporary(err) || s.leaked() || len(s.got()) != 0 {
		t.Fatalf("must refuse a server without STARTTLS: %v", err)
	}
	s, cfg = newServer(t, nil)
	cfg.rootCAs = nil
	if err := send(cfg); err == nil || temporary(err) || SafeReason(err) != "TLS certificate verification failed" || len(s.got()) != 0 {
		t.Fatalf("must refuse an untrusted certificate: %v", err)
	}
}

func TestSMTPQueueRetriesAndHidesSecrets(t *testing.T) {
	run := func(s *fakeServer, cfg SMTPConfig) string {
		cfg.retries = []time.Duration{time.Millisecond, time.Millisecond}
		var buf bytes.Buffer
		q := NewSMTP(cfg, slog.New(slog.NewTextHandler(&buf, nil)))
		if err := q.Send(context.Background(), Message{To: "victim@example.kz", Kind: KindResetPassword, Code: "987654"}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		q.Close(ctx) // drains the queue
		if q.Send(ctx, Message{}) == nil {
			t.Fatal("closed queue accepted a message")
		}
		for _, secret := range []string{"victim", "987654", "app-secret", "box@"} {
			if strings.Contains(buf.String(), secret) {
				t.Fatalf("log leaks %q: %s", secret, buf.String())
			}
		}
		return buf.String()
	}
	s, cfg := newServer(t, func(s *fakeServer) { s.rcpt = []string{"451 try <victim@example.kz> later"} })
	if run(s, cfg); len(s.got()) != 1 || s.connections() != 2 {
		t.Fatalf("temporary failure must be retried: %d mails, %d connections", len(s.got()), s.connections())
	}
	s, cfg = newServer(t, func(s *fakeServer) { s.rcpt = []string{"550 no <victim@example.kz>"} })
	if run(s, cfg); len(s.got()) != 0 || s.connections() != 1 {
		t.Fatal("permanent failure must not be retried")
	}
	s, cfg = newServer(t, nil)
	cfg.Password = "wrong"
	if log := run(s, cfg); !strings.Contains(log, "535") || s.connections() != 1 {
		t.Fatalf("login failure: %s", log)
	}
}

// A slow mail server must not slow down the API.
func TestSMTPSendDoesNotBlock(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0") // accepts and never answers
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	q := NewSMTP(SMTPConfig{Host: "localhost", Port: port, TLS: TLSStartTLS, From: "a@b.kz"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	start, full := time.Now(), false
	for i := 0; i < queueSize+10; i++ {
		full = q.Send(context.Background(), Message{To: "a@example.kz", Kind: KindTest}) != nil || full
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	q.Close(ctx)
	if time.Since(start) > 2*time.Second || !full {
		t.Fatalf("blocked for %s, queue bounded: %v", time.Since(start), full)
	}
}

func TestTemplates(t *testing.T) {
	for locale, set := range templates {
		for _, kind := range []string{KindRegister, KindAlreadyRegistered, KindResetPassword, KindAddContact, KindTest} {
			code := "123456"
			if kind == KindAlreadyRegistered || kind == KindTest {
				code = ""
			}
			subj, text := mailText(Message{Kind: kind, Locale: locale, Code: code})
			if subj == "" || strings.Contains(text, "%") || !strings.Contains(text, code) {
				t.Errorf("%s/%s: %q %q", locale, kind, subj, text)
			}
		}
		if len(set) != len(templates["kk"]) {
			t.Errorf("%s: incomplete", locale)
		}
	}
	for kind := range verificationTemplates {
		subj, text := mailText(Message{Kind: kind, Locale: "ru"})
		if subj == "" || strings.Contains(text, "%") || strings.Contains(text, "http") {
			t.Errorf("%s: %q %q", kind, subj, text)
		}
	}
	if _, err := buildMail(SMTPConfig{From: "a@b.kz"}, Message{To: "a@b.kz\r\nBcc: x@y.kz", Kind: KindTest}, time.Now()); err == nil {
		t.Error("header injection accepted")
	}
}
