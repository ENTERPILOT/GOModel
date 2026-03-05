package requestflow

import (
	"encoding/json"
	"log/slog"
)

func marshalDefinition(def *Definition) []byte {
	if def == nil {
		return nil
	}
	data, err := json.Marshal(def)
	if err != nil {
		slog.Warn("failed to marshal request flow definition", "error", err, "id", def.ID)
		return []byte("{}")
	}
	return data
}

func marshalExecution(exec *Execution) []byte {
	if exec == nil {
		return nil
	}
	data, err := json.Marshal(exec)
	if err != nil {
		slog.Warn("failed to marshal request flow execution", "error", err, "request_id", exec.RequestID)
		return []byte("{}")
	}
	return data
}
