// Package config provides shared JSON configuration parsing.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Duration accepts Go duration strings in JSON and integer seconds in flags.
type Duration time.Duration

func (d *Duration) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	*d = Duration(duration)
	return nil
}

func (d *Duration) String() string { return time.Duration(*d).String() }

func (d *Duration) Set(value string) error {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return err
	}
	*d = Duration(time.Duration(seconds) * time.Second)
	return nil
}

// Load reads a JSON object into dst. CONFIG overrides the supplied file path.
// With no path it leaves dst unchanged. Missing fields preserve existing values.
func Load(path string, dst any) error {
	if value, ok := os.LookupEnv("CONFIG"); ok {
		path = value
	}
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %q: %w", path, err)
	}
	if trimmed := bytes.TrimSpace(data); len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("config %q must contain a JSON object", path)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("parse config %q: %w", path, err)
	}
	return nil
}
