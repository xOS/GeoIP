package geo

import (
	"fmt"
	"net"

	"github.com/oschwald/maxminddb-golang"
)

// GeoCNMMDBReader implements the Reader interface for GeoCN.mmdb database
type GeoCNMMDBReader struct {
	reader *maxminddb.Reader
}

// GeoCNRecord represents the structure of records in GeoCN.mmdb
type GeoCNRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Province  string `maxminddb:"province"`
	City      string `maxminddb:"city"`
	Districts string `maxminddb:"districts"`
	ISP       string `maxminddb:"isp"`
	Net       string `maxminddb:"net"`
	Location  struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		TimeZone  string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
	Network string `maxminddb:"network"`
}

// NewGeoCNMMDBReader creates a new GeoCN MMDB reader
func NewGeoCNMMDBReader(path string) (*GeoCNMMDBReader, error) {
	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}

	return &GeoCNMMDBReader{
		reader: reader,
	}, nil
}

// Country returns country information for the given IP
func (g *GeoCNMMDBReader) Country(ip net.IP) (Country, error) {
	// Check if reader is nil
	if g.reader == nil {
		return Country{}, fmt.Errorf("reader is nil")
	}

	var record GeoCNRecord
	err := g.reader.Lookup(ip, &record)
	if err != nil {
		return Country{}, err
	}

	country := Country{}
	if record.Country.ISOCode != "" {
		country.ISO = record.Country.ISOCode
	}

	// Try to get country name in Chinese first, then English
	if len(record.Country.Names) > 0 {
		if name, exists := record.Country.Names["zh-CN"]; exists {
			country.Name = name
		} else if name, exists := record.Country.Names["en"]; exists {
			country.Name = name
		}
	}

	return country, nil
}

// City returns city information for the given IP
func (g *GeoCNMMDBReader) City(ip net.IP) (City, error) {
	// Check if reader is nil
	if g.reader == nil {
		return City{}, fmt.Errorf("reader is nil")
	}

	var record GeoCNRecord
	err := g.reader.Lookup(ip, &record)
	if err != nil {
		return City{}, err
	}

	city := City{}
	city.Name = record.City
	// 根据要求，province对应region
	city.RegionName = record.Province
	// 根据要求，districts对应district
	city.District = record.Districts

	// Set coordinates if available
	if record.Location.Latitude != 0 || record.Location.Longitude != 0 {
		city.Latitude = record.Location.Latitude
		city.Longitude = record.Location.Longitude
	}

	// Set timezone if available
	if record.Location.TimeZone != "" {
		city.Timezone = record.Location.TimeZone
	}

	return city, nil
}

// ASN returns ASN information for the given IP
func (g *GeoCNMMDBReader) ASN(ip net.IP) (ASN, error) {
	// GeoCN.mmdb doesn't typically contain ASN information
	// This would need to be obtained from a separate database
	return ASN{}, nil
}

// ISP returns ISP information for the given IP
func (g *GeoCNMMDBReader) ISP(ip net.IP) (ISP, error) {
	// Check if reader is nil
	if g.reader == nil {
		return ISP{}, fmt.Errorf("reader is nil")
	}

	var record GeoCNRecord
	err := g.reader.Lookup(ip, &record)
	if err != nil {
		return ISP{}, err
	}

	isp := ISP{}
	isp.ISP = record.ISP

	// If we have ISP info, also set the organization
	if record.ISP != "" {
		isp.Organization = record.ISP
	}

	return isp, nil
}

// ConnectionType returns connection type information for the given IP
func (g *GeoCNMMDBReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	// Check if reader is nil
	if g.reader == nil {
		return ConnectionType{}, fmt.Errorf("reader is nil")
	}

	var record GeoCNRecord
	err := g.reader.Lookup(ip, &record)
	if err != nil {
		return ConnectionType{}, err
	}

	connType := ConnectionType{}
	// 根据要求，net对应ConnectionType
	connType.ConnectionType = record.Net

	return connType, nil
}

// Proxy returns proxy information for the given IP
func (g *GeoCNMMDBReader) Proxy(ip net.IP) (Proxy, error) {
	// GeoCN.mmdb doesn't typically contain proxy information
	// Check if reader is nil
	if g.reader == nil {
		return Proxy{IsProxy: false}, nil
	}

	// Even if reader is not nil, GeoCN.mmdb doesn't typically contain proxy information
	return Proxy{IsProxy: false}, nil
}

// Network returns the network (CIDR) containing the IP if available.
// This is a best-effort helper for tests; it returns nil when not available.
func (g *GeoCNMMDBReader) Network(ip net.IP) (*net.IPNet, error) {
	if g.reader == nil {
		return nil, nil
	}
	var record GeoCNRecord
	if err := g.reader.Lookup(ip, &record); err != nil {
		return nil, err
	}
	if record.Network == "" {
		return nil, nil
	}
	_, n, err := net.ParseCIDR(record.Network)
	if err != nil {
		return nil, nil
	}
	return n, nil
}

// IsEmpty returns true if the reader is empty
func (g *GeoCNMMDBReader) IsEmpty() bool {
	return g.reader == nil
}

// Close closes the underlying database reader
func (g *GeoCNMMDBReader) Close() error {
	if g.reader != nil {
		return g.reader.Close()
	}
	return nil
}
