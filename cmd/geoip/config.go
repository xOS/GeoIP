package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config represents the configuration structure
type Config struct {
	// Server settings
	ListenAddress string `json:"listen_address,omitempty"`
	TemplateDir   string `json:"template_dir,omitempty"`
	CacheSize     int    `json:"cache_size,omitempty"`

	// Headers
	TrustedHeaders []string `json:"trusted_headers,omitempty"`

	// Features
	EnableReverseLookup bool `json:"enable_reverse_lookup,omitempty"`
	EnablePortLookup    bool `json:"enable_port_lookup,omitempty"`

	EnableProfiling bool `json:"enable_profiling,omitempty"`

	// Translation settings
	TranslationFile string `json:"translation_file,omitempty"`

	// Database paths
	Databases DatabaseConfig `json:"databases"`

	// Modes
	HybridMode    bool `json:"hybrid_mode,omitempty"`
	AutoDetect    bool `json:"auto_detect,omitempty"`
	AllowCustomIP bool `json:"allow_custom_ip,omitempty"`
}

// DatabaseConfig holds all database file paths
type DatabaseConfig struct {
	// MaxMind databases
	Country        string `json:"country,omitempty"`
	City           string `json:"city,omitempty"`
	ASN            string `json:"asn,omitempty"`
	ISP            string `json:"isp,omitempty"`
	ConnectionType string `json:"connection_type,omitempty"`

	// IP2Location/IP2Proxy databases
	IP2Proxy string `json:"ip2proxy,omitempty"`

	// QQWry databases
	QQWry string `json:"qqwry,omitempty"`

	// GeoCN.mmdb database
	GeoCN string `json:"geocn_mmdb,omitempty"`

	// CZ88 databases
	CZ88v4    string `json:"cz88v4,omitempty"`
	CZ88v6    string `json:"cz88v6,omitempty"`
	CZDBKey   string `json:"czdb_key,omitempty"`
	CZ88v4Key string `json:"cz88v4_key,omitempty"`
	CZ88v6Key string `json:"cz88v6_key,omitempty"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		ListenAddress:       ":1212",
		TemplateDir:         "html",
		CacheSize:           0,
		EnableReverseLookup: true,
		EnablePortLookup:    true,
		EnableProfiling:     true,
		HybridMode:          false,
		AutoDetect:          false,
	}
}

// LoadConfig loads configuration from a file
func LoadConfig(filename string) (*Config, error) {
	config := DefaultConfig()
	var configDir string

	// If no filename provided, try default locations
	if filename == "" {
		possiblePaths := []string{
			"geoip.json",
			"config/geoip.json",
			"/etc/geoip/config.json",
			filepath.Join(os.Getenv("HOME"), ".geoip.json"),
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				filename = path
				break
			}
		}

		if filename == "" {
			return config, nil // Return default config if no file found
		}
	}

	// Get config file directory for relative path resolution
	configDir = filepath.Dir(filename)

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", filename, err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v", filename, err)
	}

	// Resolve relative paths based on config file location
	config.resolveDatabasePaths(configDir)

	return config, nil
}

// SaveConfig saves configuration to a file
func (c *Config) SaveConfig(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %v", filename, err)
	}

	return nil
}

// generateExampleConfig creates an example configuration file
func generateExampleConfig(filename string) error {
	config := &Config{
		ListenAddress:       ":1212",
		TemplateDir:         "html",
		CacheSize:           1000,
		TrustedHeaders:      []string{
			"CF-Connecting-IP",    // Cloudflare
			"X-Real-IP",           // Nginx
			"X-Forwarded-For",     // 标准代理头
			"X-Client-IP",         // Apache
			"X-Forwarded",         // 其他代理
			"X-Cluster-Client-IP", // 集群环境
			"Forwarded-For",       // RFC 7239
			"Forwarded",           // RFC 7239
		},
		EnableReverseLookup: true,
		EnablePortLookup:    true,
		EnableProfiling:     true,
		TranslationFile:     "translations.json",
		HybridMode:          true,
		AutoDetect:          false,
		Databases: DatabaseConfig{
					Country:        "data/GeoLite2-Country.mmdb",
					City:           "data/GeoLite2-City.mmdb",
					ASN:            "data/GeoLite2-ASN.mmdb",
					ISP:            "data/GeoIP2-ISP.mmdb",
					ConnectionType: "data/GeoIP2-Connection-Type.mmdb",
					IP2Proxy:       "data/IP2PROXY-LITE-PX12.BIN",
					QQWry:          "data/qqwry.ipdb",
					GeoCN:      "data/GeoCN.mmdb",
					CZ88v4:         "data/cz88_public_v4.czdb",
					CZ88v6:         "data/cz88_public_v6.czdb",
					CZDBKey:        "tWK5IvZ6QbhBFdp8OHZ7qA==",
				},
	}

	return config.SaveConfig(filename)
}

// overrideConfigWithFlags overrides config values with command line flags when provided
func overrideConfigWithFlags(config *Config, countryFile, cityFile, asnFile, ispFile, connFile,
	ip2proxyFile, qqwryFile, geocnFile, cz88v4File, cz88v6File *string, hybridMode, autoDetect *bool, listen, template *string,
	reverseLookup, portLookup *bool, cacheSize *int, profile *bool, headers multiValueFlag) {

	// Override database paths if flags are provided
		if *countryFile != "" {
			config.Databases.Country = *countryFile
		}
		if *cityFile != "" {
			config.Databases.City = *cityFile
		}
		if *asnFile != "" {
			config.Databases.ASN = *asnFile
		}
		if *ispFile != "" {
			config.Databases.ISP = *ispFile
		}
		if *connFile != "" {
			config.Databases.ConnectionType = *connFile
		}
		if *ip2proxyFile != "" {
			config.Databases.IP2Proxy = *ip2proxyFile
		}
		if *qqwryFile != "" {
			config.Databases.QQWry = *qqwryFile
		}
		if *geocnFile != "" {
			config.Databases.GeoCN = *geocnFile
		}
		if *cz88v4File != "" {
			config.Databases.CZ88v4 = *cz88v4File
		}
		if *cz88v6File != "" {
			config.Databases.CZ88v6 = *cz88v6File
		}
		// czdb_key 由配置文件提供，如需命令行支持可自行扩展

	// Override server settings if flags are provided
	if flag.Lookup("l").Value.String() != ":1212" { // Check if -l flag was explicitly set
		config.ListenAddress = *listen
	}
	if flag.Lookup("t").Value.String() != "html" { // Check if -t flag was explicitly set
		config.TemplateDir = *template
	}
	if flag.Lookup("C").Value.String() != "0" { // Check if -C flag was explicitly set
		config.CacheSize = *cacheSize
	}

	// Override mode flags
	if flag.Lookup("hybrid").Value.String() == "true" {
		config.HybridMode = *hybridMode
	}
	if flag.Lookup("auto").Value.String() == "true" {
		config.AutoDetect = *autoDetect
	}
	if flag.Lookup("r").Value.String() == "false" {
		config.EnableReverseLookup = *reverseLookup
	}
	if flag.Lookup("p").Value.String() == "false" {
		config.EnablePortLookup = *portLookup
	}
	if flag.Lookup("P").Value.String() == "false" {
		config.EnableProfiling = *profile
	}

	// Override headers if provided
	if len(headers) > 0 {
		config.TrustedHeaders = headers
	}
}

// resolvePath resolves relative paths to absolute paths based on config file location
func resolvePath(configDir, path string) string {
	if path == "" {
		return ""
	}

	// If path is already absolute, return as-is
	if filepath.IsAbs(path) {
		return path
	}

	// For relative paths, resolve relative to config file directory
	return filepath.Join(configDir, path)
}

// validatePath checks if a file path exists and is accessible
func validatePath(path string) error {
	if path == "" {
		return nil // Empty path is valid (optional database)
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", path)
		}
		return fmt.Errorf("cannot access file %s: %v", path, err)
	}

	return nil
}

// resolveDatabasePaths resolves all relative paths in database config
func (c *Config) resolveDatabasePaths(configDir string) {
	c.Databases.Country = resolvePath(configDir, c.Databases.Country)
	c.Databases.City = resolvePath(configDir, c.Databases.City)
	c.Databases.ASN = resolvePath(configDir, c.Databases.ASN)
	c.Databases.ISP = resolvePath(configDir, c.Databases.ISP)
	c.Databases.ConnectionType = resolvePath(configDir, c.Databases.ConnectionType)
	c.Databases.IP2Proxy = resolvePath(configDir, c.Databases.IP2Proxy)
	c.Databases.QQWry = resolvePath(configDir, c.Databases.QQWry)
	c.Databases.GeoCN = resolvePath(configDir, c.Databases.GeoCN)
	c.Databases.CZ88v4 = resolvePath(configDir, c.Databases.CZ88v4)
	c.Databases.CZ88v6 = resolvePath(configDir, c.Databases.CZ88v6)

	// Also resolve template directory and translation file
	c.TemplateDir = resolvePath(configDir, c.TemplateDir)
	c.TranslationFile = resolvePath(configDir, c.TranslationFile)
}

// validateDatabasePaths validates all database paths
func (c *Config) validateDatabasePaths() error {
	paths := []struct {
		name string
		path string
	}{
		{"country database", c.Databases.Country},
		{"city database", c.Databases.City},
		{"ASN database", c.Databases.ASN},
		{"ISP database", c.Databases.ISP},
		{"connection type database", c.Databases.ConnectionType},
		{"IP2Proxy database", c.Databases.IP2Proxy},
		{"QQWry database", c.Databases.QQWry},
		{"GeoCN.mmdb database", c.Databases.GeoCN},
		{"CZ88v4 database", c.Databases.CZ88v4},
		{"CZ88v6 database", c.Databases.CZ88v6},
		{"template directory", c.TemplateDir},
		{"translation file", c.TranslationFile},
	}

	var errors []string
	for _, p := range paths {
		if err := validatePath(p.path); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", p.name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("path validation failed:\n  %s", strings.Join(errors, "\n  "))
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate database paths first
	if err := c.validateDatabasePaths(); err != nil {
		return err
	}

	// Check if at least one database is configured
		db := c.Databases
		if db.Country == "" && db.City == "" && db.ASN == "" &&
			db.ISP == "" && db.ConnectionType == "" &&
			db.IP2Proxy == "" && db.QQWry == "" &&
			db.GeoCN == "" && db.CZ88v4 == "" && db.CZ88v6 == "" {
			return fmt.Errorf("at least one database must be configured")
		}

	// Check if hybrid mode requires qqwry or cz88 database
	if c.HybridMode && (db.QQWry == "" && (db.CZ88v4 == "" || db.CZ88v6 == "")) {
		return fmt.Errorf("hybrid mode requires qqwry or cz88 database to be configured")
	}

	return nil
}

// GetTrustedHeadersString returns trusted headers as a string for compatibility
func (c *Config) GetTrustedHeadersString() string {
	return strings.Join(c.TrustedHeaders, ",")
}
