// Package validate normalises user input shared by several modules.
package validate

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

const (
	Email = "email"
	Phone = "phone"
)

var (
	phoneRe = regexp.MustCompile(`^\+7[0-9]{10}$`)
	binRe   = regexp.MustCompile(`^[0-9]{12}$`)
)

// Contact detects the type (email or phone) and returns the canonical form.
func Contact(raw string) (kind, value string, err error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "@") {
		v := strings.ToLower(raw)
		a, perr := mail.ParseAddress(v)
		at := strings.LastIndex(v, "@")
		if perr != nil || a.Address != v || len(v) > 254 || !isASCII(v) || !strings.Contains(v[at:], ".") {
			return "", "", httpx.Invalid("contact", "must be a valid email or phone number")
		}
		return Email, v, nil
	}
	v, ok := normalizePhone(raw)
	if !ok {
		return "", "", httpx.Invalid("contact", "must be a valid email or phone number")
	}
	return Phone, v, nil
}

// PhoneNumber accepts +7…, 7… and 8… with common separators.
func PhoneNumber(field, raw string) (string, error) {
	v, ok := normalizePhone(raw)
	if !ok {
		return "", httpx.Invalid(field, "must look like +7XXXXXXXXXX")
	}
	return v, nil
}

func normalizePhone(raw string) (string, bool) {
	v := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(raw))
	if len(v) == 11 && (v[0] == '8' || v[0] == '7') {
		v = "+7" + v[1:]
	}
	return v, phoneRe.MatchString(v)
}

// Text trims a single-line value and requires 1..max printable characters.
func Text(field, raw string, max int) (string, error) {
	v := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(v)
	if n < 1 || n > max || strings.ContainsFunc(v, unicode.IsControl) {
		return "", httpx.Invalid(field, "is required and must not be too long")
	}
	return v, nil
}

func BIN(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if !binRe.MatchString(v) {
		return "", httpx.Invalid("bin", "must be 12 digits")
	}
	return v, nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 126 {
			return false
		}
	}
	return true
}
