package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

// OWASP baseline for argon2id.
const (
	argonMemory  = 19 * 1024 // KiB
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
)

// hashSlots bounds concurrent hashing, so login floods cannot exhaust memory.
var hashSlots = make(chan struct{}, 8)

// dummyHash makes a login for an unknown contact cost as much as a real one.
var dummyHash = HashPassword("timing-equaliser")

func HashPassword(password string) string {
	salt := randomBytes(16)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(derive(password, salt, argonTime, argonMemory, argonThreads)))
}

func VerifyPassword(hash, password string) bool {
	p := strings.Split(hash, "$")
	var v int
	var m, t uint32
	var th uint8
	if len(p) != 6 || p[1] != "argon2id" {
		return false
	}
	if _, err := fmt.Sscanf(p[2], "v=%d", &v); err != nil || v != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(p[3], "m=%d,t=%d,p=%d", &m, &t, &th); err != nil {
		return false
	}
	salt, err1 := b64.DecodeString(p[4])
	want, err2 := b64.DecodeString(p[5])
	if err1 != nil || err2 != nil || len(want) != argonKeyLen {
		return false
	}
	return subtle.ConstantTimeCompare(derive(password, salt, t, m, th), want) == 1
}

func derive(password string, salt []byte, t, m uint32, th uint8) []byte {
	hashSlots <- struct{}{}
	defer func() { <-hashSlots }()
	return argon2.IDKey([]byte(password), salt, t, m, th, argonKeyLen)
}

var b64 = base64.RawStdEncoding

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b) // never fails since Go 1.24
	return b
}

var commonPasswords = map[string]bool{
	"1234567890": true, "0123456789": true, "0987654321": true, "1111111111": true, "qwertyuiop": true,
	"password123": true, "password1234": true, "qwerty12345": true, "1q2w3e4r5t": true, "qazwsxedcrfv": true,
	"qwerty123456": true, "12345678910": true, "123456789012": true, "йцукенгшщз": true,
}

// CheckPassword enforces the password policy: 10–128 characters, not trivial.
func CheckPassword(password, contact string) error {
	n := utf8.RuneCountInString(password)
	low := strings.ToLower(password)
	if n < 10 || n > 128 || commonPasswords[low] || (contact != "" && low == strings.ToLower(contact)) ||
		strings.Count(low, string([]rune(low + " ")[0])) == n {
		return httpx.Invalid("password", "must be 10-128 characters and not a common password")
	}
	return nil
}
