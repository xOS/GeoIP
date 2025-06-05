package geo

import (
	"net"
)

// CombinedReader combines MaxMind GeoIP2 databases with IP2Proxy databases
type CombinedReader struct {
	geoip    Reader
	ip2proxy Reader
}

// NewCombinedReader creates a new combined reader
func NewCombinedReader(geoipReader Reader, ip2proxyReader Reader) Reader {
	return &CombinedReader{
		geoip:    geoipReader,
		ip2proxy: ip2proxyReader,
	}
}

// Country returns country information, preferring IP2Proxy data if available
func (c *CombinedReader) Country(ip net.IP) (Country, error) {
	// Try IP2Proxy first (since it can replace MaxMind completely)
	if c.ip2proxy != nil {
		country, err := c.ip2proxy.Country(ip)
		if err == nil && isValidCountry(country) {
			return country, nil
		}
	}

	// Fall back to GeoIP2
	if c.geoip != nil {
		return c.geoip.Country(ip)
	}

	return Country{}, nil
}

// City returns city information, preferring IP2Proxy data if available
func (c *CombinedReader) City(ip net.IP) (City, error) {
	// Try IP2Proxy first
	if c.ip2proxy != nil {
		city, err := c.ip2proxy.City(ip)
		if err == nil && isValidCity(city) {
			return city, nil
		}
	}

	// Fall back to GeoIP2
	if c.geoip != nil {
		return c.geoip.City(ip)
	}

	return City{}, nil
}

// ASN returns ASN information, preferring IP2Proxy data if available
func (c *CombinedReader) ASN(ip net.IP) (ASN, error) {
	// Try IP2Proxy first
	if c.ip2proxy != nil {
		asn, err := c.ip2proxy.ASN(ip)
		if err == nil && isValidASN(asn) {
			return asn, nil
		}
	}

	// Fall back to GeoIP2
	if c.geoip != nil {
		return c.geoip.ASN(ip)
	}

	return ASN{}, nil
}

// ISP returns ISP information, preferring IP2Proxy data if available
func (c *CombinedReader) ISP(ip net.IP) (ISP, error) {
	// Try IP2Proxy first
	if c.ip2proxy != nil {
		isp, err := c.ip2proxy.ISP(ip)
		if err == nil && isValidISP(isp) {
			return isp, nil
		}
	}

	// Fall back to GeoIP2
	if c.geoip != nil {
		return c.geoip.ISP(ip)
	}

	return ISP{}, nil
}

// ConnectionType returns connection type, preferring IP2Proxy data if available
func (c *CombinedReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	// Try IP2Proxy first
	if c.ip2proxy != nil {
		connType, err := c.ip2proxy.ConnectionType(ip)
		if err == nil && isValidConnectionType(connType) {
			return connType, nil
		}
	}

	// Fall back to GeoIP2
	if c.geoip != nil {
		return c.geoip.ConnectionType(ip)
	}

	return ConnectionType{}, nil
}

// Proxy returns proxy information from IP2Proxy
func (c *CombinedReader) Proxy(ip net.IP) (Proxy, error) {
	if c.ip2proxy != nil {
		return c.ip2proxy.Proxy(ip)
	}
	return Proxy{}, nil
}

// IsEmpty returns true if both readers are empty
func (c *CombinedReader) IsEmpty() bool {
	geoipEmpty := c.geoip == nil || c.geoip.IsEmpty()
	ip2proxyEmpty := c.ip2proxy == nil || c.ip2proxy.IsEmpty()
	return geoipEmpty && ip2proxyEmpty
}

// Helper functions to validate if data is meaningful
func isValidCountry(country Country) bool {
	return country.Name != "" && country.Name != "-" &&
		country.ISO != "" && country.ISO != "-"
}

func isValidCity(city City) bool {
	return city.Name != "" && city.Name != "-"
}

func isValidASN(asn ASN) bool {
	return asn.AutonomousSystemNumber > 0
}

func isValidISP(isp ISP) bool {
	return (isp.ISP != "" && isp.ISP != "-") ||
		(isp.Organization != "" && isp.Organization != "-")
}

func isValidConnectionType(connType ConnectionType) bool {
	return connType.ConnectionType != "" && connType.ConnectionType != "-"
}
