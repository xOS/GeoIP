package geo

import (
	"net"
	"strings"
)

// HybridReader combines MaxMind and qqwry.ipdb databases
// Uses qqwry.ipdb for China mainland IPs and MaxMind for others
type HybridReader struct {
	maxmind Reader
	qqwry   Reader
}

// NewHybridReader creates a new hybrid reader
func NewHybridReader(maxmindReader Reader, qqwryReader Reader) Reader {
	return &HybridReader{
		maxmind: maxmindReader,
		qqwry:   qqwryReader,
	}
}

// selectReader determines which reader to use based on IP location
func (h *HybridReader) selectReader(ip net.IP) Reader {
	// First check with MaxMind to determine if it's China mainland
	if h.maxmind != nil {
		country, err := h.maxmind.Country(ip)
		if err == nil && strings.ToUpper(country.ISO) == "CN" {
			// Use qqwry for China mainland
			if h.qqwry != nil {
				return h.qqwry
			}
		}
	}

	// Use MaxMind for non-China IPs or fallback
	if h.maxmind != nil {
		return h.maxmind
	}

	// Final fallback to qqwry if MaxMind is not available
	return h.qqwry
}

// Country returns country information, preferring qqwry for China IPs
func (h *HybridReader) Country(ip net.IP) (Country, error) {
	isChinaIP := h.isChinaMainlandIP(ip)

	if isChinaIP && h.qqwry != nil {
		// For China IPs, prefer qqwry (Chinese names)
		country, err := h.qqwry.Country(ip)
		if err == nil && country.ISO != "" {
			// If qqwry doesn't have IsEU info, get it from MaxMind
			if country.IsEU == nil && h.maxmind != nil {
				maxmindCountry, maxmindErr := h.maxmind.Country(ip)
				if maxmindErr == nil {
					country.IsEU = maxmindCountry.IsEU
				}
			}
			return country, nil
		}
	}

	// Use MaxMind for non-China IPs or fallback
	if h.maxmind != nil {
		return h.maxmind.Country(ip)
	}

	// Final fallback to qqwry
	if h.qqwry != nil {
		return h.qqwry.Country(ip)
	}

	return Country{}, nil
}

// City returns city information, combining data from both sources
func (h *HybridReader) City(ip net.IP) (City, error) {
	isChinaIP := h.isChinaMainlandIP(ip)

	var primaryCity, fallbackCity City
	var primaryErr, fallbackErr error

	if isChinaIP && h.qqwry != nil {
		// For China IPs, prefer qqwry for Chinese names
		primaryCity, primaryErr = h.qqwry.City(ip)
		if h.maxmind != nil {
			fallbackCity, fallbackErr = h.maxmind.City(ip)
		}
	} else if h.maxmind != nil {
		// For non-China IPs, prefer MaxMind
		primaryCity, primaryErr = h.maxmind.City(ip)
		if h.qqwry != nil {
			fallbackCity, fallbackErr = h.qqwry.City(ip)
		}
	}

	// Combine the best data from both sources
	result := City{}

	if primaryErr == nil {
		result = primaryCity
	}

	// Fill missing data from fallback, especially coordinates and timezone
	if fallbackErr == nil {
		if result.Name == "" && fallbackCity.Name != "" {
			result.Name = fallbackCity.Name
		}
		if result.RegionName == "" && fallbackCity.RegionName != "" {
			result.RegionName = fallbackCity.RegionName
		}
		if result.RegionCode == "" && fallbackCity.RegionCode != "" {
			result.RegionCode = fallbackCity.RegionCode
		}
		if result.Latitude == 0 && fallbackCity.Latitude != 0 {
			result.Latitude = fallbackCity.Latitude
		}
		if result.Longitude == 0 && fallbackCity.Longitude != 0 {
			result.Longitude = fallbackCity.Longitude
		}
		if result.Timezone == "" && fallbackCity.Timezone != "" {
			result.Timezone = fallbackCity.Timezone
		}
		if result.PostalCode == "" && fallbackCity.PostalCode != "" {
			result.PostalCode = fallbackCity.PostalCode
		}
		if result.MetroCode == 0 && fallbackCity.MetroCode != 0 {
			result.MetroCode = fallbackCity.MetroCode
		}
	}

	return result, nil
}

