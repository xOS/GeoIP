package geo

import (
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
	ASN       uint
	ORG string
	ISP                          string
	Organization                 string
}

type ConnectionType struct {
	ConnectionType string
}

type geoip struct {
	country *geoip2.Reader
	city    *geoip2.Reader
	asn     *geoip2.Reader
	isp     *geoip2.Reader
	connectiontype     *geoip2.Reader
}

func Open(countryDB, cityDB string, asnDB string, ispDB string, connectiontypeDB string) (Reader, error) {
	var country, city, asn, isp, connectiontype *geoip2.Reader
	if countryDB != "" {
		r, err := geoip2.Open(countryDB)
		if err != nil {
			return nil, err
		}
		country = r
	}
	if cityDB != "" {
		r, err := geoip2.Open(cityDB)
		if err != nil {
			return nil, err
		}
		city = r
	}
	if asnDB != "" {
		r, err := geoip2.Open(asnDB)
		if err != nil {
			return nil, err
		}
		asn = r
	}
	if ispDB != "" {
		r, err := geoip2.Open(ispDB)
		if err != nil {
			return nil, err
		}
		isp = r
	}
	if connectiontypeDB != "" {
		r, err := geoip2.Open(connectiontypeDB)
		if err != nil {
			return nil, err
		}
		connectiontype = r
	}
	return &geoip{country: country, city: city, asn: asn, isp: isp, connectiontype: connectiontype}, nil
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

func (g *geoip) IsEmpty() bool {
	return g.country == nil && g.city == nil
}
