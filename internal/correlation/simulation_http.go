package correlation

import (
	"errors"
	"net/http"

	"observability-platform/internal/httpapi"

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
			httpapi.WriteError(c, http.StatusRequestEntityTooLarge, "SIMULATION_BODY_TOO_LARGE", "simulation request body exceeds limit", nil)
			return
		}
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_SIMULATION_REQUEST", "invalid simulation request", nil)
		return
	}
	result, err := h.simulator.Simulate(c.Request.Context(), c.Param("name"), request)
	if err != nil {
		switch {
		case errors.Is(err, ErrSimulationLimit):
			httpapi.WriteError(c, http.StatusRequestEntityTooLarge, "SIMULATION_LIMIT_EXCEEDED", err.Error(), nil)
		case errors.Is(err, ErrSimulationPolicyNotFound):
			httpapi.WriteError(c, http.StatusNotFound, "CORRELATION_POLICY_NOT_FOUND", err.Error(), nil)
		default:
			httpapi.WriteError(c, http.StatusBadRequest, "SIMULATION_FAILED", err.Error(), nil)
		}
		return
	}
	c.JSON(http.StatusOK, result)
}
