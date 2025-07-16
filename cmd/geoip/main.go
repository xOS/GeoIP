package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/xos/geoip/http"
	"github.com/xos/geoip/iputil"
	"github.com/xos/geoip/iputil/geo"
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
	ShowSponsorLogo     bool `json:"show_sponsor_logo,omitempty"`
	EnableProfiling     bool `json:"enable_profiling,omitempty"`

	// Database paths
	Databases DatabaseConfig `json:"databases"`

	// Modes
	HybridMode bool `json:"hybrid_mode,omitempty"`
	AutoDetect bool `json:"auto_detect,omitempty"`
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
}

type multiValueFlag []string

func (f *multiValueFlag) String() string {
	return strings.Join([]string(*f), ", ")
}

func (f *multiValueFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		ListenAddress:       ":1212",
		TemplateDir:         "html",
		CacheSize:           0,
		EnableReverseLookup: true,
		EnablePortLookup:    true,
		ShowSponsorLogo:     true,
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
		TrustedHeaders:      []string{"X-Real-IP", "X-Forwarded-For"},
		EnableReverseLookup: true,
		EnablePortLookup:    true,
		ShowSponsorLogo:     true,
		EnableProfiling:     true,
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
		},
	}

	return config.SaveConfig(filename)
}

// overrideConfigWithFlags overrides config values with command line flags when provided
func overrideConfigWithFlags(config *Config, countryFile, cityFile, asnFile, ispFile, connFile,
	ip2proxyFile, qqwryFile *string, hybridMode, autoDetect *bool, listen, template *string,
	reverseLookup, portLookup *bool, cacheSize *int, profile, sponsor *bool, headers multiValueFlag) {

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
	if flag.Lookup("s").Value.String() == "false" {
		config.ShowSponsorLogo = *sponsor
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

	// Also resolve template directory
	c.TemplateDir = resolvePath(configDir, c.TemplateDir)
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
		{"template directory", c.TemplateDir},
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
		db.IP2Proxy == "" && db.QQWry == "" {
		return fmt.Errorf("at least one database must be configured")
	}

	// Check if hybrid mode requires qqwry database
	if c.HybridMode && db.QQWry == "" {
		return fmt.Errorf("hybrid mode requires qqwry database to be configured")
	}

	return nil
}

// GetTrustedHeadersString returns trusted headers as a string for compatibility
func (c *Config) GetTrustedHeadersString() string {
	return strings.Join(c.TrustedHeaders, ",")
}

func init() {
	log.SetPrefix("geoip: ")
	log.SetFlags(log.Lshortfile)
}

func main() {
	// Configuration file support
	configFile := flag.String("config", "", "Path to configuration file (JSON format)")
	generateConfig := flag.String("generate-config", "", "Generate example configuration file and exit")

	// Command line flags (for backward compatibility)
	countryFile := flag.String("f", "", "Path to GeoIP country database")
	cityFile := flag.String("c", "", "Path to GeoIP city database")
	asnFile := flag.String("a", "", "Path to GeoIP ASN database")
	ispFile := flag.String("i", "", "Path to GeoIP ISP database")
	connFile := flag.String("n", "", "Path to GeoIP2 Connection-Type database")
	ip2proxyFile := flag.String("x", "", "Path to IP2Proxy database (CSV/BIN)")
	qqwryFile := flag.String("q", "", "Path to qqwry database (.ipdb or .dat format)")
	hybridMode := flag.Bool("hybrid", false, "Enable hybrid mode (use qqwry database for China mainland, MaxMind for others)")
	autoDetect := flag.Bool("auto", false, "Auto-detect database formats")
	listen := flag.String("l", ":1212", "Listening address")
	reverseLookup := flag.Bool("r", true, "Perform reverse hostname lookups")
	portLookup := flag.Bool("p", true, "Enable port lookup")
	template := flag.String("t", "html", "Path to template dir")
	cacheSize := flag.Int("C", 0, "Size of response cache. Set to 0 to disable")
	profile := flag.Bool("P", true, "Enables profiling handlers")
	sponsor := flag.Bool("s", true, "Show sponsor logo")
	var headers multiValueFlag
	flag.Var(&headers, "H", "Header to trust for remote IP, if present (e.g. X-Real-IP)")
	flag.Parse()

	if len(flag.Args()) != 0 {
		flag.Usage()
		return
	}

	// Handle config file generation
	if *generateConfig != "" {
		if err := generateExampleConfig(*generateConfig); err != nil {
			log.Fatal(err)
		}
		log.Printf("Example configuration generated: %s", *generateConfig)
		return
	}

	// Load configuration
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	// Override config with command line flags if provided
	overrideConfigWithFlags(config, countryFile, cityFile, asnFile, ispFile, connFile,
		ip2proxyFile, qqwryFile, hybridMode, autoDetect, listen, template,
		reverseLookup, portLookup, cacheSize, profile, sponsor, headers)

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}

	var r geo.Reader

	if config.HybridMode && config.Databases.QQWry != "" {
		// Use hybrid mode: qqwry database for China mainland, MaxMind for others
		r, err = geo.OpenWithHybridMode(
			config.Databases.Country,
			config.Databases.City,
			config.Databases.ASN,
			config.Databases.ISP,
			config.Databases.ConnectionType,
			config.Databases.IP2Proxy,
			config.Databases.QQWry,
		)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Using hybrid mode: qqwry database for China mainland, MaxMind for others")
	} else if config.AutoDetect {
		// Auto-detect database formats
		databases := []string{
			config.Databases.Country,
			config.Databases.City,
			config.Databases.ASN,
			config.Databases.ISP,
			config.Databases.ConnectionType,
			config.Databases.IP2Proxy,
			config.Databases.QQWry,
		}
		r, err = geo.OpenAuto(databases...)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Using auto-detection for database formats")
	} else {
		// Use traditional method
		r, err = geo.OpenWithProxy(
			config.Databases.Country,
			config.Databases.City,
			config.Databases.ASN,
			config.Databases.ISP,
			config.Databases.ConnectionType,
			config.Databases.IP2Proxy,
		)
		if err != nil {
			log.Fatal(err)
		}
	}

	cache := http.NewCache(config.CacheSize)
	server := http.New(r, cache, config.EnableProfiling)
	server.IPHeaders = config.TrustedHeaders

	if _, err := os.Stat(config.TemplateDir); err == nil {
		server.Template = config.TemplateDir
	} else {
		log.Printf("Not configuring default handler: Template not found: %s", config.TemplateDir)
	}

	if config.EnableReverseLookup {
		log.Println("Enabling reverse lookup")
		server.LookupAddr = iputil.LookupAddr
	}
	if config.EnablePortLookup {
		log.Println("Enabling port lookup")
		server.LookupPort = iputil.LookupPort
	}
	if config.ShowSponsorLogo {
		log.Println("Enabling sponsor logo")
		server.Sponsor = config.ShowSponsorLogo
	}
	if len(config.TrustedHeaders) > 0 {
		log.Printf("Trusting remote IP from header(s): %s", strings.Join(config.TrustedHeaders, ", "))
	}
	if config.CacheSize > 0 {
		log.Printf("Cache capacity set to %d", config.CacheSize)
	}
	if config.EnableProfiling {
		log.Printf("Enabling profiling handlers")
	}
	log.Printf("Listening on http://%s", config.ListenAddress)
	if err := server.ListenAndServe(config.ListenAddress); err != nil {
		log.Fatal(err)
	}
}
