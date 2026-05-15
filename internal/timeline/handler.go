package timeline

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(
	s Service,
) *Handler {

	return &Handler{
		service: s,
	}
}

func (h *Handler) GetByIncident(
	c *gin.Context,
) {

	idStr := c.Param("id")

	id, err := strconv.ParseInt(
		idStr,
		10,
		64,
	)

	if err != nil {

		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid incident id",
			},
		)

		return
	}

	result, err := h.service.GetByIncidentID(
		c,
		id,
	)

	if err != nil {

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)

		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"items": result,
		},
	)
}