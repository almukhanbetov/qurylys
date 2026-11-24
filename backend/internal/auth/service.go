package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/qurylys/qurylys/backend/internal/delivery"
	"github.com/qurylys/qurylys/backend/internal/ratelimit"
	"github.com/qurylys/qurylys/backend/internal/validate"
)

type Service struct {
	repo   *Repository
	sender delivery.Sender
	limit  *ratelimit.Limiter
	log    *slog.Logger
	// emailDailyLimit caps all code requests by email per day; 0 = no cap.
	emailDailyLimit int
}

func NewService(repo *Repository, sender delivery.Sender, limit *ratelimit.Limiter, log *slog.Logger, emailDailyLimit int) *Service {
	return &Service{repo: repo, sender: sender, limit: limit, log: log, emailDailyLimit: emailDailyLimit}
}

func hashToken(token string) []byte {
	s := sha256.Sum256([]byte(token))
	return s[:]
}

func newToken() string { return base64.RawURLEncoding.EncodeToString(randomBytes(32)) }

func hashCode(token, code string) []byte {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte(code))
	return m.Sum(nil)
}

// startChallenge stores a code and returns it together with the client's token.
func (s *Service) startChallenge(ctx context.Context, c challenge) (token, code string, err error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", "", err
	}
	token, code = newToken(), fmt.Sprintf("%06d", n)
	c.CodeHash = hashCode(token, code)
	return token, code, s.repo.createChallenge(ctx, hashToken(token), c)
}

func (s *Service) checkChannel(kind string) error {
	switch {
	case s.sender.Available(kind):
		return nil
	case s.sender.Available(validate.Email) || s.sender.Available(validate.Phone):
		return errChannel
	}
	return errNoDelivery
}

func (s *Service) limitSend(ctx context.Context, kind, contact, ip string) error {
	rules := []ratelimit.Rule{
		{Key: "send:ip:" + ip, Max: 30, Window: time.Hour},
		{Key: "send:contact:" + contact, Max: 5, Window: 15 * time.Minute},
		{Key: "send:day:" + contact, Max: 15, Window: 24 * time.Hour},
	}
	if kind == validate.Email && s.emailDailyLimit > 0 {
		// Last, so that requests rejected above do not use up the shared cap.
		rules = append(rules, ratelimit.Rule{Key: "send:email:all", Max: s.emailDailyLimit, Window: 24 * time.Hour})
	}
	return s.limit.Hit(ctx, rules...)
}

func (s *Service) checkCode(ctx context.Context, purpose, token, code, ip string) (challenge, error) {
	if err := s.limit.Hit(ctx, ratelimit.Rule{Key: "code:ip:" + ip, Max: 60, Window: 15 * time.Minute}); err != nil {
		return challenge{}, err
	}
	c, err := s.repo.attempt(ctx, hashToken(token), purpose)
	if err != nil {
		return c, err
	}
	if !hmac.Equal(c.CodeHash, hashCode(token, strings.TrimSpace(code))) {
		return c, errBadCode
	}
	return c, nil
}

// StartRegistration answers the same way whether or not the contact is taken.
func (s *Service) StartRegistration(ctx context.Context, fullName, contact, password, ip, locale string) (string, error) {
	fullName, err := validate.Text("full_name", fullName, 120)
	if err != nil {
		return "", err
	}
	kind, contact, err := validate.Contact(contact)
	if err != nil {
		return "", err
	}
	if err := CheckPassword(password, contact); err != nil {
		return "", err
	}
	if err := s.checkChannel(kind); err != nil {
		return "", err
	}
	if err := s.limitSend(ctx, kind, contact, ip); err != nil {
		return "", err
	}
	existing, err := s.repo.userByContact(ctx, kind, contact)
	if err != nil {
		return "", err
	}
	hash := HashPassword(password)
	token, code, err := s.startChallenge(ctx, challenge{Purpose: purposeRegister, ContactType: kind, Contact: contact,
		FullName: &fullName, PasswordHash: &hash})
	if err != nil {
		return "", err
	}
	msg := delivery.Message{Channel: kind, To: contact, Kind: delivery.KindRegister, Code: code, Locale: locale}
	if existing != nil {
		msg.Kind, msg.Code = delivery.KindAlreadyRegistered, ""
	}
	if err := s.sender.Send(ctx, msg); err != nil {
		s.log.ErrorContext(ctx, "contact delivery failed", "kind", msg.Kind, "error", err)
		return "", errNoDelivery
	}
	return token, nil
}

func (s *Service) ConfirmRegistration(ctx context.Context, token, code, ip, ua string) (string, error) {
	c, err := s.checkCode(ctx, purposeRegister, token, code, ip)
	if err != nil {
		return "", err
	}
	id, err := s.repo.registerUser(ctx, c)
	if err != nil {
		return "", err
	}
	return s.newSession(ctx, id, ua)
}

