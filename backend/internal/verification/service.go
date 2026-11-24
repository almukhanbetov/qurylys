package verification

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/delivery"
	"github.com/qurylys/qurylys/backend/internal/httpx"
	"github.com/qurylys/qurylys/backend/internal/ratelimit"
	"github.com/qurylys/qurylys/backend/internal/validate"
)

type Service struct {
	repo   *Repository
	limit  *ratelimit.Limiter
	sender delivery.Sender
	log    *slog.Logger
}

func NewService(repo *Repository, limit *ratelimit.Limiter, sender delivery.Sender, log *slog.Logger) *Service {
	return &Service{repo: repo, limit: limit, sender: sender, log: log}
}

// PrepareUpload runs the cheap checks before the body is read.
func (s *Service) PrepareUpload(ctx context.Context, p auth.Principal, companyID int64, kind string) error {
	if kind != "registration" && kind != "authority" && kind != "other" {
		return httpx.Invalid("kind", "unknown document kind")
	}
	if err := s.limit.Hit(ctx, ratelimit.Rule{Key: fmt.Sprintf("verification:upload:%d", p.UserID), Max: 60, Window: time.Hour}); err != nil {
		return err
	}
	return s.repo.checkOwner(ctx, p, companyID)
}

func (s *Service) Upload(ctx context.Context, p auth.Principal, companyID int64, kind, name string, data []byte) error {
	contentType, err := detectType(data)
	if err != nil {
		return err
	}
	return s.repo.addDocument(ctx, p, companyID, kind, cleanName(name, contentType), contentType, data)
}

func (s *Service) Submit(ctx context.Context, p auth.Principal, companyID int64) error {
	if err := s.limit.Hit(ctx, ratelimit.Rule{Key: fmt.Sprintf("verification:submit:%d", p.UserID), Max: 20, Window: 24 * time.Hour}); err != nil {
		return err
	}
	return s.repo.submit(ctx, p, companyID)
}

var decisions = map[string]struct{ status, action, mail string }{
	"approve":         {StatusApproved, "approved", delivery.KindVerificationApproved},
	"request_changes": {StatusNeedsChanges, "changes_requested", delivery.KindVerificationChanges},
	"reject":          {StatusRejected, "rejected", delivery.KindVerificationRejected},
}

func (s *Service) Decide(ctx context.Context, p auth.Principal, id int64, submissionNo int, decision, comment string) error {
	d, ok := decisions[decision]
	if !ok {
		return httpx.Invalid("decision", "must be approve, request_changes or reject")
	}
	comment = strings.TrimSpace(comment)
	bad := strings.ContainsFunc(comment, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' })
	if bad || utf8.RuneCountInString(comment) > maxComment || (comment == "" && d.status != StatusApproved) {
		return httpx.Invalid("comment", "is required and must not be too long")
	}
	emails, err := s.repo.decide(ctx, p, id, submissionNo, d.status, d.action, comment)
	if err != nil {
		return err
	}
	// The decision is committed; a mail problem must not undo or hide it.
	if s.sender.Available(validate.Email) {
		for _, to := range emails {
			if err := s.sender.Send(ctx, delivery.Message{Channel: validate.Email, To: to, Kind: d.mail, Locale: "kk"}); err != nil {
				s.log.ErrorContext(ctx, "verification notification failed", "kind", d.mail, "error", err)
			}
		}
	}
	return nil
}
