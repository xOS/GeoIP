package geo

import (
	"fmt"
	"math"
	"net"

	"github.com/oschwald/maxminddb-golang"
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
	Name       string `json:"city,omitempty"`
	RegionName string `json:"region,omitempty"`
	RegionCode string `json:"region_code,omitempty"`
	District   string `json:"district,omitempty"`
	Latitude   float64
	Longitude  float64
	PostalCode string
	Timezone   string
	MetroCode  uint
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
	country        *maxminddb.Reader
	city           *maxminddb.Reader
	asn            *maxminddb.Reader
	isp            *maxminddb.Reader
	connectiontype *maxminddb.Reader
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
	var country, city, asn, isp, connectiontype *maxminddb.Reader
	
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
			r, err := maxminddb.Open(*dbPath)
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

		case DatabaseTypeIP2LocationBIN, DatabaseTypeIP2ProxyBIN:
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
		hybridReader := NewHybridReader(maxmindReader, nil, nil, nil, qqwryReader)

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

func OpenWithHybridMode(countryDB, cityDB string, asnDB string, ispDB string, connectiontypeDB string, ip2proxyDB string, qqwryDB string, geocnMMDB string, czdbV4 string, czdbKey string, czdbV6 string) (Reader, error) {
	var country, city, asn, isp, connectiontype interface{}
	var maxmindReader Reader
	var qqwryReader Reader
	var geocnReader Reader
	var czdbV4Reader Reader
	var czdbV6Reader Reader
	var ip2proxyReader Reader

	if countryDB != "" {
		reader, err := openDatabaseWithDebug(countryDB, func(p string) (interface{}, error) { return maxminddb.Open(p) }, "MaxMind Country DB")
		if err == nil {
			country = reader
		}
	}
	if cityDB != "" {
		reader, err := openDatabaseWithDebug(cityDB, func(p string) (interface{}, error) { return maxminddb.Open(p) }, "MaxMind City DB")
		if err == nil {
			city = reader
		}
	}
	if asnDB != "" {
		reader, err := openDatabaseWithDebug(asnDB, func(p string) (interface{}, error) { return maxminddb.Open(p) }, "MaxMind ASN DB")
		if err == nil {
			asn = reader
		}
	}
	if ispDB != "" {
		reader, err := openDatabaseWithDebug(ispDB, func(p string) (interface{}, error) { return maxminddb.Open(p) }, "MaxMind ISP DB")
		if err == nil {
			isp = reader
		}
	}
	if connectiontypeDB != "" {
		reader, err := openDatabaseWithDebug(connectiontypeDB, func(p string) (interface{}, error) { return maxminddb.Open(p) }, "MaxMind ConnectionType DB")
		if err == nil {
			connectiontype = reader
		}
	}
	if country != nil || city != nil || asn != nil || isp != nil || connectiontype != nil {
		maxmindReader = &geoip{
			country:        country.(*maxminddb.Reader),
			city:           city.(*maxminddb.Reader),
			asn:            asn.(*maxminddb.Reader),
			isp:            isp.(*maxminddb.Reader),
			connectiontype: connectiontype.(*maxminddb.Reader),
		}
	}
	if qqwryDB != "" {
		reader, err := openDatabaseWithDebug(qqwryDB, func(p string) (interface{}, error) { return CreateReader(p) }, "QQWry DB")
		if err == nil {
			qqwryReader = reader.(Reader)
		}
	}
	if czdbV4 != "" && czdbKey != "" {
		reader, err := openDatabaseWithDebug(czdbV4, func(p string) (interface{}, error) { return CreateReader(p, czdbKey) }, "CZDB v4 DB")
		if err == nil {
			czdbV4Reader = reader.(Reader)
		}
	}
	if czdbV6 != "" && czdbKey != "" {
		reader, err := openDatabaseWithDebug(czdbV6, func(p string) (interface{}, error) { return CreateReader(p, czdbKey) }, "CZDB v6 DB")
		if err == nil {
			czdbV6Reader = reader.(Reader)
		}
	}
	if geocnMMDB != "" {
			reader, err := openDatabaseWithDebug(geocnMMDB, func(p string) (interface{}, error) { return CreateReader(p) }, "GeoCN MMDB DB")
			if err == nil {
				geocnReader = reader.(Reader)
			}
		}
		if ip2proxyDB != "" {
			reader, err := openDatabaseWithDebug(ip2proxyDB, func(p string) (interface{}, error) { return CreateReader(p) }, "IP2Proxy BIN DB")
			if err == nil {
				ip2proxyReader = reader.(Reader)
			}
		}

	hybrid := NewHybridReader(maxmindReader, geocnReader, czdbV4Reader, czdbV6Reader, qqwryReader)
		if ip2proxyReader != nil {
			return NewCombinedReader(hybrid, ip2proxyReader), nil
		}
		return hybrid, nil
}

// maxminddbOpen 用于加载 MaxMind mmdb 文件
func maxminddbOpen(path string) (*maxminddb.Reader, error) {
	return maxminddb.Open(path)
}

func (g *geoip) Country(ip net.IP) (Country, error) {
	country := Country{}
	if g.country == nil {
		return country, nil
	}
	
	// 定义用于解析 MaxMind Country 数据库的结构体
	var record struct {
		Country struct {
			IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
			ISOCode           string            `maxminddb:"iso_code"`
			Names             map[string]string `maxminddb:"names"`
		} `maxminddb:"country"`
		RegisteredCountry struct {
			IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
			ISOCode           string            `maxminddb:"iso_code"`
			Names             map[string]string `maxminddb:"names"`
		} `maxminddb:"registered_country"`
	}
	
	err := g.country.Lookup(ip, &record)
	if err != nil {
		return country, err
	}
	
	if c, exists := record.Country.Names["en"]; exists {
		country.Name = c
	}
	if c, exists := record.RegisteredCountry.Names["en"]; exists && country.Name == "" {
		country.Name = c
	}
	if record.Country.ISOCode != "" {
		country.ISO = record.Country.ISOCode
	}
	if record.RegisteredCountry.ISOCode != "" && country.ISO == "" {
		country.ISO = record.RegisteredCountry.ISOCode
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
	
	// 定义用于解析 MaxMind City 数据库的结构体
	var record struct {
		City struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"city"`
		Subdivisions []struct {
			IsoCode string            `maxminddb:"iso_code"`
			Names   map[string]string `maxminddb:"names"`
		} `maxminddb:"subdivisions"`
		Location struct {
			Latitude  float64 `maxminddb:"latitude"`
			Longitude float64 `maxminddb:"longitude"`
			MetroCode uint    `maxminddb:"metro_code"`
			TimeZone  string  `maxminddb:"time_zone"`
		} `maxminddb:"location"`
		Postal struct {
			Code string `maxminddb:"code"`
		} `maxminddb:"postal"`
		Country struct {
			IsoCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	
	err := g.city.Lookup(ip, &record)
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
	
	// 定义用于解析 MaxMind ASN 数据库的结构体
	var record struct {
		AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
		AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
	}
	
	err := g.asn.Lookup(ip, &record)
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
	
	// 定义用于解析 MaxMind ISP 数据库的结构体
	var record struct {
		ISP                          string `maxminddb:"isp"`
		Organization                 string `maxminddb:"organization"`
		AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
		AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
	}
	
	err := g.isp.Lookup(ip, &record)
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
	
	// 定义用于解析 MaxMind ConnectionType 数据库的结构体
	var record struct {
		ConnectionType string `maxminddb:"connection_type"`
	}
	
	err := g.connectiontype.Lookup(ip, &record)
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
