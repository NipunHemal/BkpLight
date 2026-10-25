package config

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"bkplight.local/bkplight/internal/safeerr"
	"go.yaml.in/yaml/v3"
)

func maintenance(t *testing.T) string {
	t.Helper()
	b, e := os.ReadFile("testdata/valid.yaml")
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func active(t *testing.T) string {
	s := maintenance(t)
	for _, p := range [][2]string{
		{"  staging_dir: /var/lib/bkplight/staging", "  staging_dir: /var/lib/bkplight/staging\n  staging_max_bytes: 1024\n  min_free_bytes: 1024"},
		{"sources: {}", "sources: {pg: {type: postgres, host: db.internal, port: 5432, credentials_file: /run/secrets/shared, databases: [app]}}"},
		{"destinations: {}", "destinations: {remote: {type: s3, endpoint: 'https://s3.example.invalid', bucket: fixture, prefix: backups, region: fixture, credentials_file: /run/secrets/shared}}"},
		{"jobs: {}", "jobs: {daily: {source: pg, schedule: {daily: '02:30'}, destinations: [{id: remote, required: true}], retention: {keep_last: 2}}}"},
		{"encryption: {}", "encryption: {recipients: [synthetic-public-recipient], signing_key_file: /run/secrets/sign}"},
	} {
		s = strings.Replace(s, p[0], p[1], 1)
	}
	return s
}
func reject(t *testing.T, s string) {
	t.Helper()
	_, e := Decode(strings.NewReader(s))
	if e == nil {
		t.Fatal("accepted invalid document")
	}
	if safeerr.Public(e).Code != safeerr.Invalid {
		t.Fatalf("unsafe error: %v", e)
	}
}
func accepted(t *testing.T, s string) Config {
	t.Helper()
	c, e := Decode(strings.NewReader(s))
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestDecodeRejectsDuplicateAndUnknown(t *testing.T) {
	raw := maintenance(t)
	for _, extra := range []string{"\nversion: 1\n", "\npassword: synthetic\n"} {
		reject(t, raw+extra)
	}
}
func TestDecodeDefaultsAndNestedUnknown(t *testing.T) {
	c := accepted(t, maintenance(t))
	if c.Service.MaxHeavyOperations != 1 || c.Logging.MaxBytes != 20<<20 || c.Logging.MaxFiles != 10 || c.Logging.MaxDays != 14 {
		t.Fatal("defaults missing")
	}
	a := accepted(t, active(t))
	j := a.Jobs["daily"]
	if j.Enabled == nil || !*j.Enabled || j.Retention.MinSuccessful != 1 || j.Retention.ProtectLastVerified == nil || !*j.Retention.ProtectLastVerified {
		t.Fatal("policy defaults missing")
	}
	for _, p := range [][2]string{{"timezone: Asia/Colombo", "timezone: Asia/Colombo\n  unknown: value"}, {"databases: [app]", "databases: [app], tls: {insecure_skip_verify: true}"}, {"keep_last: 2", "keep_last: 2, unknown: true"}, {"daily: '02:30'", "daily: '02:30', unknown: true"}} {
		reject(t, strings.Replace(active(t), p[0], p[1], 1))
	}
}
func TestConfigBounds(t *testing.T) {
	cases := map[string]string{"bytes": strings.Repeat(" ", 1<<20+1), "depth": "x: " + strings.Repeat("[", 33) + "0" + strings.Repeat("]", 33), "nodes": "x: [" + strings.Repeat("0,", 20001) + "0]", "alias": "version: &v 1\nx: *v", "anchor": "version: &v 1", "merge": "version: 1\nx: {<<: {a: 1}}", "multi": maintenance(t) + "\n---\nversion: 1", "nonscalar": "? [a, b]\n: value"}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) { reject(t, s) })
	}
	raw := maintenance(t)
	accepted(t, raw+strings.Repeat(" ", (1<<20)-len(raw)))
	// Exercise the structural budget directly: the closed schema has no valid
	// depth-32 shape, so unknown-field rejection must not mask these guards.
	for _, size := range []int{20000, 20001} {
		root := &yaml.Node{Kind: yaml.SequenceNode}
		for i := 1; i < size; i++ {
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "x"})
		}
		count := 0
		err := inspectNode(root, 0, &count)
		if (err != nil) != (size > 20000) {
			t.Fatalf("node budget %d: %v", size, err)
		}
	}
	for _, depth := range []int{32, 33} {
		root := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "x"}
		for i := 0; i < depth; i++ {
			root = &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{root}}
		}
		count := 0
		err := inspectNode(root, 0, &count)
		if (err != nil) != (depth > 32) {
			t.Fatalf("depth budget %d: %v", depth, err)
		}
	}
	reader := strings.NewReader(strings.Repeat(" ", 1<<20+1000))
	if _, err := Decode(reader); err == nil || reader.Len() != 999 {
		t.Fatal("decoder read beyond admission budget")
	}
}
func TestConfigReferences(t *testing.T) {
	for name, p := range map[string][2]string{
		"source":                {"source: pg", "source: missing"},
		"destination":           {"id: remote", "id: missing"},
		"channel":               {"source: pg", "notify: [missing], source: pg"},
		"duplicate_destination": {"[{id: remote, required: true}]", "[{id: remote, required: true}, {id: remote, required: false}]"},
		"target":                {"source: pg", "verification: {schedule: {daily: '03:00'}, destination: remote, target: missing, profile: portable_data}, source: pg"},
	} {
		t.Run(name, func(t *testing.T) { reject(t, strings.Replace(active(t), p[0], p[1], 1)) })
	}
}
func TestConfigPolicies(t *testing.T) {
	for name, p := range map[string][2]string{
		"required":           {"required: true", "required: false"},
		"negative":           {"keep_last: 2", "keep_last: -1"},
		"retention":          {"keep_last: 2", "keep_last: 0"},
		"timezone":           {"Asia/Colombo", "Missing/Zone"},
		"cron":               {"daily: '02:30'", "cron: '@daily'"},
		"seconds":            {"daily: '02:30'", "cron: '0 0 2 * * *'"},
		"cron_timezone":      {"daily: '02:30'", "cron: 'TZ=UTC 30 2 * * *'"},
		"mongo":              {"type: postgres", "type: mongodb"},
		"inline":             {"credentials_file: /run/secrets/shared", "password: secret"},
		"host":               {"host: db.internal", "host: 'user:secret@db.internal'"},
		"endpoint":           {"https://s3.example.invalid", "https://user:secret@s3.example.invalid"},
		"query":              {"https://s3.example.invalid", "https://s3.example.invalid?token=secret"},
		"port":               {"port: 5432", "port: 0"},
		"budget":             {"staging_max_bytes: 1024", "staging_max_bytes: 0"},
		"signing":            {"signing_key_file: /run/secrets/sign", "signing_key_file: ''"},
		"id":                 {"pg: {type:", "INVALID: {type:"},
		"duplicate_database": {"databases: [app]", "databases: [app, app]"},
		"empty_database":     {"databases: [app]", "databases: ['']"},
		"two_selectors":      {"daily: '02:30'", "daily: '02:30', cron: '30 2 * * *'"},
		"bad_daily":          {"daily: '02:30'", "daily: '24:00'"},
		"null_job":           {"source: pg", "enabled: null, source: pg"},
	} {
		t.Run(name, func(t *testing.T) { reject(t, strings.Replace(active(t), p[0], p[1], 1)) })
	}
	accepted(t, strings.Replace(active(t), "databases: [app]", "databases: [App, app, 'sales data', 'සිංහල']", 1))
}
func TestConfigRejectsNullEnabled(t *testing.T) {
	reject(t, strings.Replace(maintenance(t), "notifications: {channels: {}}", "notifications: {channels: {notice: {enabled: null, type: custom_http, url_file: /run/secrets/hook}}}", 1))
}
func TestConfigRejectsOverlappingDirectories(t *testing.T) {
	for _, p := range [][2]string{{"/var/log/bkplight", "/var/lib/bkplight/state/logs"}, {"/var/lib/bkplight/staging", "/var/lib/bkplight"}, {"/run/secrets]", "/var/lib/bkplight/state/keys]"}, {"/var/log/bkplight", "/run/secrets/logs"}, {"/var/log/bkplight", "/var/log/../log/bkplight"}, {"/var/log/bkplight", "/"}} {
		reject(t, strings.Replace(maintenance(t), p[0], p[1], 1))
	}
}
func TestTargetAndDestinationCollisions(t *testing.T) {
	target := "{restore: {type: postgres, host: DB.INTERNAL., port: 5432, identity: immutable-fixture, credentials_file: /run/secrets/target}}"
	reject(t, strings.Replace(active(t), "restore_targets: {}", "restore_targets: "+target, 1))
	s := strings.Replace(active(t), "destinations: {remote:", "destinations: {copy: {type: r2, endpoint: 'https://S3.EXAMPLE.INVALID:443/', bucket: fixture, prefix: backups/, credentials_file: /run/secrets/copy}, remote:", 1)
	reject(t, s)
	s = strings.Replace(active(t), "restore_targets: {}", "restore_targets: "+strings.Replace(target, "DB.INTERNAL.", "restore.internal", 1), 1)
	reject(t, strings.Replace(s, "verification_targets: {}", "verification_targets: "+strings.Replace(target, "DB.INTERNAL.", "verify.internal", 1), 1))
}

