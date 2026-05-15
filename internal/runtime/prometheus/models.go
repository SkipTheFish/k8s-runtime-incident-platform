package prometheus

type AlertsResponse struct {
	Status string `json:"status"`

	Data struct {
		Alerts []Alert `json:"alerts"`
	} `json:"data"`
}

type Alert struct {
	Labels map[string]string `json:"labels"`

	Annotations map[string]string `json:"annotations"`

	State string `json:"state"`

	ActiveAt string `json:"activeAt"`

	Value string `json:"value"`
}