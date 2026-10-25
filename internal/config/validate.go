package config

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/robfig/cron/v3"
)

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var clockPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
var scheduleParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func isEnabled(b *bool) bool { return b == nil || *b }
func textOK(s string) bool   { return s != "" && !strings.ContainsFunc(s, unicode.IsControl) }
func privatePath(s string) bool {
	return path.IsAbs(s) && s != "/" && path.Clean(s) == s && !strings.ContainsAny(s, "\\\x00") && !strings.ContainsFunc(s, unicode.IsControl)
}
func within(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }
func overlap(a, b string) bool   { return within(a, b) || within(b, a) }
func hostOK(s string) bool {
	if !textOK(s) || strings.ContainsAny(s, "/@?#\\ ") || strings.ContainsFunc(s, unicode.IsSpace) {
		return false
	}
	if net.ParseIP(s) != nil {
		return true
	}
	if strings.Contains(s, ":") {
		return false
	}
	s = strings.TrimSuffix(s, ".")
	if s == "" || len(s) > 253 {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}
func normalizedHost(s string) string {
	if ip := net.ParseIP(s); ip != nil {
		return ip.String()
	}
	return strings.ToLower(strings.TrimSuffix(s, "."))
}
func endpointKey(engine, host string, port int) string {
	return engine + "|" + normalizedHost(host) + "|" + strconv.Itoa(port)
}
func zoneOK(zone string) bool {
	if zone == "" || zone == "Local" || strings.Contains(zone, "..") || strings.HasPrefix(zone, "/") {
		return false
	}
	_, err := time.LoadLocation(zone)
	return err == nil
}

func validate(c *Config) error {
	if c.Version != 1 {
		return invalid("version")
	}
	if !zoneOK(c.Service.Timezone) {
		return invalid("service.timezone")
	}
	dirs := []string{c.Service.StateDir, c.Service.StagingDir, c.Logging.Directory}
	for i, p := range dirs {
		if !privatePath(p) {
			return invalid("directories")
		}
		for _, q := range dirs[:i] {
			if overlap(p, q) {
				return invalid("directories")
			}
		}
	}
	if !privatePath(c.Service.SocketPath) {
		return invalid("service.socket_path")
	}
	if c.Service.MaxHeavyOperations <= 0 || c.Service.StagingMaxBytes < 0 || c.Service.MinFreeBytes < 0 {
		return invalid("service")
	}
	if c.Logging.MaxBytes <= 0 || c.Logging.MaxFiles <= 0 || c.Logging.MaxDays <= 0 {
		return invalid("logging")
	}
	roots := map[string]bool{}
	for _, root := range c.Service.SecretRoots {
		if !privatePath(root) || roots[root] {
			return invalid("service.secret_roots")
		}
		roots[root] = true
		for _, d := range dirs {
			if overlap(d, root) {
				return invalid("directories")
			}
		}
		if within(c.Service.SocketPath, root) {
			return invalid("service.socket_path")
		}
	}
	for _, p := range []string{c.Encryption.SigningKeyFile, c.Encryption.IdentityFile} {
		if p != "" && !secretPath(c, p) {
			return invalid("encryption")
		}
	}
	recipients := map[string]bool{}
	for _, r := range c.Encryption.Recipients {
		if !textOK(r) || strings.TrimSpace(r) == "" || recipients[r] {
			return invalid("encryption.recipients")
		}
		recipients[r] = true
	}
	sourceEndpoints := map[string]bool{}
	for id, s := range c.Sources {
		if !idPattern.MatchString(id) {
			return invalid("sources")
		}
		if err := connection(c, s.Type, s.Host, s.Port, s.CredentialsFile, s.TLS); err != nil {
			return err
		}
		if len(s.Databases) == 0 {
			return invalid("sources.databases")
		}
		seen := map[string]bool{}
		for _, db := range s.Databases {
			if !textOK(db) || seen[db] {
				return invalid("sources.databases")
			}
			seen[db] = true
		}
		if s.Type == "mongodb" {
			if s.Consistency != "live_not_guaranteed" || s.Globals {
				return invalid("sources.consistency")
			}
		} else if s.Consistency != "" && s.Consistency != "postgres_per_database" {
			return invalid("sources.consistency")
		}
		sourceEndpoints[endpointKey(s.Type, s.Host, s.Port)] = true
	}
	identities := map[string]bool{}
	targetEndpoints := map[string]bool{}
	for _, targets := range []map[string]Target{c.RestoreTargets, c.VerificationTargets} {
		for id, target := range targets {
			if !idPattern.MatchString(id) {
				return invalid("targets")
			}
			if err := connection(c, target.Type, target.Host, target.Port, target.CredentialsFile, target.TLS); err != nil {
				return err
			}
			key := endpointKey(target.Type, target.Host, target.Port)
			if !textOK(target.Identity) || strings.TrimSpace(target.Identity) == "" || identities[target.Identity] || sourceEndpoints[key] || targetEndpoints[key] {
				return invalid("targets.identity")
			}
			identities[target.Identity] = true
			targetEndpoints[key] = true
		}
	}
	bindings := map[string]bool{}
	for id, d := range c.Destinations {
		if !idPattern.MatchString(id) || !secretPath(c, d.CredentialsFile) {
			return invalid("destinations")
		}
		binding, err := destinationBinding(d)
		if err != nil {
			return err
		}
		if bindings[binding] {
			return invalid("destinations")
		}
		bindings[binding] = true
	}
	for id, ch := range c.Notifications.Channels {
		if !idPattern.MatchString(id) || (ch.Type != "custom_http" && ch.Type != "discord") {
			return invalid("notifications.channels")
		}
		if !secretPath(c, ch.URLFile) {
			return invalid("notifications.url_file")
		}
		for _, p := range []string{ch.AuthFile, ch.HMACFile} {
			if p != "" && !secretPath(c, p) {
				return invalid("notifications.channels")
			}
		}
		if ch.Type == "discord" && (ch.AuthFile != "" || ch.HMACFile != "") {
			return invalid("notifications.channels")
		}
		seen := map[string]bool{}
		for _, event := range ch.Events {
			if !allowedEvent(event) || seen[event] {
				return invalid("notifications.events")
			}
			seen[event] = true
		}
	}
	for id, j := range c.Jobs {
		if !idPattern.MatchString(id) {
			return invalid("jobs")
		}
		source, ok := c.Sources[j.Source]
		if !ok {
			return invalid("jobs.source")
		}
		if err := normalizeSchedule(&j.Schedule, c.Service.Timezone); err != nil {
			return err
		}
		selected := map[string]bool{}
		required := false
		for _, d := range j.Destinations {
			if _, ok := c.Destinations[d.ID]; !ok || selected[d.ID] {
				return invalid("jobs.destinations")
			}
			selected[d.ID] = true
			required = required || d.Required
		}
		if isEnabled(j.Enabled) && !required {
			return invalid("jobs.destinations")
		}
		if j.Retention.KeepLast < 0 || j.Retention.KeepDays < 0 || (j.Retention.KeepLast == 0 && j.Retention.KeepDays == 0) || j.Retention.MinSuccessful < 1 {
			return invalid("jobs.retention")
		}
		seen := map[string]bool{}
		for _, ch := range j.Notify {
			if _, ok := c.Notifications.Channels[ch]; !ok || seen[ch] {
				return invalid("jobs.notify")
			}
			seen[ch] = true
		}
		v := &j.Verification
		if v.Schedule != nil || v.Destination != "" || v.Target != "" || v.Profile != "" {
			target, exists := c.VerificationTargets[v.Target]
			if v.Schedule == nil || !selected[v.Destination] || !exists || target.Type != source.Type || (v.Profile != "portable_data" && v.Profile != "faithful") {
				return invalid("jobs.verification")
			}
			if err := normalizeSchedule(v.Schedule, j.Schedule.Timezone); err != nil {
				return err
			}
			if isEnabled(j.Enabled) && c.Encryption.IdentityFile == "" {
				return invalid("encryption.identity_file")
			}
		}
		if isEnabled(j.Enabled) {
			if c.Service.StagingMaxBytes <= 0 || c.Service.MinFreeBytes <= 0 {
				return invalid("service")
			}
			if len(c.Encryption.Recipients) == 0 || c.Encryption.SigningKeyFile == "" {
				return invalid("encryption")
			}
		}
		c.Jobs[id] = j
	}
	return nil
}

func secretPath(c *Config, p string) bool {
	if !privatePath(p) {
		return false
	}
	for _, root := range c.Service.SecretRoots {
		if p != root && within(p, root) {
			return true
		}
	}
	return false
}
func connection(c *Config, engine, host string, port int, credentials string, tls TLS) error {
	if (engine != "postgres" && engine != "mongodb") || !hostOK(host) || port < 1 || port > 65535 || !secretPath(c, credentials) {
		return invalid("connection")
	}
	if tls.CAFile != "" && !secretPath(c, tls.CAFile) {
		return invalid("tls.ca_file")
	}
	if tls.ServerName != "" && !hostOK(tls.ServerName) {
		return invalid("tls.server_name")
	}
	if tls.Enabled && (tls.CAFile == "" || tls.ServerName == "") {
		return invalid("tls")
	}
	return nil
}

func destinationBinding(d Destination) (string, error) {
	if d.Type != "s3" && d.Type != "r2" && d.Type != "drive" {
		return "", invalid("destinations.type")
	}
	endpoint := ""
	if d.Endpoint != "" {
		u, err := url.Parse(d.Endpoint)
		if err != nil || u.Scheme != "https" || !hostOK(u.Hostname()) || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(d.Endpoint, "#") || u.Opaque != "" {
			return "", invalid("destinations.endpoint")
		}
		port := u.Port()
		if port != "" {
			p, e := strconv.Atoi(port)
			if e != nil || p < 1 || p > 65535 {
				return "", invalid("destinations.endpoint")
			}
		}
		host := normalizedHost(u.Hostname())
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		if port != "" && port != "443" {
			host += ":" + port
		}
		endpoint = "https://" + host + strings.TrimRight(u.EscapedPath(), "/")
	}
	if d.Type == "drive" {
		if !textOK(d.FolderID) || d.Bucket != "" || d.Prefix != "" || d.Region != "" {
			return "", invalid("destinations")
		}
		return "drive|" + d.FolderID, nil
	}
	if endpoint == "" || !textOK(d.Bucket) || strings.ContainsAny(d.Bucket, "/\\ ") || !textOK(d.Prefix) || d.FolderID != "" {
		return "", invalid("destinations")
	}
	prefix := strings.TrimSuffix(d.Prefix, "/")
	if prefix == "" || path.IsAbs(prefix) || path.Clean(prefix) != prefix || strings.Contains(prefix, "\\") || prefix == ".." || strings.HasPrefix(prefix, "../") {
		return "", invalid("destinations.prefix")
	}
	return endpoint + "|" + d.Bucket + "|" + prefix, nil
}

func normalizeSchedule(s *Schedule, defaultZone string) error {
	selectors := 0
	for _, v := range []string{s.Cron, s.Daily, s.Weekly} {
		if v != "" {
			selectors++
		}
	}
	if selectors != 1 {
		return invalid("schedule")
	}
	if s.Timezone == "" {
		s.Timezone = defaultZone
	}
	if !zoneOK(s.Timezone) {
		return invalid("schedule.timezone")
	}
	clock := s.Daily
	dow := "*"
	if s.Weekly != "" {
		parts := strings.Split(s.Weekly, "@")
		if len(parts) != 2 {
			return invalid("schedule.weekly")
		}
		days := map[string]string{"SUN": "0", "MON": "1", "TUE": "2", "WED": "3", "THU": "4", "FRI": "5", "SAT": "6"}
		var ok bool
		dow, ok = days[parts[0]]
		if !ok {
			return invalid("schedule.weekly")
		}
		clock = parts[1]
	}
	if clock != "" {
		if !clockPattern.MatchString(clock) {
			return invalid("schedule")
		}
		hour, _ := strconv.Atoi(clock[:2])
		minute, _ := strconv.Atoi(clock[3:])
		s.Cron = fmt.Sprintf("%d %d * * %s", minute, hour, dow)
	}
	fields := strings.Fields(s.Cron)
	if len(fields) != 5 {
		return invalid("schedule.cron")
	}
	s.Cron = strings.Join(fields, " ")
	if _, err := scheduleParser.Parse(s.Cron); err != nil {
		return invalid("schedule.cron")
	}
	s.Daily = ""
	s.Weekly = ""
	return nil
}

func allowedEvent(event string) bool {
	switch event {
	case "backup.started", "backup.completed", "backup.degraded", "backup.failed", "upload.completed", "upload.failed", "archive_validation.completed", "archive_validation.failed", "verification.started", "verification.completed", "verification.failed", "restore.started", "restore.completed", "restore.failed", "retention.completed", "retention.failed", "backup.overdue", "storage.low_space":
		return true
	}
	return false
}
