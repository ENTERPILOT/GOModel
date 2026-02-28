package auditlog

import (
	"strings"
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
	obj, ok := v.(map[string]interface{})
	if !ok || obj == nil {
		return ""
	}
	raw, ok := obj[key]
	if !ok {
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
