package guardrails

import (
	"encoding/json"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoConfigFromRaw_NormalizesEmptyAndNullToEmptyDocument(t *testing.T) {
	t.Parallel()

	for _, raw := range []json.RawMessage{
		nil,
		json.RawMessage(""),
		json.RawMessage("   "),
		json.RawMessage("null"),
	} {
		doc, err := mongoConfigFromRaw(raw)
		if err != nil {
			t.Fatalf("mongoConfigFromRaw(%q) error = %v", raw, err)
		}
		if doc == nil {
			t.Fatalf("mongoConfigFromRaw(%q) = nil, want empty document", raw)
		}
		if len(doc) != 0 {
			t.Fatalf("mongoConfigFromRaw(%q) = %#v, want empty document", raw, doc)
		}
	}
}

func TestDefinitionFromMongo_NormalizesNilConfigToEmptyObject(t *testing.T) {
	t.Parallel()

	definition, err := definitionFromMongo(mongoDefinitionDocument{
		Name:   "policy",
		Type:   "system_prompt",
		Config: nil,
	})
	if err != nil {
		t.Fatalf("definitionFromMongo() error = %v", err)
	}
	if got := string(definition.Config); got != "{}" {
		t.Fatalf("definition.Config = %q, want {}", got)
	}

	definition, err = definitionFromMongo(mongoDefinitionDocument{
		Name:   "policy",
		Type:   "system_prompt",
		Config: bson.M{},
	})
	if err != nil {
		t.Fatalf("definitionFromMongo() with empty map error = %v", err)
	}
	if got := string(definition.Config); got != "{}" {
		t.Fatalf("definition.Config from empty map = %q, want {}", got)
	}
}
