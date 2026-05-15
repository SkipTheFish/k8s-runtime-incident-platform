package operator

type AlertPolicySpec struct {
	Name      string  `json:"name"`
	Query     string  `json:"query"`
	Threshold float64 `json:"threshold"`
	Operator  string  `json:"operator"`
}

type AlertPolicyCR struct {
	Spec AlertPolicySpec `json:"spec"`
}