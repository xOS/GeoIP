package geo

import (
	"fmt"
	"net"
	"strings"
)

// HybridReader combines MaxMind, czdb v4/v6, and qqwry.ipdb databases
// Uses czdb for China mainland IPs (v4/v6), fallback to qqwry, MaxMind for others
// czdbV4: 纯真 czdb v4，czdbV6: 纯真 czdb v6
// qqwry: 旧版纯真，maxmind: MaxMind
type HybridReader struct {
	maxmind Reader
	geocn   Reader
	czdbV4  Reader
	czdbV6  Reader
	qqwry   Reader
}

// Getter methods for http layer lang override
func (h *HybridReader) GetMaxmind() Reader { return h.maxmind }
func (h *HybridReader) GetGeoCN() Reader   { return h.geocn }
func (h *HybridReader) GetCzdbV4() Reader  { return h.czdbV4 }
func (h *HybridReader) GetCzdbV6() Reader  { return h.czdbV6 }
func (h *HybridReader) GetQQWry() Reader   { return h.qqwry }

// NewHybridReader creates a new hybrid reader supporting czdb v4/v6
func NewHybridReader(maxmindReader, geocnReader, czdbV4Reader, czdbV6Reader, qqwryReader Reader) Reader {
	return &HybridReader{
		maxmind: maxmindReader,
		geocn:   geocnReader,
		czdbV4:  czdbV4Reader,
		czdbV6:  czdbV6Reader,
		qqwry:   qqwryReader,
	}
}

