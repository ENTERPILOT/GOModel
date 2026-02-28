package auditlog

import (
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func extractResponseID(entry *LogEntry) string {
	if entry == nil || entry.Data == nil {
		return ""
	}
	return extractStringField(entry.Data.ResponseBody, "id")
}

func extractPreviousResponseID(entry *LogEntry) string {
	if entry == nil || entry.Data == nil {
		return ""
	}
	return extractStringField(entry.Data.RequestBody, "previous_response_id")
}

func extractStringField(v interface{}, key string) string {
	switch obj := v.(type) {
	case map[string]interface{}:
		return extractTrimmedString(obj[key])
	case bson.M:
		return extractTrimmedString(obj[key])
	case bson.D:
		for _, entry := range obj {
			if entry.Key == key {
				return extractTrimmedString(entry.Value)
			}
		}
		return ""
	default:
		return ""
	}
}

func extractTrimmedString(raw interface{}) string {
	if raw == nil {
		return ""
	}
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func clampConversationLimit(limit int) int {
	if limit <= 0 {
		return 40
	}
	if limit > 200 {
		return 200
	}
	return limit
}
