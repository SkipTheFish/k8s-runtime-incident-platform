package traces

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *TracesService
}

func NewHandler(s *TracesService) *Handler {
	return &Handler{service: s}
}

func (h *Handler) GetTrace(c *gin.Context) {

	tenant := c.GetString("tenant")
	traceID := c.Param("traceId")

	res, err := h.service.GetTrace(c, tenant, traceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) Search(c *gin.Context) {

	tenant := c.GetString("tenant")
	service := c.Query("service")

	res, err := h.service.Search(c, tenant, service)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}