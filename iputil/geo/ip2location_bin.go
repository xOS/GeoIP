package geo

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	ip2location "github.com/ip2location/ip2location-go/v9"
)

// IP2LocationBinReader implements the Reader interface for IP2Location BIN databases
type IP2LocationBinReader struct {
	db *ip2location.DB
}

// NewIP2LocationBinReader creates a new IP2Location BIN reader
func NewIP2LocationBinReader(binPath string) (*IP2LocationBinReader, error) {
	if binPath == "" {
		return &IP2LocationBinReader{}, nil
	}

	db, err := ip2location.OpenDB(binPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open IP2Location BIN file: %v", err)
	}

	return &IP2LocationBinReader{db: db}, nil
}

// Country implements the Reader interface
func (r *IP2LocationBinReader) Country(ip net.IP) (Country, error) {
	if r.db == nil {
		return Country{}, nil
	}

	result, err := r.db.Get_all(ip.String())
	if err != nil {
		return Country{}, err
	}

	var isEU *bool
	if result.Country_short != "-" && result.Country_short != "" {
		// Simple EU check based on country code
		euCountries := map[string]bool{
			"AT": true, "BE": true, "BG": true, "HR": true, "CY": true, "CZ": true,
			"DK": true, "EE": true, "FI": true, "FR": true, "DE": true, "GR": true,
			"HU": true, "IE": true, "IT": true, "LV": true, "LT": true, "LU": true,
			"MT": true, "NL": true, "PL": true, "PT": true, "RO": true, "SK": true,
			"SI": true, "ES": true, "SE": true,
		}
		eu := euCountries[result.Country_short]
		isEU = &eu
	}

	return Country{
		Name: result.Country_long,
		ISO:  result.Country_short,
		IsEU: isEU,
	}, nil
}

// City implements the Reader interface
func (r *IP2LocationBinReader) City(ip net.IP) (City, error) {
	if r.db == nil {
		return City{}, nil
	}

	result, err := r.db.Get_all(ip.String())
	if err != nil {
		return City{}, err
	}

	var latitude, longitude float64
	if result.Latitude != 0 {
		latitude = float64(result.Latitude)
	}
	if result.Longitude != 0 {
		longitude = float64(result.Longitude)
	}

	var metroCode uint
	if result.Areacode != "-" && result.Areacode != "" {
		if code, err := strconv.ParseUint(result.Areacode, 10, 32); err == nil {
			metroCode = uint(code)
		}
	}

	return City{
		Name:       result.City,
		Latitude:   latitude,
		Longitude:  longitude,
		PostalCode: result.Zipcode,
		Timezone:   result.Timezone,
		MetroCode:  metroCode,
		RegionName: result.Region,
		RegionCode: "", // IP2Location doesn't provide region codes in standard format
	}, nil
}

// ASN implements the Reader interface
func (r *IP2LocationBinReader) ASN(ip net.IP) (ASN, error) {
	if r.db == nil {
		return ASN{}, nil
	}

	result, err := r.db.Get_all(ip.String())
	if err != nil {
		return ASN{}, err
	}

	var asnNumber uint
	if result.Asn != "-" && result.Asn != "" {
		// Remove "AS" prefix if present
		asnStr := strings.TrimPrefix(result.Asn, "AS")
		if parsed, err := strconv.ParseUint(asnStr, 10, 32); err == nil {
			asnNumber = uint(parsed)
		}
	}

	return ASN{
		AutonomousSystemNumber:       asnNumber,
		AutonomousSystemOrganization: result.As,
	}, nil
}

// ISP implements the Reader interface
func (r *IP2LocationBinReader) ISP(ip net.IP) (ISP, error) {
	if r.db == nil {
		return ISP{}, nil
	}

	result, err := r.db.Get_all(ip.String())
	if err != nil {
		return ISP{}, err
	}

	var asnNumber uint
	if result.Asn != "-" && result.Asn != "" {
		asnStr := strings.TrimPrefix(result.Asn, "AS")
		if parsed, err := strconv.ParseUint(asnStr, 10, 32); err == nil {
			asnNumber = uint(parsed)
		}
	}

	return ISP{
		ASN:          asnNumber,
		ORG:          result.As,
		ISP:          result.Isp,
		Organization: result.As,
	}, nil
}

// ConnectionType implements the Reader interface
func (r *IP2LocationBinReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	if r.db == nil {
		return ConnectionType{}, nil
	}

	result, err := r.db.Get_all(ip.String())
	if err != nil {
		return ConnectionType{}, err
	}

	return ConnectionType{
		ConnectionType: result.Netspeed,
	}, nil
}

// Proxy implements the Reader interface
func (r *IP2LocationBinReader) Proxy(ip net.IP) (Proxy, error) {
	if r.db == nil {
		return Proxy{IsProxy: false}, nil
	}

	result, err := r.db.Get_all(ip.String())
	if err != nil {
		return Proxy{IsProxy: false}, err
	}

	// IP2Location BIN doesn't have direct proxy detection
	// We can infer from usage type or other fields
	isProxy := false
	proxyType := ""

	// Check usage type for proxy indicators
	if result.Usagetype != "-" && result.Usagetype != "" {
		usageType := strings.ToUpper(result.Usagetype)
		switch usageType {
		case "DCH", "DCH/SES", "DCH/ISP":
			isProxy = true
			proxyType = "DCH"
		case "ISP/MOB":
			// Mobile ISP, could be proxy
			proxyType = "ISP"
		}
	}

	return Proxy{
		IsProxy:     isProxy,
		ProxyType:   proxyType,
		Country:     result.Country_long,
		CountryCode: result.Country_short,
		Domain:      result.Domain,
		UsageType:   result.Usagetype,
		LastSeen:    "",
		Threat:      "",
		Provider:    "",
		FraudScore:  "",
	}, nil
}

// IsEmpty implements the Reader interface
func (r *IP2LocationBinReader) IsEmpty() bool {
	return r.db == nil
}

// Close closes the database connection
func (r *IP2LocationBinReader) Close() {
	if r.db != nil {
		r.db.Close()
	}
}
