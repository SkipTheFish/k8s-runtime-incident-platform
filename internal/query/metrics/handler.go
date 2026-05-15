package metrics

import (
	"net/http"
	"strconv"
	"time"

	"observability-platform/internal/query"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *MetricsService
}

func NewHandler(service *MetricsService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Query(c *gin.Context) {
	tenant := c.GetString("tenant")
	expr := c.Query("query")

	q := &query.Query{
		Tenant: tenant,
		Expr:   expr,
	}

	res, err := h.service.Query(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) Range(c *gin.Context) {
	tenant := c.GetString("tenant")
	expr := c.Query("query")

	now := time.Now()
	start := now.Add(-30 * time.Minute)
	end := now
	step := 30 * time.Second

	if startRaw := c.Query("start"); startRaw != "" {
		if unix, err := strconv.ParseInt(startRaw, 10, 64); err == nil {
			start = time.Unix(unix, 0)
		}
	}

	if endRaw := c.Query("end"); endRaw != "" {
		if unix, err := strconv.ParseInt(endRaw, 10, 64); err == nil {
			end = time.Unix(unix, 0)
		}
	}

	if stepRaw := c.Query("step"); stepRaw != "" {
		if parsed, err := time.ParseDuration(stepRaw); err == nil {
			step = parsed
		}
	}

	q := &query.Query{
		Tenant:    tenant,
		Expr:      expr,
		StartTime: start,
		EndTime:   end,
		Step:      step,
	}

	res, err := h.service.Range(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) GoldenSignals(c *gin.Context) {
	tenant := c.GetString("tenant")
	serviceName := c.Query("service")

	if serviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "missing service",
		})
		return
	}

	res, err := h.service.GoldenSignals(c.Request.Context(), tenant, serviceName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) Labels(c *gin.Context) {
	tenant := c.GetString("tenant")

	res, err := h.service.Labels(c.Request.Context(), tenant)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, res)
}