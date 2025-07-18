package geo

import (
	"net"
	"strings"
)

// HybridReader combines MaxMind, czdb v4/v6, and qqwry.ipdb databases
// Uses czdb for China mainland IPs (v4/v6), fallback to qqwry, MaxMind for others
// czdbV4: 纯真 czdb v4，czdbV6: 纯真 czdb v6
// qqwry: 旧版纯真，maxmind: MaxMind
type HybridReader struct {
	maxmind Reader
	czdbV4  Reader
	czdbV6  Reader
	qqwry   Reader
}

// Getter methods for http layer lang override
func (h *HybridReader) GetMaxmind() Reader { return h.maxmind }
func (h *HybridReader) GetCzdbV4() Reader  { return h.czdbV4 }
func (h *HybridReader) GetCzdbV6() Reader  { return h.czdbV6 }
func (h *HybridReader) GetQQWry() Reader   { return h.qqwry }

// NewHybridReader creates a new hybrid reader supporting czdb v4/v6
func NewHybridReader(maxmindReader, czdbV4Reader, czdbV6Reader, qqwryReader Reader) Reader {
	return &HybridReader{
		maxmind: maxmindReader,
		czdbV4:  czdbV4Reader,
		czdbV6:  czdbV6Reader,
		qqwry:   qqwryReader,
	}
}

// selectReader determines which reader to use based on IP location and version
func (h *HybridReader) selectReader(ip net.IP) Reader {
	if isChinaIPv4(ip) && h.czdbV4 != nil {
		return h.czdbV4
	}
	if isChinaIPv6(ip) && h.czdbV6 != nil {
		return h.czdbV6
	}
	if h.qqwry != nil && h.isChinaMainlandIP(ip) {
		return h.qqwry
	}
	if h.maxmind != nil {
		return h.maxmind
	}
	return nil
}

// mergeCountry 合并多个 Country 结构体，优先第一个非空字段
func mergeCountry(cs ...Country) Country {
	var out Country
	for _, c := range cs {
		if out.Name == "" && c.Name != "" {
			out.Name = c.Name
		}
		if out.ISO == "" && c.ISO != "" {
			out.ISO = c.ISO
		}
		if out.IsEU == nil && c.IsEU != nil {
			out.IsEU = c.IsEU
		}
	}
	return out
}

func mergeCity(cs ...City) City {
	var out City
	for _, c := range cs {
		if out.Name == "" && c.Name != "" {
			out.Name = c.Name
		}
		if out.RegionName == "" && c.RegionName != "" {
			out.RegionName = c.RegionName
		}
		if out.RegionCode == "" && c.RegionCode != "" {
			out.RegionCode = c.RegionCode
		}
		if out.Latitude == 0 && c.Latitude != 0 {
			out.Latitude = c.Latitude
		}
		if out.Longitude == 0 && c.Longitude != 0 {
			out.Longitude = c.Longitude
		}
		if out.Timezone == "" && c.Timezone != "" {
			out.Timezone = c.Timezone
		}
		if out.PostalCode == "" && c.PostalCode != "" {
			out.PostalCode = c.PostalCode
		}
		if out.MetroCode == 0 && c.MetroCode != 0 {
			out.MetroCode = c.MetroCode
		}
	}
	return out
}

func mergeISP(isps ...ISP) ISP {
	var out ISP
	for _, i := range isps {
		if out.ISP == "" && i.ISP != "" {
			out.ISP = i.ISP
		}
		if out.Organization == "" && i.Organization != "" {
			out.Organization = i.Organization
		}
		if out.ASN == 0 && i.ASN > 0 {
			out.ASN = i.ASN
		}
		if out.ORG == "" && i.ORG != "" {
			out.ORG = i.ORG
		}
	}
	return out
}