func TestDestinationBindingNormalizesNumericPorts(t *testing.T) {
	for name, endpoints := range map[string][2]string{
		"default_https_port": {"https://s3.example.invalid", "https://S3.EXAMPLE.INVALID:0443/"},
		"leading_zero_port":  {"https://s3.example.invalid:8443", "https://S3.EXAMPLE.INVALID:08443/"},
	} {
		t.Run(name, func(t *testing.T) {
			s := strings.Replace(active(t), "destinations: {remote:", "destinations: {copy: {type: r2, endpoint: '"+endpoints[1]+"', bucket: fixture, prefix: backups, credentials_file: /run/secrets/copy}, remote:", 1)
			s = strings.Replace(s, "endpoint: 'https://s3.example.invalid'", "endpoint: '"+endpoints[0]+"'", 1)
			reject(t, s)
		})
	}
}

func TestRevisionCanonical(t *testing.T) {
	a := accepted(t, active(t))
	b := accepted(t, strings.Replace(active(t), "daily: '02:30'", "cron: '30 2 * * *'", 1))
	ha, sa, e := Revision(a)
	if e != nil {
		t.Fatal(e)
	}
	hb, sb, e := Revision(b)
	if e != nil || ha != hb || !bytes.Equal(sa, sb) {
		t.Fatal("equivalent schedule revision differs")
	}
	c := accepted(t, strings.Replace(active(t), "daily: '02:30'", "weekly: 'SUN@02:30'", 1))
	if c.Jobs["daily"].Schedule.Cron != "30 2 * * 0" {
		t.Fatal("weekly not normalized")
	}
	b.Logging.MaxDays++
	hb, _, _ = Revision(b)
	if ha == hb {
		t.Fatal("meaningful change lost")
	}
	reversed := strings.Replace(active(t), "{type: postgres, host: db.internal, port: 5432, credentials_file: /run/secrets/shared, databases: [app]}", "{databases: [app], credentials_file: /run/secrets/shared, port: 5432, host: db.internal, type: postgres}", 1)
	hb, _, _ = Revision(accepted(t, reversed))
	if ha != hb {
		t.Fatal("mapping order affects revision")
	}
}
func TestSecretRefsActiveOnly(t *testing.T) {
	s := strings.Replace(active(t), "notifications: {channels: {}}", "notifications: {channels: {off: {enabled: false, type: custom_http, url_file: /run/secrets/off}, on: {type: custom_http, url_file: /run/secrets/on, auth_file: /run/secrets/shared}}}", 1)
	s = strings.Replace(s, "source: pg", "notify: [off, on], source: pg", 1)
	if got := SecretRefs(accepted(t, s)); !reflect.DeepEqual(got, []string{"/run/secrets/on", "/run/secrets/shared", "/run/secrets/sign"}) {
		t.Fatalf("unexpected active dependencies: %v", got)
	}
	s = strings.Replace(s, "source: pg", "enabled: false, source: pg", 1)
	if got := SecretRefs(accepted(t, s)); len(got) != 0 {
		t.Fatalf("disabled job dependencies: %v", got)
	}
}
func TestOfflineValidationDoesNotReadSecrets(t *testing.T) {
	s := strings.ReplaceAll(active(t), "/run/secrets/", "/run/secrets/nonexistent-synthetic-fixture/")
	accepted(t, s)
}

