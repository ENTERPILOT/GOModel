package requestflow

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type yamlDocument struct {
	Version int           `yaml:"version"`
	Plans   []*Definition `yaml:"plans"`
}

func loadYAMLDefinitions(path string) ([]*Definition, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read flow yaml: %w", err)
	}
	var doc yamlDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse flow yaml: %w", err)
	}
	now := time.Now().UTC()
	defs := make([]*Definition, 0, len(doc.Plans))
	for i, def := range doc.Plans {
		if def == nil {
			continue
		}
		copyDef := *def
		if strings.TrimSpace(copyDef.ID) == "" {
			copyDef.ID = fmt.Sprintf("yaml-plan-%d", i+1)
		}
		if copyDef.CreatedAt.IsZero() {
			copyDef.CreatedAt = now
		}
		if copyDef.UpdatedAt.IsZero() {
			copyDef.UpdatedAt = now
		}
		copyDef.Source = "yaml"
		defs = append(defs, &copyDef)
	}
	return defs, nil
}
