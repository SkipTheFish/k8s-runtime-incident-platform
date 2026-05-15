package timeline

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"observability-platform/internal/repository"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) ListByIncident(c *gin.Context) {
	idStr := c.Param("id")

	incidentID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid incident id"})
		return
	}

	rows, err := repository.DB.Query(c.Request.Context(), `
		SELECT id, incident_id, event_type, source, namespace, policy_name,
		       resource_kind, resource_name, message, created_at
		FROM runtime_timeline
		WHERE incident_id = $1
		ORDER BY id ASC
	`, incidentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type item struct {
		ID           int64  `json:"id"`
		IncidentID   int64  `json:"incidentID"`
		EventType    string `json:"eventType"`
		Source       string `json:"source"`
		Namespace    string `json:"namespace"`
		PolicyName   string `json:"policyName"`
		ResourceKind string `json:"resourceKind"`
		ResourceName string `json:"resourceName"`
		Message      string `json:"message"`
		CreatedAt    string `json:"createdAt"`
	}

	var result []item

	for rows.Next() {
		var it item
		if err := rows.Scan(
			&it.ID,
			&it.IncidentID,
			&it.EventType,
			&it.Source,
			&it.Namespace,
			&it.PolicyName,
			&it.ResourceKind,
			&it.ResourceName,
			&it.Message,
			&it.CreatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		result = append(result, it)
	}

	c.JSON(http.StatusOK, gin.H{"items": result})
}