func TestTypedConfigRoundTripOmittedEnabled(t *testing.T) {
	c := accepted(t, active(t))
	j := c.Jobs["daily"]
	j.Enabled = nil
	j.Retention.ProtectLastVerified = nil
	c.Jobs["daily"] = j
	raw, err := yaml.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	accepted(t, string(raw))
}
func TestVerificationPoliciesAndDependencies(t *testing.T) {
	s := strings.Replace(active(t), "verification_targets: {}", "verification_targets: {verify: {type: postgres, host: verify.internal, port: 5432, identity: immutable-verify, credentials_file: /run/secrets/verify}}", 1)
	s = strings.Replace(s, "source: pg", "verification: {schedule: {weekly: 'SUN@03:00'}, destination: remote, target: verify, profile: portable_data}, source: pg", 1)
	s = strings.Replace(s, "recipients:", "identity_file: /run/secrets/identity, recipients:", 1)
	want := []string{"/run/secrets/identity", "/run/secrets/shared", "/run/secrets/sign", "/run/secrets/verify"}
	if got := SecretRefs(accepted(t, s)); !reflect.DeepEqual(got, want) {
		t.Fatalf("verification dependencies: %v", got)
	}
	for name, p := range map[string][2]string{"profile": {"portable_data", "unknown"},
		"engine":     {"verify: {type: postgres", "verify: {type: mongodb"},
		"identity":   {"identity_file: /run/secrets/identity, ", ""},
		"selection":  {"destination: remote", "destination: missing"},
		"incomplete": {"schedule: {weekly: 'SUN@03:00'}, ", ""}} {
		t.Run(name, func(t *testing.T) { reject(t, strings.Replace(s, p[0], p[1], 1)) })
	}
}
func TestConfigAdditionalPolicies(t *testing.T) {
	for name, p := range map[string][2]string{
		"version":                    {"version: 1", "version: 2"},
		"logging_zero":               {"logging: {directory:", "logging: {max_files: 0, directory:"},
		"min_successful_zero":        {"keep_last: 2", "keep_last: 2, min_successful: 0"},
		"heavy_zero":                 {"timezone: Asia/Colombo", "max_heavy_operations: 0\n  timezone: Asia/Colombo"},
		"tls_incomplete":             {"databases: [app]", "databases: [app], tls: {enabled: true}"},
		"secret_escape":              {"credentials_file: /run/secrets/shared", "credentials_file: /tmp/outside"},
		"engine":                     {"type: postgres", "type: unsupported"},
		"provider":                   {"type: s3", "type: unsupported"},
		"prefix":                     {"prefix: backups", "prefix: ../outside"},
		"local_zone":                 {"Asia/Colombo", "Local"},
		"unsupported_pg_consistency": {"type: postgres", "consistency: live_not_guaranteed, type: postgres"},
		"bad_weekly":                 {"daily: '02:30'", "weekly: 'mon@02:30'"},
		"empty_recipient":            {"synthetic-public-recipient", "''"},
	} {
		t.Run(name, func(t *testing.T) { reject(t, strings.Replace(active(t), p[0], p[1], 1)) })
	}
	accepted(t, strings.Replace(active(t), "type: postgres", "consistency: live_not_guaranteed, type: mongodb", 1))
	accepted(t, strings.Replace(active(t), "type: s3, endpoint: 'https://s3.example.invalid', bucket: fixture, prefix: backups, region: fixture", "type: drive, folder_id: synthetic-folder", 1))
}
func TestChannels(t *testing.T) {
	s := strings.Replace(active(t), "notifications: {channels: {}}", "notifications: {channels: {notice: {type: discord, url_file: /run/secrets/hook, events: [backup.failed, storage.low_space]}}}", 1)
	accepted(t, s)
	for _, p := range [][2]string{{"backup.failed", "unknown"}, {"type: discord", "type: unsupported"}, {"type: discord", "type: discord, mentions: ['all']"}, {"type: discord", "type: discord, auth_file: /run/secrets/auth"}, {"url_file: /run/secrets/hook", "url: https://example.invalid"}} {
		reject(t, strings.Replace(s, p[0], p[1], 1))
	}
}

func TestTLSActiveDependencies(t *testing.T) {
	s := strings.Replace(active(t), "databases: [app]", "databases: [app], tls: {enabled: true, ca_file: /run/secrets/ca, server_name: db.internal}", 1)
	want := []string{"/run/secrets/ca", "/run/secrets/shared", "/run/secrets/sign"}
	if got := SecretRefs(accepted(t, s)); !reflect.DeepEqual(got, want) {
		t.Fatalf("TLS dependencies: %v", got)
	}
	reject(t, strings.Replace(s, "ca_file: /run/secrets/ca", "ca_file: /tmp/outside-ca", 1))
	s = strings.Replace(s, "enabled: true", "enabled: false", 1)
	want = []string{"/run/secrets/shared", "/run/secrets/sign"}
	if got := SecretRefs(accepted(t, s)); !reflect.DeepEqual(got, want) {
		t.Fatalf("disabled TLS dependency retained: %v", got)
	}
}
