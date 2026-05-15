package timeline

type EntryType string

const (
	EntryTypeCause   EntryType = "CAUSE"

	EntryTypeSymptom EntryType = "SYMPTOM"

	EntryTypeContext EntryType = "CONTEXT"

	EntryTypeAction  EntryType = "ACTION"

	EntryTypeSystem  EntryType = "SYSTEM"
)