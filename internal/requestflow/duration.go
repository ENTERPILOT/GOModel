package requestflow

import (
	"encoding/json"
	"fmt"
	"time"
)

// Duration marshals as a human-readable string like "2s".
type Duration time.Duration

func (d Duration) String() string {
	return time.Duration(d).String()
}

// MarshalJSON writes the duration as a string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON accepts either a duration string or a raw integer nanoseconds value.
func (d *Duration) UnmarshalJSON(data []byte) error {
	if d == nil {
		return fmt.Errorf("duration target is nil")
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return err
		}
		*d = Duration(parsed)
		return nil
	}
	var nanos int64
	if err := json.Unmarshal(data, &nanos); err != nil {
		return err
	}
	*d = Duration(time.Duration(nanos))
	return nil
}

// MarshalText writes the duration as a string.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText parses a duration string.
func (d *Duration) UnmarshalText(text []byte) error {
	if d == nil {
		return fmt.Errorf("duration target is nil")
	}
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}
