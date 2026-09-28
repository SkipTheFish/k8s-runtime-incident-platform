package incident

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type transitionStub struct {
	outcome TransitionOutcome
	err     error
	command TransitionCommand
}

func (s *transitionStub) ApplyTransition(_ context.Context, _ int64, command TransitionCommand) (TransitionOutcome, error) {
	s.command = command
	return s.outcome, s.err
}

func TestAcknowledgeRequiresVersionAndMapsConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name   string
		header string
		stub   *transitionStub
		want   int
	}{
		{name: "missing", want: http.StatusPreconditionRequired, stub: &transitionStub{}},
		{name: "malformed", header: `"incident-9-v1"`, want: http.StatusBadRequest, stub: &transitionStub{}},
		{name: "stale", header: `"incident-7-v1"`, want: http.StatusPreconditionFailed, stub: &transitionStub{err: &DomainError{Code: CodeVersionConflict, Message: "changed"}}},
		{name: "success", header: `"incident-7-v2"`, want: http.StatusOK, stub: &transitionStub{outcome: TransitionOutcome{IncidentID: 7, Status: IncidentAcknowledged, Version: 3, Changed: true}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/incidents/:id/acknowledge", NewAPIHandler(nil, test.stub).Acknowledge)
			request := httptest.NewRequest(http.MethodPost, "/incidents/7/acknowledge", strings.NewReader(`{"actor":"oncall","reason":"investigating"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("If-Match", test.header)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.want == http.StatusOK && (test.stub.command.ExpectedVersion == nil || *test.stub.command.ExpectedVersion != 2 || response.Header().Get("ETag") != `"incident-7-v3"`) {
				t.Fatalf("command=%+v etag=%s", test.stub.command, response.Header().Get("ETag"))
			}
		})
	}
}
