package prometheus

type QueryResponse struct {
	Status string `json:"status"`
	Data   any    `json:"data"`
}