package webhook

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"observability-platform/internal/outbox"
)

func TestSignatureContractAndTampering(t *testing.T) {
	now := time.Unix(1790000000, 0).UTC()
	signer := NewSigner("test-secret")
	body := []byte(`{"incidentID":42}`)
	signature := signer.Sign(now, "event-1", body)
	if signature != signer.Sign(now, "event-1", body) {
		t.Fatal("signature is not deterministic")
	}
	if err := signer.Verify(now, 5*time.Minute, "1790000000", "event-1", body, signature); err != nil {
		t.Fatal(err)
	}
	for name, changed := range map[string]struct {
		event     string
		body      []byte
		signature string
	}{
		"body":      {"event-1", []byte(`{"incidentID":43}`), signature},
		"event":     {"event-2", body, signature},
		"signature": {"event-1", body, signature[:len(signature)-1] + "0"},
	} {
		t.Run(name, func(t *testing.T) {
			if !errors.Is(signer.Verify(now, 5*time.Minute, "1790000000", changed.event, changed.body, changed.signature), ErrInvalidSignature) {
				t.Fatal("expected invalid signature")
			}
		})
	}
}

func TestClientSignsExactRequestBody(t *testing.T) {
	now := time.Unix(1790000000, 0).UTC()
	signer := NewSigner("test-secret")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.Header.Get(HeaderEventID) != "event-1" {
			t.Errorf("event header=%q", request.Header.Get(HeaderEventID))
		}
		if err := signer.Verify(now, time.Minute, request.Header.Get(HeaderTimestamp), request.Header.Get(HeaderEventID), body, request.Header.Get(HeaderSignature)); err != nil {
			t.Errorf("verify: %v", err)
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := NewClient(server.URL, "test-secret", server.Client())
	client.now = func() time.Time { return now }
	if err := client.Deliver(context.Background(), outbox.Event{EventID: "event-1", Payload: []byte(`{"ok":true}`)}); err != nil {
		t.Fatal(err)
	}
}

func TestSignatureRejectsExpiredOrFutureTimestamp(t *testing.T) {
	signer := NewSigner("test-secret")
	now := time.Unix(1790000000, 0)
	for _, timestamp := range []time.Time{now.Add(-6 * time.Minute), now.Add(6 * time.Minute)} {
		signature := signer.Sign(timestamp, "event", nil)
		if !errors.Is(signer.Verify(now, 5*time.Minute, strconv.FormatInt(timestamp.Unix(), 10), "event", nil, signature), ErrExpiredSignature) {
			t.Fatal("expected expiration")
		}
	}
}
