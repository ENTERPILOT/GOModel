package usage

import "strings"

// buildWhereClause joins condition strings into a SQL WHERE clause.
// Returns an empty string when conditions is empty.
func buildWhereClause(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}

// clampLimitOffset normalises pagination parameters:
//   - limit defaults to 50 and is capped at 200
//   - offset floors at 0
func clampLimitOffset(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
