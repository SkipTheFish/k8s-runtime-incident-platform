package logs

import (
	"net/http"
	"time"

	"observability-platform/internal/query"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *LogsService
}

func NewHandler(s *LogsService) *Handler {
	return &Handler{service: s}
}

func (h *Handler) Query(c *gin.Context) {
	tenant := c.GetString("tenant")
	expr := c.Query("query")

	q := &query.Query{
		Tenant:    tenant,
		Expr:      expr,
		StartTime: time.Now().Add(-30 * time.Minute),
		EndTime:   time.Now(),
	}

	res, err := h.service.Query(c, q)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) Stats(c *gin.Context) {
	tenant := c.GetString("tenant")
	expr := c.Query("query")

	q := &query.Query{
		Tenant:    tenant,
		Expr:      expr,
		StartTime: time.Now().Add(-30 * time.Minute),
		EndTime:   time.Now(),
	}

	res, err := h.service.Stats(c, q)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}