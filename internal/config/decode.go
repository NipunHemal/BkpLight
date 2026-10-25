package config

import (
	"bytes"
	"io"

	"bkplight.local/bkplight/internal/safeerr"
	"go.yaml.in/yaml/v3"
)

const maxYAMLBytes = 1 << 20

func invalid(field string) error { return &safeerr.Error{Code: safeerr.Invalid, Field: field} }

// Decode performs bounded syntax and semantic checks without opening any
// configured resources. Filesystem permissions and real identities are startup checks.
func Decode(r io.Reader) (Config, error) {
	var c Config
	raw, err := io.ReadAll(io.LimitReader(r, maxYAMLBytes+1))
	if err != nil || len(raw) > maxYAMLBytes {
		return c, invalid("config")
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var root, extra yaml.Node
	if dec.Decode(&root) != nil {
		return c, invalid("config")
	}
	if dec.Decode(&extra) != io.EOF {
		return c, invalid("config")
	}
	count := 0
	if err = inspectNode(&root, 0, &count); err != nil {
		return c, err
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return c, invalid("config")
	}
	dec = yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if dec.Decode(&c) != nil {
		return Config{}, invalid("config")
	}
	// Defaults start at zero for scalar fields; explicit zero is rejected below
	// where zero is not a valid policy value and an omitted field has a default.
	if err = applyDefaults(&c, root.Content[0]); err != nil {
		return Config{}, err
	}
	if err = validate(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func inspectNode(n *yaml.Node, depth int, count *int) error {
	(*count)++
	if depth > 32 || *count > 20000 || n.Kind == yaml.AliasNode || n.Anchor != "" {
		return invalid("config")
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" || k.Value == "<<" || seen[k.Value] {
				return invalid("config")
			}
			seen[k.Value] = true
			if k.Value == "enabled" && v.Tag == "!!null" {
				return invalid("enabled")
			}
		}
	}
	for _, child := range n.Content {
		if err := inspectNode(child, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

func field(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
func defaultInt(value *int, n *yaml.Node, key string, def int) error {
	if field(n, key) == nil {
		*value = def
		return nil
	}
	if *value <= 0 {
		return invalid(key)
	}
	return nil
}
func applyDefaults(c *Config, n *yaml.Node) error {
	if err := defaultInt(&c.Service.MaxHeavyOperations, field(n, "service"), "max_heavy_operations", 1); err != nil {
		return err
	}
	log := field(n, "logging")
	if field(log, "max_bytes") == nil {
		c.Logging.MaxBytes = 20 << 20
	}
	if err := defaultInt(&c.Logging.MaxFiles, log, "max_files", 10); err != nil {
		return err
	}
	if err := defaultInt(&c.Logging.MaxDays, log, "max_days", 14); err != nil {
		return err
	}
	for id, j := range c.Jobs {
		if j.Enabled == nil {
			j.Enabled = new(true)
		}
		if j.Retention.ProtectLastVerified == nil {
			j.Retention.ProtectLastVerified = new(true)
		}
		if err := defaultInt(&j.Retention.MinSuccessful, field(field(field(n, "jobs"), id), "retention"), "min_successful", 1); err != nil {
			return err
		}
		c.Jobs[id] = j
	}
	for id, ch := range c.Notifications.Channels {
		if ch.Enabled == nil {
			ch.Enabled = new(true)
		}
		c.Notifications.Channels[id] = ch
	}
	return nil
}
