package incident

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrIncidentNotFound = errors.New("incident not found")
	ErrInvalidFilter    = errors.New("invalid incident filter")
)

type ListFilter struct {
	Status   string
	Severity string
	From     *time.Time
	To       *time.Time
	Page     int
	PageSize int
	CursorAt *time.Time
	CursorID int64
}

type IncidentView struct {
	ID               int64          `json:"id"`
	Title            string         `json:"title"`
	Status           string         `json:"status"`
	Severity         string         `json:"severity"`
	Namespace        string         `json:"namespace"`
	Policy           string         `json:"policy"`
	Category         string         `json:"category"`
	Fingerprint      string         `json:"fingerprint"`
	RootResource     map[string]any `json:"rootResource"`
	StartedAt        time.Time      `json:"startedAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	ResolvedAt       *time.Time     `json:"resolvedAt,omitempty"`
	AcknowledgedAt   *time.Time     `json:"acknowledgedAt,omitempty"`
	Version          int64          `json:"version"`
	PolicyGeneration int64          `json:"policyGeneration"`
}

type IncidentPage struct {
	Items      []IncidentView
	NextCursor string
	Page       int
	PageSize   int
	Total      int64
}

type QueryStore interface {
	List(context.Context, string, ListFilter) ([]IncidentView, int64, error)
	Get(context.Context, string, int64) (*IncidentView, error)
	Timeline(context.Context, string, int64, int, int) ([]TimelineView, int64, error)
	Correlation(context.Context, string, int64, int, int) ([]CorrelationView, int64, error)
}

type TimelineView struct {
	ID         int64          `json:"id"`
	Type       string         `json:"type"`
	Actor      string         `json:"actor"`
	Message    string         `json:"message"`
	Evidence   map[string]any `json:"evidence"`
	Truncated  bool           `json:"truncated"`
	OccurredAt time.Time      `json:"occurredAt"`
}

type CorrelationView struct {
	ID                int64          `json:"id"`
	SignalID          string         `json:"signalID"`
	RuleName          string         `json:"ruleName"`
	RuleVersion       string         `json:"ruleVersion"`
	PolicyGeneration  int64          `json:"policyGeneration"`
	RootResource      map[string]any `json:"rootResource"`
	Fingerprint       string         `json:"fingerprint"`
	MatchedConditions []string       `json:"matchedConditions"`
	Reason            string         `json:"reason"`
	DecidedAt         time.Time      `json:"decidedAt"`
}

type QueryService struct{ store QueryStore }

func NewQueryService(store QueryStore) *QueryService { return &QueryService{store: store} }

func (s *QueryService) List(ctx context.Context, tenant string, filter ListFilter) (IncidentPage, error) {
	items, total, err := s.store.List(ctx, tenant, filter)
	if err != nil {
		return IncidentPage{}, err
	}
	next := ""
	if len(items) == filter.PageSize {
		last := items[len(items)-1]
		next = encodeCursor(last.UpdatedAt, last.ID)
	}
	return IncidentPage{Items: items, NextCursor: next, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

func (s *QueryService) Get(ctx context.Context, tenant string, id int64) (*IncidentView, error) {
	return s.store.Get(ctx, tenant, id)
}

func ParseListFilter(values map[string][]string) (ListFilter, error) {
	filter := ListFilter{Page: 1, PageSize: 50}
	var err error
	if raw := first(values, "page"); raw != "" {
		filter.Page, err = strconv.Atoi(raw)
		if err != nil || filter.Page < 1 || filter.Page > 100000 {
			return ListFilter{}, fmt.Errorf("%w: page must be between 1 and 100000", ErrInvalidFilter)
		}
	}
	if raw := first(values, "pageSize"); raw != "" {
		filter.PageSize, err = strconv.Atoi(raw)
		if err != nil || filter.PageSize < 1 || filter.PageSize > 200 {
			return ListFilter{}, fmt.Errorf("%w: pageSize must be between 1 and 200", ErrInvalidFilter)
		}
	}
	filter.Status = strings.ToLower(first(values, "status"))
	if filter.Status != "" && filter.Status != "open" && filter.Status != "acknowledged" && filter.Status != "resolved" && filter.Status != "reopened" {
		return ListFilter{}, fmt.Errorf("%w: unsupported status", ErrInvalidFilter)
	}
	filter.Severity = strings.ToLower(first(values, "severity"))
	if filter.Severity != "" && filter.Severity != "info" && filter.Severity != "warning" && filter.Severity != "critical" {
		return ListFilter{}, fmt.Errorf("%w: unsupported severity", ErrInvalidFilter)
	}
	if filter.From, err = parseOptionalTime(first(values, "from")); err != nil {
		return ListFilter{}, fmt.Errorf("%w: invalid from", ErrInvalidFilter)
	}
	if filter.To, err = parseOptionalTime(first(values, "to")); err != nil {
		return ListFilter{}, fmt.Errorf("%w: invalid to", ErrInvalidFilter)
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return ListFilter{}, fmt.Errorf("%w: from must not be after to", ErrInvalidFilter)
	}
	if raw := first(values, "cursor"); raw != "" {
		if filter.Page != 1 {
			return ListFilter{}, fmt.Errorf("%w: cursor and page cannot be combined", ErrInvalidFilter)
		}
		filter.CursorAt, filter.CursorID, err = decodeCursor(raw)
		if err != nil {
			return ListFilter{}, fmt.Errorf("%w: invalid cursor", ErrInvalidFilter)
		}
	}
	return filter, nil
}

func first(values map[string][]string, key string) string {
	if len(values[key]) == 0 {
		return ""
	}
	return strings.TrimSpace(values[key][0])
}

func parseOptionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	return &parsed, err
}

func encodeCursor(at time.Time, id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + ":" + strconv.FormatInt(id, 10)))
}

func decodeCursor(value string) (*time.Time, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, 0, err
	}
	separator := strings.LastIndexByte(string(raw), ':')
	if separator < 0 {
		return nil, 0, errors.New("missing separator")
	}
	at, err := time.Parse(time.RFC3339Nano, string(raw[:separator]))
	if err != nil {
		return nil, 0, err
	}
	id, err := strconv.ParseInt(string(raw[separator+1:]), 10, 64)
	return &at, id, err
}

func IncidentETag(id, version int64) string {
	return fmt.Sprintf("\"incident-%d-v%d\"", id, version)
}
