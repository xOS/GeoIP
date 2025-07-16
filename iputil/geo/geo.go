package geo

import (
	"fmt"
	"math"
	"net"

	geoip2 "github.com/oschwald/geoip2-golang"
)

type Reader interface {
	Country(net.IP) (Country, error)
	City(net.IP) (City, error)
	ASN(net.IP) (ASN, error)
	ISP(net.IP) (ISP, error)
	ConnectionType(net.IP) (ConnectionType, error)
	Proxy(net.IP) (Proxy, error)
	IsEmpty() bool
}

type Country struct {
	Name string
	ISO  string
	IsEU *bool
}

type City struct {
	Name       string
	Latitude   float64
	Longitude  float64
	PostalCode string
	Timezone   string
	MetroCode  uint
	RegionName string
	RegionCode string
}

type ASN struct {
	AutonomousSystemNumber       uint
	AutonomousSystemOrganization string
}

type ISP struct {
	ASN          uint
	ORG          string
	ISP          string
	Organization string
}

type ConnectionType struct {
	ConnectionType string
}

type Proxy struct {
	IsProxy     bool
	ProxyType   string
	Country     string
	CountryCode string
	Domain      string
	UsageType   string
	LastSeen    string
	Threat      string
	Provider    string
	FraudScore  string
}

type geoip struct {
	country        *geoip2.Reader
	city           *geoip2.Reader
	asn            *geoip2.Reader
	isp            *geoip2.Reader
	connectiontype *geoip2.Reader
}

func Open(countryDB, cityDB string, asnDB string, ispDB string, connectiontypeDB string) (Reader, error) {
	return OpenWithProxy(countryDB, cityDB, asnDB, ispDB, connectiontypeDB, "")
}

func OpenWithProxy(countryDB, cityDB string, asnDB string, ispDB string, connectiontypeDB string, ip2proxyDB string) (Reader, error) {
	return OpenWithAutoDetection(countryDB, cityDB, asnDB, ispDB, connectiontypeDB, ip2proxyDB)
}

func OpenWithAutoDetection(countryDB, cityDB string, asnDB string, ispDB string, connectiontypeDB string, ip2proxyDB string) (Reader, error) {
	var readers []Reader
	var maxmindReader Reader
	var ip2Reader Reader
	var qqwryReader Reader

	// Process MaxMind databases
	var country, city, asn, isp, connectiontype *geoip2.Reader

	// Auto-detect and load each database
	databases := map[string]*string{
		"country":        &countryDB,
		"city":           &cityDB,
		"asn":            &asnDB,
		"isp":            &ispDB,
		"connectiontype": &connectiontypeDB,
	}

	for dbType, dbPath := range databases {
		if *dbPath == "" {
			continue
		}

		// Detect if this is actually an IP2Location/IP2Proxy database
		detectedType, err := DetectDatabaseType(*dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to detect database type for %s: %v", *dbPath, err)
		}

		switch detectedType {
		case DatabaseTypeMaxMindMMDB:
			// Handle MaxMind MMDB
			r, err := geoip2.Open(*dbPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open MaxMind database %s: %v", *dbPath, err)
			}
			switch dbType {
			case "country":
				country = r
			case "city":
				city = r
			case "asn":
				asn = r
			case "isp":
				isp = r
			case "connectiontype":
				connectiontype = r
			}

		case DatabaseTypeIP2LocationBIN, DatabaseTypeIP2ProxyBIN, DatabaseTypeIP2ProxyCSV:
			// Handle IP2Location/IP2Proxy databases
			reader, err := CreateReader(*dbPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create IP2 reader for %s: %v", *dbPath, err)
			}
			if ip2Reader == nil {
				ip2Reader = reader
			} else {
				// If we already have an IP2 reader, combine them
				ip2Reader = NewCombinedReader(ip2Reader, reader)
			}

		case DatabaseTypeQQWryIPDB, DatabaseTypeQQWryDAT:
			// Handle qqwry.ipdb or qqwry.dat database
			reader, err := CreateReader(*dbPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create qqwry reader for %s: %v", *dbPath, err)
			}
			qqwryReader = reader

		default:
			return nil, fmt.Errorf("unsupported database type for %s", *dbPath)
		}
	}

	// Create MaxMind reader if we have any MaxMind databases
	if country != nil || city != nil || asn != nil || isp != nil || connectiontype != nil {
		maxmindReader = &geoip{country: country, city: city, asn: asn, isp: isp, connectiontype: connectiontype}
	}

	// Handle the explicit IP2Proxy database parameter
	if ip2proxyDB != "" {
		reader, err := CreateReader(ip2proxyDB)
		if err != nil {
			return nil, fmt.Errorf("failed to create IP2Proxy reader: %v", err)
		}
		if ip2Reader == nil {
			ip2Reader = reader
		} else {
			ip2Reader = NewCombinedReader(ip2Reader, reader)
		}
	}

	// Create hybrid reader if we have both MaxMind and qqwry
	if maxmindReader != nil && qqwryReader != nil {
		hybridReader := NewHybridReader(maxmindReader, qqwryReader)

		// If we also have IP2 databases, combine with hybrid
		if ip2Reader != nil {
			return NewCombinedReader(hybridReader, ip2Reader), nil
		}

		return hybridReader, nil
	}

	// Add readers to the list
	if maxmindReader != nil {
		readers = append(readers, maxmindReader)
	}
	if qqwryReader != nil {
		readers = append(readers, qqwryReader)
	}
	if ip2Reader != nil {
		readers = append(readers, ip2Reader)
	}

	// Return appropriate reader based on what we have
	if len(readers) == 0 {
		return &EmptyReader{}, nil
	} else if len(readers) == 1 {
		return readers[0], nil
	} else {
		// Combine all readers, with IP2 taking priority
		if ip2Reader != nil && maxmindReader != nil {
			return NewCombinedReader(maxmindReader, ip2Reader), nil
		}
		return readers[0], nil
	}
}

