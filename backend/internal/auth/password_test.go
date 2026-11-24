package auth

import (
	"strings"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	h := HashPassword("дұрыс-құпия-сөз")
	if !strings.HasPrefix(h, "$argon2id$") || h == HashPassword("дұрыс-құпия-сөз") {
		t.Fatal("hash must be argon2id with a random salt")
	}
	if !VerifyPassword(h, "дұрыс-құпия-сөз") || VerifyPassword(h, "wrong") || VerifyPassword("garbage", "x") {
		t.Fatal("verification is wrong")
	}
}

func TestPasswordPolicy(t *testing.T) {
	for _, p := range []string{"", "short", "aaaaaaaaaaaa", "Password123", "user@example.kz", strings.Repeat("ab", 65)} {
		if CheckPassword(p, "user@example.kz") == nil {
			t.Errorf("%q accepted", p)
		}
	}
	if err := CheckPassword("correct horse", "user@example.kz"); err != nil {
		t.Error(err)
	}
}
