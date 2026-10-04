package outbox

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"observability-platform/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct{ store *Store }

func NewAdminHandler(store *Store) *AdminHandler { return &AdminHandler{store: store} }

func (h *AdminHandler) List(c *gin.Context) {
	page, pageSize := 1, 50
	if raw := c.Query("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			httpapi.WriteError(c, http.StatusBadRequest, "INVALID_PAGINATION", "page must be positive", nil)
			return
		}
		page = value
	}
	if raw := c.Query("pageSize"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			httpapi.WriteError(c, http.StatusBadRequest, "INVALID_PAGINATION", "pageSize must be from 1 to 100", nil)
			return
		}
		pageSize = value
	}
	items, total, err := h.store.ListDeadLetters(c.Request.Context(), pageSize, (page-1)*pageSize)
	if err != nil {
		httpapi.WriteMappedError(c, err)
		return
	}
	c.JSON(http.StatusOK, httpapi.Page{Items: items, Page: page, PageSize: pageSize, Total: total})
}

func (h *AdminHandler) Replay(c *gin.Context) {
	var request struct {
		Actor  string `json:"actor"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Actor) == "" || strings.TrimSpace(request.Reason) == "" {
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_REPLAY_REQUEST", "actor and reason are required", nil)
		return
	}
	audit, err := h.store.Replay(c.Request.Context(), c.Param("eventID"), request.Actor, request.Reason)
	if errors.Is(err, ErrEventNotFound) {
		httpapi.WriteError(c, http.StatusNotFound, "OUTBOX_EVENT_NOT_FOUND", "outbox event was not found", nil)
		return
	}
	if errors.Is(err, ErrNotDeadLetter) {
		httpapi.WriteError(c, http.StatusConflict, "OUTBOX_EVENT_NOT_DEAD_LETTER", "outbox event is not replayable", nil)
		return
	}
	if err != nil {
		httpapi.WriteMappedError(c, err)
		return
	}
	c.JSON(http.StatusOK, audit)
}