func mergeASN(asns ...ASN) ASN {
	var out ASN
	for _, a := range asns {
		if out.AutonomousSystemNumber == 0 && a.AutonomousSystemNumber > 0 {
			out.AutonomousSystemNumber = a.AutonomousSystemNumber
		}
		if out.AutonomousSystemOrganization == "" && a.AutonomousSystemOrganization != "" {
			out.AutonomousSystemOrganization = a.AutonomousSystemOrganization
		}
	}
	return out
}

// mergeCountryWithPriority: 主优先，次补全
func mergeCountryWithPriority(primary Country, supplements ...Country) Country {
	out := primary
	for _, c := range supplements {
		if out.Name == "" && c.Name != "" {
			out.Name = c.Name
		}
		if out.ISO == "" && c.ISO != "" {
			out.ISO = c.ISO
		}
		if out.IsEU == nil && c.IsEU != nil {
			out.IsEU = c.IsEU
		}
	}
	return out
}

func mergeCityWithPriority(primary City, supplements ...City) City {
	out := primary
	for _, c := range supplements {
		if out.Name == "" && c.Name != "" {
			out.Name = c.Name
		}
		if out.RegionName == "" && c.RegionName != "" {
			out.RegionName = c.RegionName
		}
		if out.RegionCode == "" && c.RegionCode != "" {
			out.RegionCode = c.RegionCode
		}
		if out.Latitude == 0 && c.Latitude != 0 {
			out.Latitude = c.Latitude
		}
		if out.Longitude == 0 && c.Longitude != 0 {
			out.Longitude = c.Longitude
		}
		if out.Timezone == "" && c.Timezone != "" {
			out.Timezone = c.Timezone
		}
		if out.PostalCode == "" && c.PostalCode != "" {
			out.PostalCode = c.PostalCode
		}
		if out.MetroCode == 0 && c.MetroCode != 0 {
			out.MetroCode = c.MetroCode
		}
	}
	return out
}

func mergeISPWithPriority(primary ISP, supplements ...ISP) ISP {
	out := primary
	for _, i := range supplements {
		if out.ISP == "" && i.ISP != "" {
			out.ISP = i.ISP
		}
		if out.Organization == "" && i.Organization != "" {
			out.Organization = i.Organization
		}
		if out.ASN == 0 && i.ASN > 0 {
			out.ASN = i.ASN
		}
		if out.ORG == "" && i.ORG != "" {
			out.ORG = i.ORG
		}
	}
	return out
}

func mergeASNWithPriority(primary ASN, supplements ...ASN) ASN {
	out := primary
	for _, a := range supplements {
		if out.AutonomousSystemNumber == 0 && a.AutonomousSystemNumber > 0 {
			out.AutonomousSystemNumber = a.AutonomousSystemNumber
		}
		if out.AutonomousSystemOrganization == "" && a.AutonomousSystemOrganization != "" {
			out.AutonomousSystemOrganization = a.AutonomousSystemOrganization
		}
	}
	return out
}

// Country returns merged country information from all sources
func (h *HybridReader) Country(ip net.IP) (Country, error) {
	if h.maxmind == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return Country{}, nil
	}
	if h.maxmind != nil && !h.isChinaMainlandIP(ip) {
		// 非中国大陆 IP 以 MaxMind 为主
		return h.maxmind.Country(ip)
	}
	// 中国大陆 IP 优先 czdb/qqwry，缺失字段用其它库补全
	var main Country
	var c1, c2, c3 Country
	if isChinaIPv4(ip) && h.czdbV4 != nil {
		main, _ = h.czdbV4.Country(ip)
	} else if isChinaIPv6(ip) && h.czdbV6 != nil {
		main, _ = h.czdbV6.Country(ip)
	} else if h.qqwry != nil {
		main, _ = h.qqwry.Country(ip)
	}
	if h.maxmind != nil {
		c1, _ = h.maxmind.Country(ip)
	}
	if h.qqwry != nil {
		c2, _ = h.qqwry.Country(ip)
	}
	return mergeCountryWithPriority(main, c1, c2, c3), nil
}

