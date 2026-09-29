package httpapi

const (
	IncidentsPath           = "/api/v1/incidents"
	IncidentPath            = "/api/v1/incidents/:id"
	IncidentTimelinePath    = "/api/v1/incidents/:id/timeline"
	IncidentCorrelationPath = "/api/v1/incidents/:id/correlation"
	IncidentContextPath     = "/api/v1/incidents/:id/context"
	IncidentAcknowledgePath = "/api/v1/incidents/:id/acknowledge"
	PolicySimulationPath    = "/api/v1/correlation-policies/:name/simulate"
)

type Route struct{ Method, Path string }

func RequiredRoutes() []Route {
	return []Route{{"GET", IncidentsPath}, {"GET", IncidentPath}, {"GET", IncidentTimelinePath}, {"GET", IncidentCorrelationPath}, {"GET", IncidentContextPath}, {"POST", IncidentAcknowledgePath}, {"POST", PolicySimulationPath}}
}