// ASN returns ASN information, preferring MaxMind for comprehensive data
func (h *HybridReader) ASN(ip net.IP) (ASN, error) {
	// For ASN, always try MaxMind first as it has comprehensive ASN data
	if h.maxmind != nil {
		asn, err := h.maxmind.ASN(ip)
		if err == nil && asn.AutonomousSystemNumber > 0 {
			return asn, nil
		}
	}

	// Fallback to selected reader
	reader := h.selectReader(ip)
	if reader != nil {
		return reader.ASN(ip)
	}
	return ASN{}, nil
}

// ISP returns ISP information, combining data from both sources
func (h *HybridReader) ISP(ip net.IP) (ISP, error) {
	// Check if it's China mainland to decide primary source
	isChinaIP := h.isChinaMainlandIP(ip)

	var primaryISP, fallbackISP ISP
	var primaryErr, fallbackErr error

	if isChinaIP && h.qqwry != nil {
		// For China IPs, prefer qqwry for ISP name but get ASN from MaxMind
		primaryISP, primaryErr = h.qqwry.ISP(ip)
		if h.maxmind != nil {
			fallbackISP, fallbackErr = h.maxmind.ISP(ip)
		}
	} else if h.maxmind != nil {
		// For non-China IPs, prefer MaxMind
		primaryISP, primaryErr = h.maxmind.ISP(ip)
		if h.qqwry != nil {
			fallbackISP, fallbackErr = h.qqwry.ISP(ip)
		}
	}

	// Combine the best data from both sources
	result := ISP{}

	if primaryErr == nil {
		result = primaryISP
	}

	// Fill missing data from fallback
	if fallbackErr == nil {
		if result.ISP == "" && fallbackISP.ISP != "" {
			result.ISP = fallbackISP.ISP
		}
		if result.Organization == "" && fallbackISP.Organization != "" {
			result.Organization = fallbackISP.Organization
		}
		if result.ASN == 0 && fallbackISP.ASN > 0 {
			result.ASN = fallbackISP.ASN
		}
		if result.ORG == "" && fallbackISP.ORG != "" {
			result.ORG = fallbackISP.ORG
		}
	}

	return result, nil
}

// ConnectionType returns connection type, preferring MaxMind data
func (h *HybridReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	// MaxMind has better ConnectionType data
	if h.maxmind != nil {
		connType, err := h.maxmind.ConnectionType(ip)
		if err == nil && connType.ConnectionType != "" {
			return connType, nil
		}
	}

	// Fallback to selected reader
	reader := h.selectReader(ip)
	if reader != nil {
		return reader.ConnectionType(ip)
	}
	return ConnectionType{}, nil
}

// Proxy returns proxy information using the appropriate reader
func (h *HybridReader) Proxy(ip net.IP) (Proxy, error) {
	reader := h.selectReader(ip)
	if reader != nil {
		return reader.Proxy(ip)
	}
	return Proxy{}, nil
}

// isChinaMainlandIP checks if an IP belongs to China mainland
func (h *HybridReader) isChinaMainlandIP(ip net.IP) bool {
	if h.maxmind != nil {
		country, err := h.maxmind.Country(ip)
		if err == nil && strings.ToUpper(country.ISO) == "CN" {
			return true
		}
	}
	return false
}

// IsEmpty returns true if both readers are empty
func (h *HybridReader) IsEmpty() bool {
	maxmindEmpty := h.maxmind == nil || h.maxmind.IsEmpty()
	qqwryEmpty := h.qqwry == nil || h.qqwry.IsEmpty()
	return maxmindEmpty && qqwryEmpty
}