func (s *Service) Login(ctx context.Context, contact, password, ip, ua string) (string, error) {
	kind, contact, err := validate.Contact(contact)
	if err != nil {
		return "", err
	}
	perIP, perContact := "login:pair:"+ip+":"+contact, "login:contact:"+contact
	if err := s.limit.Hit(ctx,
		ratelimit.Rule{Key: "login:ip:" + ip, Max: 100, Window: 15 * time.Minute},
		ratelimit.Rule{Key: perIP, Max: 8, Window: 15 * time.Minute},
		ratelimit.Rule{Key: perContact, Max: 40, Window: 15 * time.Minute}); err != nil {
		return "", err
	}
	u, err := s.repo.userByContact(ctx, kind, contact)
	if err != nil {
		return "", err
	}
	hash := dummyHash
	if u != nil {
		hash = u.PasswordHash
	}
	if !VerifyPassword(hash, limitPassword(password)) || u == nil || u.Status != "active" {
		return "", errCredentials
	}
	if err := s.limit.Reset(ctx, perIP, perContact); err != nil {
		return "", err
	}
	return s.newSession(ctx, u.ID, ua)
}

// limitPassword stops oversized input from being hashed; such passwords never exist.
func limitPassword(p string) string {
	if utf8.RuneCountInString(p) > 128 {
		return ""
	}
	return p
}

func (s *Service) newSession(ctx context.Context, userID int64, ua string) (string, error) {
	if r := []rune(strings.ToValidUTF8(strings.ReplaceAll(ua, "\x00", ""), "")); len(r) > 200 {
		ua = string(r[:200])
	} else {
		ua = string(r)
	}
	token := newToken()
	return token, s.repo.createSession(ctx, userID, hashToken(token), ua)
}

// StartReset answers the same way for known and unknown contacts.
func (s *Service) StartReset(ctx context.Context, contact, ip, locale string) (string, error) {
	kind, contact, err := validate.Contact(contact)
	if err != nil {
		return "", err
	}
	if err := s.checkChannel(kind); err != nil {
		return "", err
	}
	if err := s.limitSend(ctx, kind, contact, ip); err != nil {
		return "", err
	}
	u, err := s.repo.userByContact(ctx, kind, contact)
	if err != nil {
		return "", err
	}
	c := challenge{Purpose: purposeReset, ContactType: kind, Contact: contact}
	if u != nil && u.Status == "active" {
		c.UserID = &u.ID
	}
	token, code, err := s.startChallenge(ctx, c)
	if err != nil {
		return "", err
	}
	if c.UserID != nil {
		// A failure here must not reveal that the contact exists.
		if err := s.sender.Send(ctx, delivery.Message{Channel: kind, To: contact, Kind: delivery.KindResetPassword, Code: code, Locale: locale}); err != nil {
			s.log.ErrorContext(ctx, "contact delivery failed", "kind", purposeReset, "error", err)
		}
	}
	return token, nil
}

func (s *Service) Reset(ctx context.Context, token, code, password, ip string) error {
	if err := CheckPassword(password, ""); err != nil {
		return err
	}
	c, err := s.checkCode(ctx, purposeReset, token, code, ip)
	if err != nil {
		return err
	}
	if c.UserID == nil {
		return errBadCode
	}
	if err := CheckPassword(password, c.Contact); err != nil {
		return err
	}
	if err := s.repo.resetPassword(ctx, c, HashPassword(password)); err != nil {
		return err
	}
	return s.limit.Reset(ctx, "login:contact:"+c.Contact, "login:pair:"+ip+":"+c.Contact)
}

// reauth checks the current password before a sensitive change.
func (s *Service) reauth(ctx context.Context, p Principal, password string) error {
	key := fmt.Sprintf("reauth:%d", p.UserID)
	if err := s.limit.Hit(ctx, ratelimit.Rule{Key: key, Max: 8, Window: 15 * time.Minute}); err != nil {
		return err
	}
	hash, err := s.repo.passwordHash(ctx, p.UserID)
	if err != nil {
		return err
	}
	if !VerifyPassword(hash, limitPassword(password)) {
		return errWrongPassword
	}
	return s.limit.Reset(ctx, key)
}

func (s *Service) ChangePassword(ctx context.Context, p Principal, current, next string) error {
	if err := CheckPassword(next, ""); err != nil {
		return err
	}
	if err := s.reauth(ctx, p, current); err != nil {
		return err
	}
	return s.repo.changePassword(ctx, p, HashPassword(next))
}

func (s *Service) StartContact(ctx context.Context, p Principal, contact, password, ip, locale string) (string, error) {
	kind, contact, err := validate.Contact(contact)
	if err != nil {
		return "", err
	}
	if err := s.checkChannel(kind); err != nil {
		return "", err
	}
	if err := s.reauth(ctx, p, password); err != nil {
		return "", err
	}
	if err := s.limitSend(ctx, kind, contact, ip); err != nil {
		return "", err
	}
	token, code, err := s.startChallenge(ctx, challenge{Purpose: purposeAddContact, ContactType: kind, Contact: contact, UserID: &p.UserID})
	if err != nil {
		return "", err
	}
	if err := s.sender.Send(ctx, delivery.Message{Channel: kind, To: contact, Kind: delivery.KindAddContact, Code: code, Locale: locale}); err != nil {
		s.log.ErrorContext(ctx, "contact delivery failed", "kind", purposeAddContact, "error", err)
		return "", errNoDelivery
	}
	return token, nil
}

func (s *Service) ConfirmContact(ctx context.Context, p Principal, token, code, ip string) error {
	c, err := s.checkCode(ctx, purposeAddContact, token, code, ip)
	if err != nil {
		return err
	}
	if c.UserID == nil || *c.UserID != p.UserID {
		return errBadCode
	}
	return s.repo.setContact(ctx, c)
}
