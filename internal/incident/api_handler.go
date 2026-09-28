package incident

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"observability-platform/internal/httpapi"
	"observability-platform/internal/middleware"

	"github.com/gin-gonic/gin"
)

type TransitionApplier interface {
	ApplyTransition(context.Context, int64, TransitionCommand) (TransitionOutcome, error)
}

type APIHandler struct {
	queries     *QueryService
	transitions TransitionApplier
}

func NewAPIHandler(queries *QueryService, transitions ...TransitionApplier) *APIHandler {
	handler := &APIHandler{queries: queries}
	if len(transitions) > 0 {
		handler.transitions = transitions[0]
	}
	return handler
}

func (h *APIHandler) List(c *gin.Context) {
	filter, err := ParseListFilter(c.Request.URL.Query())
	if err != nil {
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_FILTER", err.Error(), nil)
		return
	}
	page, err := h.queries.List(c.Request.Context(), c.GetString(middleware.TenantKey), filter)
	if err != nil {
		httpapi.WriteMappedError(c, err)
		return
	}
	c.JSON(http.StatusOK, httpapi.Page{Items: page.Items, NextCursor: page.NextCursor, Page: page.Page, PageSize: page.PageSize, Total: page.Total})
}

func (h *APIHandler) ListOpen(c *gin.Context) {
	values := c.Request.URL.Query()
	values.Set("status", "open")
	c.Request.URL.RawQuery = values.Encode()
	h.List(c)
}

func (h *APIHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_INCIDENT_ID", "incident id must be a positive integer", nil)
		return
	}
	item, err := h.queries.Get(c.Request.Context(), c.GetString(middleware.TenantKey), id)
	if errors.Is(err, ErrIncidentNotFound) {
		httpapi.WriteError(c, http.StatusNotFound, "INCIDENT_NOT_FOUND", "incident was not found", nil)
		return
	}
	if err != nil {
		httpapi.WriteMappedError(c, err)
		return
	}
	c.Header("ETag", IncidentETag(item.ID, item.Version))
	c.JSON(http.StatusOK, item)
}

func (h *APIHandler) Timeline(c *gin.Context)    { h.related(c, true) }
func (h *APIHandler) Correlation(c *gin.Context) { h.related(c, false) }

func (h *APIHandler) Context(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_INCIDENT_ID", "incident id must be a positive integer", nil)
		return
	}
	result, err := h.queries.Context(c.Request.Context(), c.GetString(middleware.TenantKey), id)
	if errors.Is(err, ErrIncidentNotFound) {
		httpapi.WriteError(c, http.StatusNotFound, "INCIDENT_NOT_FOUND", "incident was not found", nil)
		return
	}
	if err != nil {
		httpapi.WriteMappedError(c, err)
		return
	}
	c.Header("ETag", IncidentETag(id, result.Revision))
	c.JSON(http.StatusOK, result)
}

type acknowledgeRequest struct {
	Actor  string `json:"actor" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}

func (h *APIHandler) Acknowledge(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_INCIDENT_ID", "incident id must be a positive integer", nil)
		return
	}
	if h.transitions == nil {
		httpapi.WriteError(c, http.StatusServiceUnavailable, "INCIDENT_TRANSITIONS_UNAVAILABLE", "incident transitions are unavailable", nil)
		return
	}
	version, err := parseIfMatch(c.GetHeader("If-Match"), id)
	if errors.Is(err, errIfMatchRequired) {
		httpapi.WriteError(c, http.StatusPreconditionRequired, "IF_MATCH_REQUIRED", "If-Match is required", nil)
		return
	}
	if err != nil {
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_IF_MATCH", "If-Match does not identify this incident", nil)
		return
	}
	var request acknowledgeRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Actor) == "" || strings.TrimSpace(request.Reason) == "" {
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_ACKNOWLEDGEMENT", "actor and reason are required", nil)
		return
	}
	outcome, err := h.transitions.ApplyTransition(c.Request.Context(), id, TransitionCommand{Target: IncidentAcknowledged, Actor: request.Actor, Reason: request.Reason, OccurredAt: time.Now().UTC(), ExpectedVersion: &version})
	if IsCode(err, CodeVersionConflict) {
		httpapi.WriteError(c, http.StatusPreconditionFailed, string(CodeVersionConflict), "incident version changed", nil)
		return
	}
	if IsCode(err, CodeInvalidTransition) || IsCode(err, CodeInvalidCommand) {
		var domain *DomainError
		errors.As(err, &domain)
		httpapi.WriteError(c, http.StatusConflict, string(domain.Code), domain.Message, nil)
		return
	}
	if err != nil {
		httpapi.WriteMappedError(c, err)
		return
	}
	c.Header("ETag", IncidentETag(id, outcome.Version))
	c.JSON(http.StatusOK, outcome)
}

var errIfMatchRequired = errors.New("If-Match is required")

func parseIfMatch(value string, incidentID int64) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, errIfMatchRequired
	}
	prefix := `"incident-` + strconv.FormatInt(incidentID, 10) + `-v`
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, `"`) {
		return 0, errors.New("invalid ETag")
	}
	version, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(value, prefix), `"`), 10, 64)
	if err != nil || version < 1 {
		return 0, errors.New("invalid ETag")
	}
	return version, nil
}

func (h *APIHandler) related(c *gin.Context, timeline bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_INCIDENT_ID", "incident id must be a positive integer", nil)
		return
	}
	filter, err := ParseListFilter(map[string][]string{"page": c.QueryArray("page"), "pageSize": c.QueryArray("pageSize")})
	if err != nil {
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_PAGINATION", err.Error(), nil)
		return
	}
	var items any
	var total int64
	if timeline {
		items, total, err = h.queries.store.Timeline(c.Request.Context(), c.GetString(middleware.TenantKey), id, filter.Page, filter.PageSize)
	} else {
		items, total, err = h.queries.store.Correlation(c.Request.Context(), c.GetString(middleware.TenantKey), id, filter.Page, filter.PageSize)
	}
	if errors.Is(err, ErrIncidentNotFound) {
		httpapi.WriteError(c, http.StatusNotFound, "INCIDENT_NOT_FOUND", "incident was not found", nil)
		return
	}
	if err != nil {
		httpapi.WriteMappedError(c, err)
		return
	}
	c.JSON(http.StatusOK, httpapi.Page{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total})
}
