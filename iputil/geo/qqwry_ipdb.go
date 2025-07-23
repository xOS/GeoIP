package geo

import (
	"net"
	"strconv"
	"strings"

	"github.com/ipipdotnet/ipdb-go"
)

// QQWryIPDBReader implements Reader interface for qqwry.ipdb format
type QQWryIPDBReader struct {
	db *ipdb.City
}

// NewQQWryIPDBReader creates a new qqwry.ipdb reader
func NewQQWryIPDBReader(path string) (Reader, error) {
	db, err := ipdb.NewCity(path)
	if err != nil {
		return nil, err
	}

	return &QQWryIPDBReader{db: db}, nil
}

// Country returns country information from qqwry.ipdb
func (q *QQWryIPDBReader) Country(ip net.IP) (Country, error) {
	info, err := q.db.FindInfo(ip.String(), "CN")
	if err != nil {
		return Country{}, err
	}

	country := Country{}
	if len(info.CountryName) > 0 {
		country.Name = info.CountryName
	}
	if len(info.CountryCode) > 0 {
		country.ISO = info.CountryCode
	}

	return country, nil
}

// City returns city information from qqwry.ipdb
func (q *QQWryIPDBReader) City(ip net.IP) (City, error) {
	info, err := q.db.FindInfo(ip.String(), "CN")
	if err != nil {
		return City{}, err
	}

	city := City{}
	if len(info.CityName) > 0 {
		city.Name = info.CityName
	}
	if len(info.RegionName) > 0 {
		city.RegionName = info.RegionName
	}

	// Parse latitude and longitude if available
	if len(info.Latitude) > 0 {
		if lat, err := strconv.ParseFloat(info.Latitude, 64); err == nil {
			city.Latitude = lat
		}
	}
	if len(info.Longitude) > 0 {
		if lng, err := strconv.ParseFloat(info.Longitude, 64); err == nil {
			city.Longitude = lng
		}
	}

	// Set timezone if available
	if len(info.Timezone) > 0 {
		city.Timezone = info.Timezone
	}

	return city, nil
}

// ASN returns ASN information (limited for qqwry.ipdb)
func (q *QQWryIPDBReader) ASN(ip net.IP) (ASN, error) {
	// qqwry.ipdb typically doesn't have ASN information
	return ASN{}, nil
}

// ISP returns ISP information from qqwry.ipdb
func (q *QQWryIPDBReader) ISP(ip net.IP) (ISP, error) {
	info, err := q.db.FindInfo(ip.String(), "CN")
	if err != nil {
		return ISP{}, err
	}

	isp := ISP{}
	if len(info.IspDomain) > 0 {
		isp.ISP = info.IspDomain
		isp.Organization = info.IspDomain
	}

	// Try to parse ASN if available
	if len(info.ASN) > 0 {
		if asn, err := strconv.ParseUint(strings.TrimPrefix(info.ASN, "AS"), 10, 32); err == nil {
			isp.ASN = uint(asn)
		}
	}

	return isp, nil
}

// ConnectionType returns connection type (not typically available in qqwry.ipdb)
func (q *QQWryIPDBReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	return ConnectionType{}, nil
}

// Proxy returns proxy information (not available in qqwry.ipdb)
func (q *QQWryIPDBReader) Proxy(ip net.IP) (Proxy, error) {
	return Proxy{}, nil
}

// Network returns network information for the given IP
func (q *QQWryIPDBReader) Network(ip net.IP) (*net.IPNet, error) {
	// QQWry databases don't typically provide network information
	return nil, nil
}


// IsEmpty returns whether the database is empty
func (q *QQWryIPDBReader) IsEmpty() bool {
	return q.db == nil
}

// IsChinaMainlandIP checks if an IP belongs to China mainland
func IsChinaMainlandIP(ip net.IP, reader Reader) bool {
	country, err := reader.Country(ip)
	if err != nil {
		return false
	}
	return strings.ToUpper(country.ISO) == "CN"
}