// selectReader determines which reader to use based on IP location and version
func (h *HybridReader) selectReader(ip net.IP) Reader {
	// GeoCN.mmdb has the highest priority for China mainland IPs
	if h.isChinaMainlandIP(ip) && h.geocn != nil {
		return h.geocn
	}
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
		if out.District == "" && c.District != "" {
			out.District = c.District
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
		if out.District == "" && c.District != "" {
			out.District = c.District
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
	if h.maxmind == nil && h.geocn == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return Country{}, nil
	}
	
	// 对于中国大陆IP，优先使用GeoCN.mmdb
	if h.isChinaMainlandIP(ip) {
		var main Country
		var supplements []Country
		
		// GeoCN.mmdb has the highest priority for China mainland IPs
		if h.geocn != nil {
			main, _ = h.geocn.Country(ip)
		}
		
		// 只有当GeoCN.mmdb没有数据或字段为空时，才使用其他库补充
		if main.Name == "" || main.ISO == "" {
			if h.czdbV4 != nil && isChinaIPv4(ip) {
				if c, err := h.czdbV4.Country(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
			if h.czdbV6 != nil && isChinaIPv6(ip) {
				if c, err := h.czdbV6.Country(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
			if h.qqwry != nil {
				if c, err := h.qqwry.Country(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
			if h.maxmind != nil {
				if c, err := h.maxmind.Country(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
		}
		
		return mergeCountryWithPriority(main, supplements...), nil
	}
	
	// 非中国大陆 IP 以 MaxMind 为主
	if h.maxmind != nil {
		return h.maxmind.Country(ip)
	}
	
	// 如果没有MaxMind，尝试其他数据库
	if h.geocn != nil {
		return h.geocn.Country(ip)
	}
	if h.czdbV4 != nil {
		return h.czdbV4.Country(ip)
	}
	if h.czdbV6 != nil {
		return h.czdbV6.Country(ip)
	}
	if h.qqwry != nil {
		return h.qqwry.Country(ip)
	}
	
	return Country{}, nil
}

// City returns merged city information from all sources
func (h *HybridReader) City(ip net.IP) (City, error) {
	if h.maxmind == nil && h.geocn == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return City{}, nil
	}
	
	// 对于中国大陆IP，优先使用GeoCN.mmdb
	if h.isChinaMainlandIP(ip) {
		var main City
		var supplements []City
		
		// GeoCN.mmdb has the highest priority for China mainland IPs
		if h.geocn != nil {
			main, _ = h.geocn.City(ip)
		}
		
		// 只有当GeoCN.mmdb没有数据或字段为空时，才使用其他库补充
		needSupplement := main.Name == "" || main.RegionName == "" ||
			main.District == "" || main.Latitude == 0 || main.Longitude == 0 || main.Timezone == ""
		
		if needSupplement {
			if h.czdbV4 != nil && isChinaIPv4(ip) {
				if c, err := h.czdbV4.City(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
			if h.czdbV6 != nil && isChinaIPv6(ip) {
				if c, err := h.czdbV6.City(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
			if h.qqwry != nil {
				if c, err := h.qqwry.City(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
			if h.maxmind != nil {
				if c, err := h.maxmind.City(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
		}
		
		return mergeCityWithPriority(main, supplements...), nil
	}
	
	// 非中国大陆 IP 以 MaxMind 为主
	if h.maxmind != nil {
		main, _ := h.maxmind.City(ip)
		var supplements []City
		if h.geocn != nil {
			if c, err := h.geocn.City(ip); err == nil {
				supplements = append(supplements, c)
			}
		}
		if h.czdbV4 != nil {
			if c, err := h.czdbV4.City(ip); err == nil {
				supplements = append(supplements, c)
			}
		}
		if h.czdbV6 != nil {
			if c, err := h.czdbV6.City(ip); err == nil {
				supplements = append(supplements, c)
			}
		}
		if h.qqwry != nil {
			if c, err := h.qqwry.City(ip); err == nil {
				supplements = append(supplements, c)
			}
		}
		return mergeCityWithPriority(main, supplements...), nil
	}
	
	// 如果没有MaxMind，尝试其他数据库
	if h.geocn != nil {
		return h.geocn.City(ip)
	}
	if h.czdbV4 != nil {
		return h.czdbV4.City(ip)
	}
	if h.czdbV6 != nil {
		return h.czdbV6.City(ip)
	}
	if h.qqwry != nil {
		return h.qqwry.City(ip)
	}
	
	return City{}, nil
}

// ASN returns merged ASN information from all sources
func (h *HybridReader) ASN(ip net.IP) (ASN, error) {
	if h.maxmind == nil && h.geocn == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return ASN{}, nil
	}
	if h.maxmind != nil && !h.isChinaMainlandIP(ip) {
		main, _ := h.maxmind.ASN(ip)
		var a1, a2, a3, a4 ASN
		if h.geocn != nil {
			a1, _ = h.geocn.ASN(ip)
		}
		if h.czdbV4 != nil {
			a2, _ = h.czdbV4.ASN(ip)
		}
		if h.czdbV6 != nil {
			a3, _ = h.czdbV6.ASN(ip)
		}
		if h.qqwry != nil {
			a4, _ = h.qqwry.ASN(ip)
		}
		return mergeASNWithPriority(main, a1, a2, a3, a4), nil
	}
	var main ASN
	var a1, a2, a3, a4 ASN
	// GeoCN.mmdb has the highest priority
	if h.isChinaMainlandIP(ip) && h.geocn != nil {
		main, _ = h.geocn.ASN(ip)
	} else if isChinaIPv4(ip) && h.czdbV4 != nil {
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
	if h.czdbV4 != nil {
		a3, _ = h.czdbV4.ASN(ip)
	}
	if h.czdbV6 != nil {
		a4, _ = h.czdbV6.ASN(ip)
	}
	return mergeASNWithPriority(main, a1, a2, a3, a4), nil
}

// ISP returns merged ISP information from all sources
func (h *HybridReader) ISP(ip net.IP) (ISP, error) {
	if h.maxmind == nil && h.geocn == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return ISP{}, nil
	}
	
	// 对于中国大陆IP，优先使用GeoCN.mmdb
	if h.isChinaMainlandIP(ip) {
		var main ISP
		var supplements []ISP
		
		// GeoCN.mmdb has the highest priority for China mainland IPs
		if h.geocn != nil {
			main, _ = h.geocn.ISP(ip)
		}
		
		// 只有当GeoCN.mmdb没有数据或字段为空时，才使用其他库补充
		needSupplement := main.ISP == "" || main.Organization == "" || main.ASN == 0
		
		if needSupplement {
			if h.czdbV4 != nil && isChinaIPv4(ip) {
				if i, err := h.czdbV4.ISP(ip); err == nil {
					supplements = append(supplements, i)
				}
			}
			if h.czdbV6 != nil && isChinaIPv6(ip) {
				if i, err := h.czdbV6.ISP(ip); err == nil {
					supplements = append(supplements, i)
				}
			}
			if h.qqwry != nil {
				if i, err := h.qqwry.ISP(ip); err == nil {
					supplements = append(supplements, i)
				}
			}
			if h.maxmind != nil {
				if i, err := h.maxmind.ISP(ip); err == nil {
					supplements = append(supplements, i)
				}
			}
		}
		
		return mergeISPWithPriority(main, supplements...), nil
	}
	
	// 非中国大陆 IP 以 MaxMind 为主
	if h.maxmind != nil {
		main, _ := h.maxmind.ISP(ip)
		var supplements []ISP
		if h.geocn != nil {
			if i, err := h.geocn.ISP(ip); err == nil {
				supplements = append(supplements, i)
			}
		}
		if h.czdbV4 != nil {
			if i, err := h.czdbV4.ISP(ip); err == nil {
				supplements = append(supplements, i)
			}
		}
		if h.czdbV6 != nil {
			if i, err := h.czdbV6.ISP(ip); err == nil {
				supplements = append(supplements, i)
			}
		}
		if h.qqwry != nil {
			if i, err := h.qqwry.ISP(ip); err == nil {
				supplements = append(supplements, i)
			}
		}
		return mergeISPWithPriority(main, supplements...), nil
	}
	
	// 如果没有MaxMind，尝试其他数据库
	if h.geocn != nil {
		return h.geocn.ISP(ip)
	}
	if h.czdbV4 != nil {
		return h.czdbV4.ISP(ip)
	}
	if h.czdbV6 != nil {
		return h.czdbV6.ISP(ip)
	}
	if h.qqwry != nil {
		return h.qqwry.ISP(ip)
	}
	
	return ISP{}, nil
}

// ConnectionType returns connection type, preferring GeoCN.mmdb for China mainland IPs
func (h *HybridReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	// 对于中国大陆IP，优先使用GeoCN.mmdb
	if h.isChinaMainlandIP(ip) {
		var main ConnectionType
		var supplements []ConnectionType
		
		// GeoCN.mmdb has the highest priority for China mainland IPs
		if h.geocn != nil {
			main, _ = h.geocn.ConnectionType(ip)
		}
		
		// 只有当GeoCN.mmdb没有数据或字段为空时，才使用其他库补充
		if main.ConnectionType == "" {
			if h.czdbV4 != nil && isChinaIPv4(ip) {
				if c, err := h.czdbV4.ConnectionType(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
			if h.czdbV6 != nil && isChinaIPv6(ip) {
				if c, err := h.czdbV6.ConnectionType(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
			if h.qqwry != nil {
				if c, err := h.qqwry.ConnectionType(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
			if h.maxmind != nil {
				if c, err := h.maxmind.ConnectionType(ip); err == nil {
					supplements = append(supplements, c)
				}
			}
		}
		
		// 如果主数据有值，直接返回；否则合并补充数据
		if main.ConnectionType != "" {
			return main, nil
		}
		
		// 从补充数据中找到第一个非空的
		for _, c := range supplements {
			if c.ConnectionType != "" {
				return c, nil
			}
		}
		
		return ConnectionType{}, nil
	} else {
		// 非中国大陆IP，MaxMind优先
		if h.maxmind != nil {
			connType, err := h.maxmind.ConnectionType(ip)
			if err == nil && connType.ConnectionType != "" {
				return connType, nil
			}
		}
		
		// Fallback to other readers
		if h.geocn != nil {
			connType, err := h.geocn.ConnectionType(ip)
			if err == nil && connType.ConnectionType != "" {
				return connType, nil
			}
		}
		if h.czdbV4 != nil {
			connType, err := h.czdbV4.ConnectionType(ip)
			if err == nil && connType.ConnectionType != "" {
				return connType, nil
			}
		}
		if h.czdbV6 != nil {
			connType, err := h.czdbV6.ConnectionType(ip)
			if err == nil && connType.ConnectionType != "" {
				return connType, nil
			}
		}
		if h.qqwry != nil {
			connType, err := h.qqwry.ConnectionType(ip)
			if err == nil && connType.ConnectionType != "" {
				return connType, nil
			}
		}
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

// Network returns network information for the given IP
func (h *HybridReader) Network(ip net.IP) (*net.IPNet, error) {
	// 对于中国大陆IP，优先使用GeoCN.mmdb
	if h.isChinaMainlandIP(ip) {
		if h.geocn != nil {
			network, err := h.geocn.Network(ip)
			if err == nil && network != nil {
				return network, nil
			}
		}
		
		// 如果GeoCN.mmdb没有数据，尝试其他数据库
		if h.czdbV4 != nil && isChinaIPv4(ip) {
			network, err := h.czdbV4.Network(ip)
			if err == nil && network != nil {
				return network, nil
			}
		}
		if h.czdbV6 != nil && isChinaIPv6(ip) {
			network, err := h.czdbV6.Network(ip)
			if err == nil && network != nil {
				return network, nil
			}
		}
		if h.qqwry != nil {
			network, err := h.qqwry.Network(ip)
			if err == nil && network != nil {
				return network, nil
			}
		}
		if h.maxmind != nil {
			network, err := h.maxmind.Network(ip)
			if err == nil && network != nil {
				return network, nil
			}
		}
	} else {
		// 非中国大陆IP，MaxMind优先
		if h.maxmind != nil {
			network, err := h.maxmind.Network(ip)
			if err == nil && network != nil {
				return network, nil
			}
		}
		
		// Fallback to other readers
		if h.geocn != nil {
			network, err := h.geocn.Network(ip)
			if err == nil && network != nil {
				return network, nil
			}
		}
		if h.czdbV4 != nil {
			network, err := h.czdbV4.Network(ip)
			if err == nil && network != nil {
				return network, nil
			}
		}
		if h.czdbV6 != nil {
			network, err := h.czdbV6.Network(ip)
			if err == nil && network != nil {
				return network, nil
			}
		}
		if h.qqwry != nil {
			network, err := h.qqwry.Network(ip)
			if err == nil && network != nil {
				return network, nil
			}
		}
	}
	
	return nil, fmt.Errorf("no network information available")
}


// IsEmpty returns true if both readers are empty
func (h *HybridReader) IsEmpty() bool {
	maxmindEmpty := h.maxmind == nil || h.maxmind.IsEmpty()
	qqwryEmpty := h.qqwry == nil || h.qqwry.IsEmpty()
	geocnEmpty := h.geocn == nil || h.geocn.IsEmpty()
	return maxmindEmpty && qqwryEmpty && geocnEmpty
}