func OpenWithHybridMode(countryDB, cityDB string, asnDB string, ispDB string, connectiontypeDB string, ip2proxyDB string, qqwryDB string) (Reader, error) {
	var readers []Reader
	var maxmindReader Reader
	var ip2Reader Reader
	var qqwryReader Reader

	// Process MaxMind databases
	var country, city, asn, isp, connectiontype *geoip2.Reader

	// Auto-detect and load each database
	databases := map[string]*string{
		"country":        &countryDB,
		"city":           &cityDB,
		"asn":            &asnDB,
		"isp":            &ispDB,
		"connectiontype": &connectiontypeDB,
	}

	for dbType, dbPath := range databases {
		if *dbPath == "" {
			continue
		}

		// Detect if this is actually an IP2Location/IP2Proxy database
		detectedType, err := DetectDatabaseType(*dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to detect database type for %s: %v", *dbPath, err)
		}

		switch detectedType {
		case DatabaseTypeMaxMindMMDB:
			// Handle MaxMind MMDB
			r, err := geoip2.Open(*dbPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open MaxMind database %s: %v", *dbPath, err)
			}
			switch dbType {
			case "country":
				country = r
			case "city":
				city = r
			case "asn":
				asn = r
			case "isp":
				isp = r
			case "connectiontype":
				connectiontype = r
			}

		case DatabaseTypeIP2LocationBIN, DatabaseTypeIP2ProxyBIN, DatabaseTypeIP2ProxyCSV:
			// Handle IP2Location/IP2Proxy databases
			reader, err := CreateReader(*dbPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create IP2 reader for %s: %v", *dbPath, err)
			}
			if ip2Reader == nil {
				ip2Reader = reader
			} else {
				// If we already have an IP2 reader, combine them
				ip2Reader = NewCombinedReader(ip2Reader, reader)
			}

		case DatabaseTypeQQWryIPDB, DatabaseTypeQQWryDAT:
			// Handle qqwry.ipdb or qqwry.dat database
			reader, err := CreateReader(*dbPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create qqwry reader for %s: %v", *dbPath, err)
			}
			qqwryReader = reader

		default:
			return nil, fmt.Errorf("unsupported database type for %s", *dbPath)
		}
	}

	// Create MaxMind reader if we have any MaxMind databases
	if country != nil || city != nil || asn != nil || isp != nil || connectiontype != nil {
		maxmindReader = &geoip{country: country, city: city, asn: asn, isp: isp, connectiontype: connectiontype}
	}

	// Handle the explicit IP2Proxy database parameter
	if ip2proxyDB != "" {
		reader, err := CreateReader(ip2proxyDB)
		if err != nil {
			return nil, fmt.Errorf("failed to create IP2Proxy reader: %v", err)
		}
		if ip2Reader == nil {
			ip2Reader = reader
		} else {
			ip2Reader = NewCombinedReader(ip2Reader, reader)
		}
	}

	// Handle the explicit qqwry database parameter (supports both .ipdb and .dat)
	if qqwryDB != "" {
		reader, err := CreateReader(qqwryDB)
		if err != nil {
			return nil, fmt.Errorf("failed to create qqwry reader: %v", err)
		}
		qqwryReader = reader
	}

	// Create hybrid reader if we have both MaxMind and qqwry
	if maxmindReader != nil && qqwryReader != nil {
		hybridReader := NewHybridReader(maxmindReader, qqwryReader)

		// If we also have IP2 databases, combine with hybrid
		if ip2Reader != nil {
			return NewCombinedReader(hybridReader, ip2Reader), nil
		}

		return hybridReader, nil
	}

	// Add readers in priority order
	if maxmindReader != nil {
		readers = append(readers, maxmindReader)
	}
	if qqwryReader != nil {
		readers = append(readers, qqwryReader)
	}
	if ip2Reader != nil {
		readers = append(readers, ip2Reader)
	}

	// Return appropriate reader based on what we have
	if len(readers) == 0 {
		return &EmptyReader{}, nil
	} else if len(readers) == 1 {
		return readers[0], nil
	} else {
		// Combine all readers, with IP2 taking priority
		if ip2Reader != nil && len(readers) > 1 {
			// Find the non-IP2 reader
			var otherReader Reader
			for _, r := range readers {
				if r != ip2Reader {
					otherReader = r
					break
				}
			}
			if otherReader != nil {
				return NewCombinedReader(otherReader, ip2Reader), nil
			}
		}
		return readers[0], nil
	}
}

