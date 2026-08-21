package event

type RuntimeResource struct {
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type Status struct {
	Subscribers int `json:"subscribers"`

	Metrics map[string]uint64 `json:"metrics"`
}
