package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/xos/geoip/http"
	"github.com/xos/geoip/iputil"
	"github.com/xos/geoip/iputil/geo"
)

type multiValueFlag []string

func (f *multiValueFlag) String() string {
	return strings.Join([]string(*f), ", ")
}

func (f *multiValueFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
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
