package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// Revision hashes normalized configuration containing references, never secret
// contents. Re-decode a copy so caller-owned maps and pointers remain unchanged.
func Revision(c Config) (id string, snapshot []byte, err error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return "", nil, invalid("config")
	}
	var normalized Config
	if json.Unmarshal(raw, &normalized) != nil {
		return "", nil, invalid("config")
	}
	// Apply the same defaults for callers constructing typed configurations.
	if normalized.Service.MaxHeavyOperations == 0 {
		normalized.Service.MaxHeavyOperations = 1
	}
	if normalized.Logging.MaxBytes == 0 {
		normalized.Logging.MaxBytes = 20 << 20
	}
	if normalized.Logging.MaxFiles == 0 {
		normalized.Logging.MaxFiles = 10
	}
	if normalized.Logging.MaxDays == 0 {
		normalized.Logging.MaxDays = 14
	}
	for k, j := range normalized.Jobs {
		if j.Enabled == nil {
			j.Enabled = new(true)
		}
		if j.Retention.MinSuccessful == 0 {
			j.Retention.MinSuccessful = 1
		}
		if j.Retention.ProtectLastVerified == nil {
			j.Retention.ProtectLastVerified = new(true)
		}
		normalized.Jobs[k] = j
	}
	for k, ch := range normalized.Notifications.Channels {
		if ch.Enabled == nil {
			ch.Enabled = new(true)
		}
		normalized.Notifications.Channels[k] = ch
	}
	if err = validate(&normalized); err != nil {
		return "", nil, err
	}
	canonicalCollections(&normalized)
	snapshot, err = json.Marshal(normalized)
	if err != nil {
		return "", nil, invalid("config")
	}
	sum := sha256.Sum256(snapshot)
	return hex.EncodeToString(sum[:]), snapshot, nil
}

func canonicalCollections(c *Config) {
	if c.Sources == nil {
		c.Sources = map[string]Source{}
	}
	if c.Destinations == nil {
		c.Destinations = map[string]Destination{}
	}
	if c.Jobs == nil {
		c.Jobs = map[string]Job{}
	}
	if c.RestoreTargets == nil {
		c.RestoreTargets = map[string]Target{}
	}
	if c.VerificationTargets == nil {
		c.VerificationTargets = map[string]Target{}
	}
	if c.Notifications.Channels == nil {
		c.Notifications.Channels = map[string]ChannelConfig{}
	}
	if c.Service.SecretRoots == nil {
		c.Service.SecretRoots = []string{}
	}
	if c.Encryption.Recipients == nil {
		c.Encryption.Recipients = []string{}
	}
	for k, j := range c.Jobs {
		if j.Notify == nil {
			j.Notify = []string{}
		}
		if j.Destinations == nil {
			j.Destinations = []JobDestination{}
		}
		c.Jobs[k] = j
	}
	for k, ch := range c.Notifications.Channels {
		if ch.Events == nil {
			ch.Events = []string{}
		}
		c.Notifications.Channels[k] = ch
	}
}

// SecretRefs returns unique sorted dependencies of enabled jobs and their enabled
// channels. Merely declaring a source, destination or target does not activate it.
func SecretRefs(c Config) []string {
	paths := map[string]bool{}
	add := func(p string) {
		if p != "" {
			paths[p] = true
		}
	}
	tls := func(v TLS) {
		if v.Enabled {
			add(v.CAFile)
		}
	}
	for _, j := range c.Jobs {
		if !isEnabled(j.Enabled) {
			continue
		}
		source := c.Sources[j.Source]
		add(source.CredentialsFile)
		tls(source.TLS)
		for _, d := range j.Destinations {
			add(c.Destinations[d.ID].CredentialsFile)
		}
		add(c.Encryption.SigningKeyFile)
		if j.Verification.Schedule != nil {
			add(c.Encryption.IdentityFile)
			target := c.VerificationTargets[j.Verification.Target]
			add(target.CredentialsFile)
			tls(target.TLS)
		}
		for _, id := range j.Notify {
			ch := c.Notifications.Channels[id]
			if !isEnabled(ch.Enabled) {
				continue
			}
			add(ch.URLFile)
			add(ch.AuthFile)
			add(ch.HMACFile)
		}
	}
	result := make([]string, 0, len(paths))
	for p := range paths {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}
