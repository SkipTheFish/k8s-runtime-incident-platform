package correlation

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const MaxSimulationBodyBytes int64 = 1 << 20

type SimulationHandler struct{ simulator *Simulator }

func NewSimulationHandler(simulator *Simulator) *SimulationHandler {
	return &SimulationHandler{simulator: simulator}
}
func (h *SimulationHandler) Simulate(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxSimulationBodyBytes)
	var request SimulationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "simulation request body exceeds limit"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid simulation request"})
		return
	}
	result, err := h.simulator.Simulate(c.Request.Context(), c.Param("name"), request)
	if err != nil {
		switch {
		case errors.Is(err, ErrSimulationLimit):
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
		case errors.Is(err, ErrSimulationPolicyNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, result)
}
