package password

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultCost    = 12
	minPasswordLen = 8
)

var ErrWeakPassword = errors.New("password must be at least 8 characters")

func Hash(plain string) (string, error) {
	if len(plain) < minPasswordLen {
		return "", ErrWeakPassword
	}
	encoded, err := bcrypt.GenerateFromPassword([]byte(plain), defaultCost)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func Verify(plain, encoded string) bool {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return false
	}
	if strings.HasPrefix(encoded, "$2a$") || strings.HasPrefix(encoded, "$2b$") || strings.HasPrefix(encoded, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(encoded), []byte(plain)) == nil
	}
	// Legacy v1 hash is no longer accepted for production usage.
	return false
}

