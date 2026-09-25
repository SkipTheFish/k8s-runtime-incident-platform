package incident

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSanitizeJSONRedactsAndMarksTruncation(t *testing.T) {
	value, truncated := SanitizeJSON([]byte(`{"token":"visible","nested":{"Authorization":"Bearer abc","message":"` + strings.Repeat("x", maxEvidenceStringBytes+10) + `"}}`))
	if !truncated || value["token"] != "[REDACTED]" || value["_truncated"] != true {
		t.Fatalf("value=%+v truncated=%v", value, truncated)
	}
	nested := value["nested"].(map[string]any)
	if nested["Authorization"] != "[REDACTED]" || !strings.Contains(nested["message"].(string), "[TRUNCATED]") {
		t.Fatalf("nested=%+v", nested)
	}
}

func TestTimelineAndCorrelationAreBoundedAndSanitized(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	service := NewTransactionService(pool, 3*time.Second)
	created, err := service.Process(context.Background(), incidentSignal("evidence", time.Now().UTC()), incidentMatch())
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), `UPDATE timeline_entries SET evidence=$2 WHERE incident_id=$1`, created.IncidentID, `{"password":"hidden","message":"`+strings.Repeat("z", maxEvidenceStringBytes+1)+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	store := NewPGQueryStoreWithPool(pool)
	timeline, total, err := store.Timeline(context.Background(), "default", created.IncidentID, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(timeline) != 1 || !timeline[0].Truncated || timeline[0].Evidence["password"] != "[REDACTED]" {
		t.Fatalf("timeline=%+v total=%d", timeline, total)
	}
	decisions, total, err := store.Correlation(context.Background(), "default", created.IncidentID, 1, 1)
	if err != nil || total != 1 || len(decisions) != 1 || decisions[0].RuleVersion != "v1" || decisions[0].PolicyGeneration != 3 || len(decisions[0].MatchedConditions) != 1 {
		t.Fatalf("decisions=%+v total=%d err=%v", decisions, total, err)
	}
}
