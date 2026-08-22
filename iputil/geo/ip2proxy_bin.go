package geo

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	ip2proxy "github.com/ip2location/ip2proxy-go/v4"
)

// IP2ProxyBinReader implements the Reader interface for IP2Proxy BIN databases
type IP2ProxyBinReader struct {
	db *ip2proxy.DB
}

// NewIP2ProxyBinReader creates a new IP2Proxy BIN reader
// cleanIP2ProxyString cleans proxy strings
func cleanIP2ProxyString(s string) string {
	if s == "-" || s == "" || strings.Contains(s, "This parameter is unavailable") || s == "Not_Supported" || s == "Please upgrade the data file" || strings.Contains(strings.ToLower(s), "ipv6 address missing in ipv4 bin") {
		return ""
	}
	return s
}

func NewIP2ProxyBinReader(binPath string) (*IP2ProxyBinReader, error) {
	if binPath == "" {
		return &IP2ProxyBinReader{}, nil
	}

	db, err := ip2proxy.OpenDB(binPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open IP2Proxy BIN file: %v", err)
	}

	return &IP2ProxyBinReader{db: db}, nil
}

// Country implements the Reader interface
func (r *IP2ProxyBinReader) Country(ip net.IP) (Country, error) {
	if r.db == nil {
		return Country{}, nil
	}

	result, err := r.db.GetAll(ip.String())
	if err != nil {
		return Country{}, err
	}

	return Country{
		Name: cleanIP2ProxyString(result.CountryLong),
		ISO:  cleanIP2ProxyString(result.CountryShort),
		IsEU: nil, // IP2Proxy doesn't provide EU information
	}, nil
}

// City implements the Reader interface
func (r *IP2ProxyBinReader) City(ip net.IP) (City, error) {
	if r.db == nil {
		return City{}, nil
	}

	result, err := r.db.GetAll(ip.String())
	if err != nil {
		return City{}, err
	}

	return City{
		Name:       cleanIP2ProxyString(result.City),
		RegionName: cleanIP2ProxyString(result.Region),
		RegionCode: "", // IP2Proxy doesn't provide region codes
		Latitude:   0,  // IP2Proxy doesn't provide coordinates in basic format
		Longitude:  0,  // IP2Proxy doesn't provide coordinates in basic format
		PostalCode: "", // IP2Proxy doesn't provide postal codes
		Timezone:   "", // IP2Proxy doesn't provide timezone
		MetroCode:  0,  // IP2Proxy doesn't provide metro codes
	}, nil
}

// ASN implements the Reader interface
func (r *IP2ProxyBinReader) ASN(ip net.IP) (ASN, error) {
	if r.db == nil {
		return ASN{}, nil
	}

	result, err := r.db.GetAll(ip.String())
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
func (r *IP2ProxyBinReader) ISP(ip net.IP) (ISP, error) {
	if r.db == nil {
		return ISP{}, nil
	}

	result, err := r.db.GetAll(ip.String())
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
func (r *IP2ProxyBinReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	if r.db == nil {
		return ConnectionType{}, nil
	}

	result, err := r.db.GetAll(ip.String())
	if err != nil {
		return ConnectionType{}, err
	}

	return ConnectionType{
		ConnectionType: cleanIP2ProxyString(result.UsageType),
	}, nil
}

// Proxy implements the Reader interface
func (r *IP2ProxyBinReader) Proxy(ip net.IP) (Proxy, error) {
	if r.db == nil {
		return Proxy{IsProxy: false}, nil
	}

	result, err := r.db.GetAll(ip.String())
	if err != nil {
		return Proxy{IsProxy: false}, err
	}

	isProxy := result.IsProxy != 0

	return Proxy{
		IsProxy:     isProxy,
		ProxyType:   cleanIP2ProxyString(result.ProxyType),
		Country:     cleanIP2ProxyString(result.CountryLong),
		CountryCode: cleanIP2ProxyString(result.CountryShort),
		Domain:      cleanIP2ProxyString(result.Domain),
		UsageType:   cleanIP2ProxyString(result.UsageType),
		LastSeen:    cleanIP2ProxyString(result.LastSeen),
		Threat:      cleanIP2ProxyString(result.Threat),
		Provider:    cleanIP2ProxyString(result.Provider),
		FraudScore:  "", // Convert if available
	}, nil
}

// Network returns network information for the given IP
func (r *IP2ProxyBinReader) Network(ip net.IP) (*net.IPNet, error) {
	// IP2Proxy databases don't typically provide network information
	return nil, nil
}


// IsEmpty implements the Reader interface
func (r *IP2ProxyBinReader) IsEmpty() bool {
	return r.db == nil
}

// Close closes the database connection
func (r *IP2ProxyBinReader) Close() {
	if r.db != nil {
		r.db.Close()
	}
}
