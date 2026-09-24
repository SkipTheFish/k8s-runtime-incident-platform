package incident

import (
	"context"
	"net/url"
	"testing"
	"time"
)

func TestParseListFilterBoundaries(t *testing.T) {
	from := "2026-09-01T00:00:00Z"
	filter, err := ParseListFilter(url.Values{"page": {"2"}, "pageSize": {"200"}, "status": {"OPEN"}, "severity": {"critical"}, "from": {from}})
	if err != nil || filter.Page != 2 || filter.PageSize != 200 || filter.Status != "open" || filter.From == nil {
		t.Fatalf("filter=%+v err=%v", filter, err)
	}
	for _, values := range []url.Values{
		{"page": {"0"}}, {"pageSize": {"201"}}, {"status": {"closed"}}, {"severity": {"urgent"}},
		{"from": {"2026-09-02T00:00:00Z"}, "to": {"2026-09-01T00:00:00Z"}}, {"cursor": {"invalid"}},
	} {
		if _, err := ParseListFilter(values); err == nil {
			t.Fatalf("expected invalid filter for %v", values)
		}
	}
}

func TestPGQueryStorePaginationHasNoDuplicatesOrOmissions(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	base := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		_, err := pool.Exec(context.Background(), `INSERT INTO incidents(tenant_id,title,status,namespace,policy_name,severity,started_at,updated_at,last_event_at,fingerprint,category,root_resource,version,correlation_policy,policy_generation,reopen_window_seconds) VALUES('default',$1,'open','apps','rule','critical',$2,$2,$2,$3,'test','{}',1,'rule',1,1800)`, "incident", base.Add(time.Duration(i)*time.Minute), "page-fingerprint-"+itoa(i))
		if err != nil {
			t.Fatal(err)
		}
	}
	service := NewQueryService(NewPGQueryStoreWithPool(pool))
	first, err := service.List(context.Background(), "default", ListFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	cursorAt, cursorID, err := decodeCursor(first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.List(context.Background(), "default", ListFilter{Page: 1, PageSize: 2, CursorAt: cursorAt, CursorID: cursorID})
	if err != nil {
		t.Fatal(err)
	}
	thirdCursorAt, thirdCursorID, err := decodeCursor(second.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	third, err := service.List(context.Background(), "default", ListFilter{Page: 1, PageSize: 2, CursorAt: thirdCursorAt, CursorID: thirdCursorID})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for _, page := range [][]IncidentView{first.Items, second.Items, third.Items} {
		for _, item := range page {
			if seen[item.ID] {
				t.Fatalf("duplicate id %d", item.ID)
			}
			seen[item.ID] = true
		}
	}
	if len(seen) != 5 || first.Total != 5 || second.Total != 5 || third.Total != 5 {
		t.Fatalf("seen=%d totals=%d/%d/%d", len(seen), first.Total, second.Total, third.Total)
	}
	item, err := service.Get(context.Background(), "default", first.Items[0].ID)
	if err != nil || IncidentETag(item.ID, item.Version) == "" {
		t.Fatalf("item=%+v err=%v", item, err)
	}
}
