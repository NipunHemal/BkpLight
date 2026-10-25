// Package config decodes the closed version 1 configuration without resource IO.
package config

type Config struct {
	Version             int                    `yaml:"version" json:"version"`
	Service             Service                `yaml:"service" json:"service"`
	Sources             map[string]Source      `yaml:"sources" json:"sources"`
	Destinations        map[string]Destination `yaml:"destinations" json:"destinations"`
	Jobs                map[string]Job         `yaml:"jobs" json:"jobs"`
	RestoreTargets      map[string]Target      `yaml:"restore_targets" json:"restore_targets"`
	VerificationTargets map[string]Target      `yaml:"verification_targets" json:"verification_targets"`
	Encryption          Encryption             `yaml:"encryption" json:"encryption"`
	Notifications       Notifications          `yaml:"notifications" json:"notifications"`
	Logging             Logging                `yaml:"logging" json:"logging"`
}
type Service struct {
	Timezone           string   `yaml:"timezone" json:"timezone"`
	StateDir           string   `yaml:"state_dir" json:"state_dir"`
	SocketPath         string   `yaml:"socket_path" json:"socket_path"`
	SecretRoots        []string `yaml:"secret_roots" json:"secret_roots"`
	StagingDir         string   `yaml:"staging_dir" json:"staging_dir"`
	StagingMaxBytes    int64    `yaml:"staging_max_bytes" json:"staging_max_bytes"`
	MinFreeBytes       int64    `yaml:"min_free_bytes" json:"min_free_bytes"`
	MaxHeavyOperations int      `yaml:"max_heavy_operations" json:"max_heavy_operations"`
}
type Source struct {
	Type            string   `yaml:"type" json:"type"`
	Host            string   `yaml:"host" json:"host"`
	Port            int      `yaml:"port" json:"port"`
	CredentialsFile string   `yaml:"credentials_file" json:"credentials_file"`
	Databases       []string `yaml:"databases" json:"databases"`
	Consistency     string   `yaml:"consistency" json:"consistency"`
	Globals         bool     `yaml:"globals" json:"globals"`
	TLS             TLS      `yaml:"tls" json:"tls"`
}
type TLS struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	CAFile     string `yaml:"ca_file" json:"ca_file"`
	ServerName string `yaml:"server_name" json:"server_name"`
}
type Destination struct {
	Type            string `yaml:"type" json:"type"`
	Endpoint        string `yaml:"endpoint" json:"endpoint"`
	Bucket          string `yaml:"bucket" json:"bucket"`
	Prefix          string `yaml:"prefix" json:"prefix"`
	Region          string `yaml:"region" json:"region"`
	FolderID        string `yaml:"folder_id" json:"folder_id"`
	CredentialsFile string `yaml:"credentials_file" json:"credentials_file"`
}
type Job struct {
	Enabled      *bool            `yaml:"enabled,omitempty" json:"enabled"`
	Source       string           `yaml:"source" json:"source"`
	Schedule     Schedule         `yaml:"schedule" json:"schedule"`
	Destinations []JobDestination `yaml:"destinations" json:"destinations"`
	Retention    Retention        `yaml:"retention" json:"retention"`
	Verification Verification     `yaml:"verification" json:"verification"`
	Notify       []string         `yaml:"notify" json:"notify"`
}
type Schedule struct {
	Cron     string `yaml:"cron" json:"cron"`
	Daily    string `yaml:"daily" json:"daily"`
	Weekly   string `yaml:"weekly" json:"weekly"`
	Timezone string `yaml:"timezone" json:"timezone"`
}
type JobDestination struct {
	ID       string `yaml:"id" json:"id"`
	Required bool   `yaml:"required" json:"required"`
}
type Retention struct {
	KeepLast            int   `yaml:"keep_last" json:"keep_last"`
	KeepDays            int   `yaml:"keep_days" json:"keep_days"`
	MinSuccessful       int   `yaml:"min_successful" json:"min_successful"`
	ProtectLastVerified *bool `yaml:"protect_last_verified,omitempty" json:"protect_last_verified"`
}
type Verification struct {
	Schedule    *Schedule `yaml:"schedule" json:"schedule"`
	Destination string    `yaml:"destination" json:"destination"`
	Target      string    `yaml:"target" json:"target"`
	Profile     string    `yaml:"profile" json:"profile"`
}
type Target struct {
	Type            string `yaml:"type" json:"type"`
	Host            string `yaml:"host" json:"host"`
	Port            int    `yaml:"port" json:"port"`
	Identity        string `yaml:"identity" json:"identity"`
	CredentialsFile string `yaml:"credentials_file" json:"credentials_file"`
	TLS             TLS    `yaml:"tls" json:"tls"`
}
type Encryption struct {
	Recipients     []string `yaml:"recipients" json:"recipients"`
	SigningKeyFile string   `yaml:"signing_key_file" json:"signing_key_file"`
	IdentityFile   string   `yaml:"identity_file" json:"identity_file"`
}
type Notifications struct {
	Channels map[string]ChannelConfig `yaml:"channels" json:"channels"`
}
type ChannelConfig struct {
	Enabled  *bool    `yaml:"enabled,omitempty" json:"enabled"`
	Type     string   `yaml:"type" json:"type"`
	URLFile  string   `yaml:"url_file" json:"url_file"`
	AuthFile string   `yaml:"auth_file" json:"auth_file"`
	HMACFile string   `yaml:"hmac_file" json:"hmac_file"`
	Events   []string `yaml:"events" json:"events"`
}
type Logging struct {
	Directory string `yaml:"directory" json:"directory"`
	MaxBytes  int64  `yaml:"max_bytes" json:"max_bytes"`
	MaxFiles  int    `yaml:"max_files" json:"max_files"`
	MaxDays   int    `yaml:"max_days" json:"max_days"`
}