// City returns merged city information from all sources
func (h *HybridReader) City(ip net.IP) (City, error) {
	if h.maxmind == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return City{}, nil
	}
	if h.maxmind != nil && !h.isChinaMainlandIP(ip) {
		return h.maxmind.City(ip)
	}
	// 中国大陆 IP 优先 czdb/qqwry，缺失字段用其它库补全
	var main City
	var c1, c2, c3 City
	if isChinaIPv4(ip) && h.czdbV4 != nil {
		main, _ = h.czdbV4.City(ip)
	} else if isChinaIPv6(ip) && h.czdbV6 != nil {
		main, _ = h.czdbV6.City(ip)
	} else if h.qqwry != nil {
		main, _ = h.qqwry.City(ip)
	}
	if h.maxmind != nil {
		c1, _ = h.maxmind.City(ip)
	}
	if h.qqwry != nil {
		c2, _ = h.qqwry.City(ip)
	}
	return mergeCityWithPriority(main, c1, c2, c3), nil
}

// ASN returns merged ASN information from all sources
func (h *HybridReader) ASN(ip net.IP) (ASN, error) {
	if h.maxmind == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return ASN{}, nil
	}
	if h.maxmind != nil && !h.isChinaMainlandIP(ip) {
		main, _ := h.maxmind.ASN(ip)
		a1, _ := h.czdbV4.ASN(ip)
		a2, _ := h.czdbV6.ASN(ip)
		a3, _ := h.qqwry.ASN(ip)
		return mergeASNWithPriority(main, a1, a2, a3), nil
	}
	var main ASN
	var a1, a2, a3 ASN
	if isChinaIPv4(ip) && h.czdbV4 != nil {
		main, _ = h.czdbV4.ASN(ip)
	} else if isChinaIPv6(ip) && h.czdbV6 != nil {
		main, _ = h.czdbV6.ASN(ip)
	} else if h.qqwry != nil {
		main, _ = h.qqwry.ASN(ip)
	}
	if h.maxmind != nil {
		a1, _ = h.maxmind.ASN(ip)
	}
	if h.qqwry != nil {
		a2, _ = h.qqwry.ASN(ip)
	}
	return mergeASNWithPriority(main, a1, a2, a3), nil
}

// ISP returns merged ISP information from all sources
func (h *HybridReader) ISP(ip net.IP) (ISP, error) {
	if h.maxmind == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return ISP{}, nil
	}
	if h.maxmind != nil && !h.isChinaMainlandIP(ip) {
		return h.maxmind.ISP(ip)
	}
	// 中国大陆 IP 优先 czdb/qqwry，缺失字段用其它库补全
	var main ISP
	var i1, i2, i3 ISP
	if isChinaIPv4(ip) && h.czdbV4 != nil {
		main, _ = h.czdbV4.ISP(ip)
	} else if isChinaIPv6(ip) && h.czdbV6 != nil {
		main, _ = h.czdbV6.ISP(ip)
	} else if h.qqwry != nil {
		main, _ = h.qqwry.ISP(ip)
	}
	if h.maxmind != nil {
		i1, _ = h.maxmind.ISP(ip)
	}
	if h.qqwry != nil {
		i2, _ = h.qqwry.ISP(ip)
	}
	return mergeISPWithPriority(main, i1, i2, i3), nil
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

// isChinaIPv4/IPv6 判断 IP 是否为中国大陆（仅根据 czdb 是否可查）
func isChinaIPv4(ip net.IP) bool {
	return ip.To4() != nil
}
func isChinaIPv6(ip net.IP) bool {
	return ip.To16() != nil && ip.To4() == nil
}

// IsEmpty returns true if both readers are empty
func (h *HybridReader) IsEmpty() bool {
	maxmindEmpty := h.maxmind == nil || h.maxmind.IsEmpty()
	qqwryEmpty := h.qqwry == nil || h.qqwry.IsEmpty()
	return maxmindEmpty && qqwryEmpty
}
