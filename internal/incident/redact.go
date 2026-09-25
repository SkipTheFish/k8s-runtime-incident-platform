package incident

import (
	"encoding/json"
	"strings"
)

const maxEvidenceStringBytes = 4096

func SanitizeJSON(raw []byte) (map[string]any, bool) {
	value := map[string]any{}
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return map[string]any{}, false
	}
	truncated := sanitizeMap(value)
	if truncated {
		value["_truncated"] = true
	}
	return value, truncated
}

func sanitizeMap(value map[string]any) bool {
	truncated := false
	for key, item := range value {
		if sensitiveKey(key) {
			value[key] = "[REDACTED]"
			continue
		}
		clean, cut := sanitizeValue(item)
		value[key] = clean
		truncated = truncated || cut
	}
	return truncated
}

func sanitizeValue(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, sanitizeMap(typed)
	case []any:
		truncated := false
		for index, item := range typed {
			typed[index], truncated = sanitizeValue(item)
			if truncated {
				break
			}
		}
		return typed, truncated
	case string:
		if len(typed) > maxEvidenceStringBytes {
			return typed[:maxEvidenceStringBytes] + "...[TRUNCATED]", true
		}
	}
	return value, false
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, fragment := range []string{"secret", "token", "password", "passwd", "authorization", "api_key", "apikey"} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}
