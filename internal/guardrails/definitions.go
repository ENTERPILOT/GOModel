package guardrails

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gomodel/internal/core"
	"gomodel/internal/responsecache"
)

// Definition is one persisted reusable guardrail instance.
type Definition struct {
	Name        string          `json:"name" bson:"name"`
	Type        string          `json:"type" bson:"type"`
	Description string          `json:"description,omitempty" bson:"description,omitempty"`
	UserPath    string          `json:"user_path,omitempty" bson:"user_path,omitempty"`
	Config      json.RawMessage `json:"config" bson:"config"`
	CreatedAt   time.Time       `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" bson:"updated_at"`
}

// View is the admin-facing representation of a persisted guardrail.
type View struct {
	Definition
	Summary string `json:"summary,omitempty"`
}

// ViewFromDefinition projects one guardrail definition into its admin-facing view.
func ViewFromDefinition(def Definition) View {
	return View{
		Definition: cloneDefinition(def),
		Summary:    summarizeDefinition(def),
	}
}

// TypeOption is one allowed option for a typed guardrail config field.
type TypeOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// TypeField describes one UI field for a guardrail type.
type TypeField struct {
	Key         string       `json:"key"`
	Label       string       `json:"label"`
	Input       string       `json:"input"`
	Required    bool         `json:"required"`
	Help        string       `json:"help,omitempty"`
	Placeholder string       `json:"placeholder,omitempty"`
	Options     []TypeOption `json:"options,omitempty"`
}

// TypeDefinition describes one supported guardrail type and its config schema.
type TypeDefinition struct {
	Type        string          `json:"type"`
	Label       string          `json:"label"`
	Description string          `json:"description,omitempty"`
	Defaults    json.RawMessage `json:"defaults"`
	Fields      []TypeField     `json:"fields"`
}

type systemPromptDefinitionConfig struct {
	Mode    string `json:"mode"`
	Content string `json:"content"`
}

func normalizeDefinition(def Definition) (Definition, error) {
	def.Name = strings.TrimSpace(def.Name)
	def.Type = strings.TrimSpace(def.Type)
	def.Description = strings.TrimSpace(def.Description)
	userPath, err := core.NormalizeUserPath(def.UserPath)
	if err != nil {
		return Definition{}, newValidationError("invalid user_path", err)
	}
	def.UserPath = userPath

	if def.Name == "" {
		return Definition{}, newValidationError("guardrail name is required", nil)
	}
	if def.Type == "" {
		return Definition{}, newValidationError("guardrail type is required", nil)
	}

	switch def.Type {
	case "system_prompt":
		cfg, err := decodeSystemPromptDefinitionConfig(def.Config)
		if err != nil {
			return Definition{}, err
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			return Definition{}, newValidationError("marshal guardrail config", err)
		}
		def.Config = raw
	default:
		return Definition{}, newValidationError(`unknown guardrail type: "`+def.Type+`"`, nil)
	}

	return def, nil
}

func cloneDefinition(def Definition) Definition {
	cloned := def
	if len(def.Config) > 0 {
		cloned.Config = append(json.RawMessage(nil), def.Config...)
	}
	return cloned
}

func cloneTypeDefinitions(defs []TypeDefinition) []TypeDefinition {
	if len(defs) == 0 {
		return []TypeDefinition{}
	}
	cloned := make([]TypeDefinition, 0, len(defs))
	for _, def := range defs {
		copyDef := def
		if len(def.Defaults) > 0 {
			copyDef.Defaults = append(json.RawMessage(nil), def.Defaults...)
		}
		if len(def.Fields) > 0 {
			copyDef.Fields = append([]TypeField(nil), def.Fields...)
			for i := range copyDef.Fields {
				if len(copyDef.Fields[i].Options) > 0 {
					copyDef.Fields[i].Options = append([]TypeOption(nil), copyDef.Fields[i].Options...)
				}
			}
		}
		cloned = append(cloned, copyDef)
	}
	return cloned
}

func decodeSystemPromptDefinitionConfig(raw json.RawMessage) (systemPromptDefinitionConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte(`{}`)
	}

	var cfg systemPromptDefinitionConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return systemPromptDefinitionConfig{}, newValidationError("invalid system_prompt config: "+err.Error(), err)
	}
	if decoder.More() {
		return systemPromptDefinitionConfig{}, newValidationError("invalid system_prompt config: trailing data", nil)
	}

	cfg.Mode = effectiveSystemPromptMode(cfg.Mode)
	if !isValidSystemPromptMode(cfg.Mode) {
		return systemPromptDefinitionConfig{}, newValidationError("system_prompt mode is invalid", nil)
	}
	cfg.Content = strings.TrimSpace(cfg.Content)
	if cfg.Content == "" {
		return systemPromptDefinitionConfig{}, newValidationError("system_prompt content is required", nil)
	}
	return cfg, nil
}

func buildDefinition(def Definition) (Guardrail, responsecache.GuardrailRuleDescriptor, error) {
	switch def.Type {
	case "system_prompt":
		cfg, err := decodeSystemPromptDefinitionConfig(def.Config)
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, err
		}
		mode := SystemPromptMode(cfg.Mode)
		instance, err := NewSystemPromptGuardrail(def.Name, mode, cfg.Content)
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, newValidationError("build system_prompt guardrail: "+err.Error(), err)
		}
		return instance, responsecache.GuardrailRuleDescriptor{
			Name:    def.Name,
			Type:    def.Type,
			Mode:    string(mode),
			Content: cfg.Content,
		}, nil
	default:
		return nil, responsecache.GuardrailRuleDescriptor{}, newValidationError(`unknown guardrail type: "`+def.Type+`"`, nil)
	}
}

func summarizeDefinition(def Definition) string {
	switch def.Type {
	case "system_prompt":
		cfg, err := decodeSystemPromptDefinitionConfig(def.Config)
		if err != nil {
			return ""
		}
		content := strings.Join(strings.Fields(cfg.Content), " ")
		const maxLen = 72
		if len(content) > maxLen {
			content = content[:maxLen-3] + "..."
		}
		if content == "" {
			return cfg.Mode
		}
		return fmt.Sprintf("%s • %s", cfg.Mode, content)
	default:
		return ""
	}
}

// TypeDefinitions returns the UI-facing definitions for supported guardrail types.
func TypeDefinitions() []TypeDefinition {
	return cloneTypeDefinitions([]TypeDefinition{
		{
			Type:        "system_prompt",
			Label:       "System Prompt",
			Description: "Injects, overrides, or decorates the system message before the request reaches the provider.",
			Defaults:    mustMarshalRaw(systemPromptDefinitionConfig{Mode: string(SystemPromptInject), Content: ""}),
			Fields: []TypeField{
				{
					Key:      "mode",
					Label:    "Mode",
					Input:    "select",
					Required: true,
					Help:     "Choose whether the prompt is injected only when absent, overrides existing system prompts, or decorates the first one.",
					Options: []TypeOption{
						{Value: string(SystemPromptInject), Label: "Inject"},
						{Value: string(SystemPromptOverride), Label: "Override"},
						{Value: string(SystemPromptDecorator), Label: "Decorator"},
					},
				},
				{
					Key:         "content",
					Label:       "Content",
					Input:       "textarea",
					Required:    true,
					Help:        "The system prompt text applied by this guardrail.",
					Placeholder: "You are a precise assistant. Follow the compliance policy...",
				},
			},
		},
	})
}

func mustMarshalRaw(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
