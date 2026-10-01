package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderTimestamp = "X-KRIP-Timestamp"
	HeaderEventID   = "X-KRIP-Event-ID"
	HeaderSignature = "X-KRIP-Signature"
)

var (
	ErrInvalidSignature = errors.New("invalid webhook signature")
	ErrExpiredSignature = errors.New("expired webhook signature")
)

type Signer struct{ secret []byte }

func NewSigner(secret string) *Signer { return &Signer{secret: []byte(secret)} }

func (s *Signer) Sign(timestamp time.Time, eventID string, body []byte) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(signatureInput(timestamp.Unix(), eventID, body))
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

func (s *Signer) Verify(now time.Time, maxAge time.Duration, timestampValue, eventID string, body []byte, signature string) error {
	unix, err := strconv.ParseInt(timestampValue, 10, 64)
	if err != nil {
		return ErrInvalidSignature
	}
	timestamp := time.Unix(unix, 0)
	if timestamp.After(now.Add(maxAge)) || timestamp.Before(now.Add(-maxAge)) {
		return ErrExpiredSignature
	}
	expected := s.Sign(timestamp, eventID, body)
	if !hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature))) {
		return ErrInvalidSignature
	}
	return nil
}

func signatureInput(unix int64, eventID string, body []byte) []byte {
	prefix := strconv.FormatInt(unix, 10) + "\n" + eventID + "\n"
	result := make([]byte, 0, len(prefix)+len(body))
	result = append(result, prefix...)
	return append(result, body...)
}
