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
		// Generate a random secret
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic(fmt.Sprintf("crypto/rand failed: %v", err))
		}
		secret = hex.EncodeToString(b)
	}
	return &AuthService{secret: secret}
}

// GenerateToken creates a new bearer token.
func (a *AuthService) GenerateToken(workspace string) string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	h := sha256.Sum256(append(b, []byte(a.secret)...))
	return "hr_" + hex.EncodeToString(h[:16])
}

// GenerateOTP creates a 6-digit OTP code.
func (a *AuthService) GenerateOTP() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	val := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	code := fmt.Sprintf("%06d", val%1000000)
	return code
}

// GenerateHandle creates a short handle like "hook_k7m2q".
func GenerateHandle(prefix string) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
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
