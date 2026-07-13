package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// AuthService handles token generation and verification.
type AuthService struct {
	secret string
}

// New creates a new AuthService.
func New(secret string) *AuthService {
	if secret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		secret = hex.EncodeToString(b)
	}
	return &AuthService{secret: secret}
}

// GenerateToken creates a new bearer token.
func (a *AuthService) GenerateToken(workspace string) string {
	b := make([]byte, 24)
	rand.Read(b)
	h := sha256.Sum256(append(b, []byte(a.secret)...))
	return "hr_" + hex.EncodeToString(h[:16])
}

// GenerateOTP creates a 6-digit OTP code.
func (a *AuthService) GenerateOTP() string {
	b := make([]byte, 4)
	rand.Read(b)
	code := fmt.Sprintf("%06d", int(b[0])%1000000)
	return code
}

// GenerateHandle creates a short handle like "hook_k7m2q".
func GenerateHandle(prefix string) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 5)
	rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return prefix + "_" + string(b)
}

// TokenExpiry returns the default token expiry (1 year).
func TokenExpiry() time.Time {
	return time.Now().AddDate(1, 0, 0)
}

// OTPExpiry returns the default OTP expiry (10 minutes).
func OTPExpiry() time.Time {
	return time.Now().Add(10 * time.Minute)
}
