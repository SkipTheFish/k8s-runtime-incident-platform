package timeline

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type API struct {
	repo *Repository
}

func NewAPI(repo *Repository) *API {
	return &API{
		repo: repo,
	}
}

func (a *API) Register(r *gin.Engine) {

	group := r.Group("/runtime")

	group.GET(
		"/incidents/:id/timeline",
		a.listTimeline,
	)
}

func (a *API) listTimeline(
	c *gin.Context,
) {

	idStr := c.Param("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {

		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid incident id",
			},
		)

		return
	}

	entries, err := a.repo.ListByIncident(
		c.Request.Context(),
		id,
		200,
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
		entries,
	)
}