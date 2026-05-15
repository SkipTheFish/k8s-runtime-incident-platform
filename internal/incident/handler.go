package incident

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(
	s *Service,
) *Handler {

	return &Handler{
		service: s,
	}
}

func (h *Handler) List(
	c *gin.Context,
) {

	tenant := c.GetString("tenant")

	result, err := h.service.List(
		c,
		tenant,
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

func (h *Handler) Get(
	c *gin.Context,
) {

	tenant := c.GetString("tenant")

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

	result, err := h.service.Get(
		c,
		tenant,
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
		result,
	)
}

func (h *Handler) ListOpen(
	c *gin.Context,
) {

	tenant := c.GetString("tenant")

	result, err := h.service.ListOpen(
		c,
		tenant,
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