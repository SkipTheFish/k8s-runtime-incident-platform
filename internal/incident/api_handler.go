package incident

import (
	"errors"
	"net/http"
	"strconv"

	"observability-platform/internal/httpapi"
	"observability-platform/internal/middleware"

	"github.com/gin-gonic/gin"
)

type APIHandler struct{ queries *QueryService }

func NewAPIHandler(queries *QueryService) *APIHandler { return &APIHandler{queries: queries} }

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