func (g *geoip) Country(ip net.IP) (Country, error) {
	country := Country{}
	if g.country == nil {
		return country, nil
	}
	record, err := g.country.Country(ip)
	if err != nil {
		return country, err
	}
	if c, exists := record.Country.Names["en"]; exists {
		country.Name = c
	}
	if c, exists := record.RegisteredCountry.Names["en"]; exists && country.Name == "" {
		country.Name = c
	}
	if record.Country.IsoCode != "" {
		country.ISO = record.Country.IsoCode
	}
	if record.RegisteredCountry.IsoCode != "" && country.ISO == "" {
		country.ISO = record.RegisteredCountry.IsoCode
	}
	isEU := record.Country.IsInEuropeanUnion || record.RegisteredCountry.IsInEuropeanUnion
	country.IsEU = &isEU
	return country, nil
}

func (g *geoip) City(ip net.IP) (City, error) {
	city := City{}
	if g.city == nil {
		return city, nil
	}
	record, err := g.city.City(ip)
	if err != nil {
		return city, err
	}
	if c, exists := record.City.Names["en"]; exists {
		city.Name = c
	}
	if len(record.Subdivisions) > 0 {
		if c, exists := record.Subdivisions[0].Names["en"]; exists {
			city.RegionName = c
		}
		if record.Subdivisions[0].IsoCode != "" {
			city.RegionCode = record.Subdivisions[0].IsoCode
		}
	}
	if !math.IsNaN(record.Location.Latitude) {
		city.Latitude = record.Location.Latitude
	}
	if !math.IsNaN(record.Location.Longitude) {
		city.Longitude = record.Location.Longitude
	}
	// Metro code is US Only https://maxmind.github.io/GeoIP2-dotnet/doc/v2.7.1/html/P_MaxMind_GeoIP2_Model_Location_MetroCode.htm
	if record.Location.MetroCode > 0 && record.Country.IsoCode == "US" {
		city.MetroCode = record.Location.MetroCode
	}
	if record.Postal.Code != "" {
		city.PostalCode = record.Postal.Code
	}
	if record.Location.TimeZone != "" {
		city.Timezone = record.Location.TimeZone
	}

	return city, nil
}

func (g *geoip) ASN(ip net.IP) (ASN, error) {
	asn := ASN{}
	if g.asn == nil {
		return asn, nil
	}
	record, err := g.asn.ASN(ip)
	if err != nil {
		return asn, err
	}
	if record.AutonomousSystemNumber > 0 {
		asn.AutonomousSystemNumber = record.AutonomousSystemNumber
	}
	if record.AutonomousSystemOrganization != "" {
		asn.AutonomousSystemOrganization = record.AutonomousSystemOrganization
	}
	return asn, nil
}

func (g *geoip) ISP(ip net.IP) (ISP, error) {
	isp := ISP{}
	if g.isp == nil {
		return isp, nil
	}
	record, err := g.isp.ISP(ip)
	if err != nil {
		return isp, err
	}
	if record.ISP != "" {
		isp.ISP = record.ISP
	}
	if record.Organization != "" {
		isp.Organization = record.Organization
	}
	if record.AutonomousSystemNumber > 0 {
		isp.ASN = record.AutonomousSystemNumber
	}
	if record.AutonomousSystemOrganization != "" {
		isp.ORG = record.AutonomousSystemOrganization
	}
	return isp, nil
}

func (g *geoip) ConnectionType(ip net.IP) (ConnectionType, error) {
	connectiontype := ConnectionType{}
	if g.connectiontype == nil {
		return connectiontype, nil
	}
	record, err := g.connectiontype.ConnectionType(ip)
	if err != nil {
		return connectiontype, err
	}
	if record.ConnectionType != "" {
		connectiontype.ConnectionType = record.ConnectionType
	}
	return connectiontype, nil
}

func (g *geoip) Proxy(ip net.IP) (Proxy, error) {
	proxy := Proxy{}
	// For now, return empty proxy info since this is for MaxMind databases
	// IP2Proxy functionality will be implemented separately
	return proxy, nil
}

func (g *geoip) IsEmpty() bool {
	return g.country == nil && g.city == nil
}
