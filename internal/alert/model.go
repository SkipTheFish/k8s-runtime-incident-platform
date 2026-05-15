package alert

import "time"

type AlertStatus string

const (
	StatusFiring   AlertStatus = "firing"
	StatusResolved AlertStatus = "resolved"
)

type AlertPolicy struct {
	ID        int64
	Tenant    string
	Name      string
	Query     string
	Threshold float64
	Operator  string // > < >= <=
	Duration  string
	CreatedAt time.Time
}

type AlertInstance struct {
	ID        int64
	PolicyID  int64
	Status    AlertStatus
	Value     float64
	StartTime time.Time
	EndTime   *time.Time
}