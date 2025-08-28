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
// (legacy merge* helpers removed)

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
	// Primary always MaxMind
	var primary Country
	if h.maxmind != nil {
		primary, _ = h.maxmind.Country(ip)
	}
	// If no MaxMind, fallback to others in order
	if primary.ISO == "" && primary.Name == "" {
		if h.geocn != nil {
			if c, err := h.geocn.Country(ip); err == nil {
				primary = c
			}
		}
		if primary.ISO == "" && primary.Name == "" && h.czdbV4 != nil {
			if c, err := h.czdbV4.Country(ip); err == nil {
				primary = c
			}
		}
		if primary.ISO == "" && primary.Name == "" && h.czdbV6 != nil {
			if c, err := h.czdbV6.Country(ip); err == nil {
				primary = c
			}
		}
		if primary.ISO == "" && primary.Name == "" && h.qqwry != nil {
			if c, err := h.qqwry.Country(ip); err == nil {
				primary = c
			}
		}
		return primary, nil
	}
	// Supplements with ISO guard following order: geocn, czdbV4, czdbV6, qqwry
	primaryISO := strings.ToUpper(primary.ISO)
	var supplements []Country
	if h.geocn != nil && h.isoMatches(h.geocn, ip, primaryISO) {
		if c, err := h.geocn.Country(ip); err == nil {
			supplements = append(supplements, c)
		}
	}
	if h.czdbV4 != nil && h.isoMatches(h.czdbV4, ip, primaryISO) {
		if c, err := h.czdbV4.Country(ip); err == nil {
			supplements = append(supplements, c)
		}
	}
	if h.czdbV6 != nil && h.isoMatches(h.czdbV6, ip, primaryISO) {
		if c, err := h.czdbV6.Country(ip); err == nil {
			supplements = append(supplements, c)
		}
	}
	if h.qqwry != nil && h.isoMatches(h.qqwry, ip, primaryISO) {
		if c, err := h.qqwry.Country(ip); err == nil {
			supplements = append(supplements, c)
		}
	}
	return mergeCountryWithPriority(primary, supplements...), nil
}

// City returns merged city information from all sources
func (h *HybridReader) City(ip net.IP) (City, error) {
	if h.maxmind == nil && h.geocn == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return City{}, nil
	}

	// Country is determined by MaxMind; use it to branch CN/non-CN
	var primaryISO string
	if h.maxmind != nil {
		if c, err := h.maxmind.Country(ip); err == nil {
			primaryISO = strings.ToUpper(c.ISO)
		}
	}
	var main City
	var supplements []City
	if primaryISO == "CN" {
		// CN: GeoCN primary, then czdbV4 -> czdbV6 -> qqwry (ISO guard)
		if h.geocn != nil {
			main, _ = h.geocn.City(ip)
		}
		if h.czdbV4 != nil && h.isoMatches(h.czdbV4, ip, primaryISO) {
			if c, err := h.czdbV4.City(ip); err == nil {
				supplements = append(supplements, c)
			}
		}
		if h.czdbV6 != nil && h.isoMatches(h.czdbV6, ip, primaryISO) {
			if c, err := h.czdbV6.City(ip); err == nil {
				supplements = append(supplements, c)
			}
		}
		if h.qqwry != nil && h.isoMatches(h.qqwry, ip, primaryISO) {
			if c, err := h.qqwry.City(ip); err == nil {
				supplements = append(supplements, c)
			}
		}
		// Optionally use MaxMind as the last resort for missing fields if ISO matches
		if h.maxmind != nil && h.isoMatches(h.maxmind, ip, primaryISO) {
			if c, err := h.maxmind.City(ip); err == nil {
				supplements = append(supplements, c)
			}
		}
		return mergeCityWithPriority(main, supplements...), nil
	}
	// Non-CN: MaxMind primary; then geocn -> czdbV4 -> czdbV6 -> qqwry (ISO guard)
	if h.maxmind != nil {
		main, _ = h.maxmind.City(ip)
	}
	if h.geocn != nil && h.isoMatches(h.geocn, ip, primaryISO) {
		if c, err := h.geocn.City(ip); err == nil {
			supplements = append(supplements, c)
		}
	}
	if h.czdbV4 != nil && h.isoMatches(h.czdbV4, ip, primaryISO) {
		if c, err := h.czdbV4.City(ip); err == nil {
			supplements = append(supplements, c)
		}
	}
	if h.czdbV6 != nil && h.isoMatches(h.czdbV6, ip, primaryISO) {
		if c, err := h.czdbV6.City(ip); err == nil {
			supplements = append(supplements, c)
		}
	}
	if h.qqwry != nil && h.isoMatches(h.qqwry, ip, primaryISO) {
		if c, err := h.qqwry.City(ip); err == nil {
			supplements = append(supplements, c)
		}
	}
	return mergeCityWithPriority(main, supplements...), nil
}

// ASN returns merged ASN information from all sources
func (h *HybridReader) ASN(ip net.IP) (ASN, error) {
	if h.maxmind == nil && h.geocn == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return ASN{}, nil
	}
	// Country is determined by MaxMind; branch CN/non-CN for primary
	var primaryISO string
	if h.maxmind != nil {
		if c, err := h.maxmind.Country(ip); err == nil {
			primaryISO = strings.ToUpper(c.ISO)
		}
	}
	var main ASN
	var a1, a2, a3, a4 ASN
	if primaryISO == "CN" {
		if h.geocn != nil {
			main, _ = h.geocn.ASN(ip)
		}
		if h.czdbV4 != nil && h.isoMatches(h.czdbV4, ip, primaryISO) {
			a1, _ = h.czdbV4.ASN(ip)
		}
		if h.czdbV6 != nil && h.isoMatches(h.czdbV6, ip, primaryISO) {
			a2, _ = h.czdbV6.ASN(ip)
		}
		if h.qqwry != nil && h.isoMatches(h.qqwry, ip, primaryISO) {
			a3, _ = h.qqwry.ASN(ip)
		}
		if h.maxmind != nil && h.isoMatches(h.maxmind, ip, primaryISO) {
			a4, _ = h.maxmind.ASN(ip)
		}
		return mergeASNWithPriority(main, a1, a2, a3, a4), nil
	}
	if h.maxmind != nil {
		main, _ = h.maxmind.ASN(ip)
	}
	if h.geocn != nil && h.isoMatches(h.geocn, ip, primaryISO) {
		a1, _ = h.geocn.ASN(ip)
	}
	if h.czdbV4 != nil && h.isoMatches(h.czdbV4, ip, primaryISO) {
		a2, _ = h.czdbV4.ASN(ip)
	}
	if h.czdbV6 != nil && h.isoMatches(h.czdbV6, ip, primaryISO) {
		a3, _ = h.czdbV6.ASN(ip)
	}
	if h.qqwry != nil && h.isoMatches(h.qqwry, ip, primaryISO) {
		a4, _ = h.qqwry.ASN(ip)
	}
	return mergeASNWithPriority(main, a1, a2, a3, a4), nil
}

// ISP returns merged ISP information from all sources
func (h *HybridReader) ISP(ip net.IP) (ISP, error) {
	if h.maxmind == nil && h.geocn == nil && h.czdbV4 == nil && h.czdbV6 == nil && h.qqwry == nil {
		return ISP{}, nil
	}

	// Use country (MaxMind) to branch CN/non-CN for primary
	var primaryISO string
	if h.maxmind != nil {
		if c, err := h.maxmind.Country(ip); err == nil {
			primaryISO = strings.ToUpper(c.ISO)
		}
	}
	var main ISP
	var supplements []ISP
	if primaryISO == "CN" {
		if h.geocn != nil {
			main, _ = h.geocn.ISP(ip)
		}
		if h.czdbV4 != nil && h.isoMatches(h.czdbV4, ip, primaryISO) {
			if i, err := h.czdbV4.ISP(ip); err == nil {
				supplements = append(supplements, i)
			}
		}
		if h.czdbV6 != nil && h.isoMatches(h.czdbV6, ip, primaryISO) {
			if i, err := h.czdbV6.ISP(ip); err == nil {
				supplements = append(supplements, i)
			}
		}
		if h.qqwry != nil && h.isoMatches(h.qqwry, ip, primaryISO) {
			if i, err := h.qqwry.ISP(ip); err == nil {
				supplements = append(supplements, i)
			}
		}
		if h.maxmind != nil && h.isoMatches(h.maxmind, ip, primaryISO) {
			if i, err := h.maxmind.ISP(ip); err == nil {
				supplements = append(supplements, i)
			}
		}
		return mergeISPWithPriority(main, supplements...), nil
	}
	if h.maxmind != nil {
		main, _ = h.maxmind.ISP(ip)
	}
	if h.geocn != nil && h.isoMatches(h.geocn, ip, primaryISO) {
		if i, err := h.geocn.ISP(ip); err == nil {
			supplements = append(supplements, i)
		}
	}
	if h.czdbV4 != nil && h.isoMatches(h.czdbV4, ip, primaryISO) {
		if i, err := h.czdbV4.ISP(ip); err == nil {
			supplements = append(supplements, i)
		}
	}
	if h.czdbV6 != nil && h.isoMatches(h.czdbV6, ip, primaryISO) {
		if i, err := h.czdbV6.ISP(ip); err == nil {
			supplements = append(supplements, i)
		}
	}
	if h.qqwry != nil && h.isoMatches(h.qqwry, ip, primaryISO) {
		if i, err := h.qqwry.ISP(ip); err == nil {
			supplements = append(supplements, i)
		}
	}
	return mergeISPWithPriority(main, supplements...), nil
}

// ConnectionType returns connection type, preferring GeoCN.mmdb for China mainland IPs
func (h *HybridReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	// 对于中国大陆IP，优先使用GeoCN.mmdb
	// Use country (MaxMind) to branch CN/non-CN for primary
	var primaryISO string
	if h.maxmind != nil {
		if c, err := h.maxmind.Country(ip); err == nil {
			primaryISO = strings.ToUpper(c.ISO)
		}
	}
	if primaryISO == "CN" {
		// CN: GeoCN primary; then czdbV4 -> czdbV6 -> qqwry -> maxmind
		if h.geocn != nil {
			if ct, err := h.geocn.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
				return ct, nil
			}
		}
		if h.czdbV4 != nil && h.isoMatches(h.czdbV4, ip, primaryISO) {
			if ct, err := h.czdbV4.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
				return ct, nil
			}
		}
		if h.czdbV6 != nil && h.isoMatches(h.czdbV6, ip, primaryISO) {
			if ct, err := h.czdbV6.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
				return ct, nil
			}
		}
		if h.qqwry != nil && h.isoMatches(h.qqwry, ip, primaryISO) {
			if ct, err := h.qqwry.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
				return ct, nil
			}
		}
		if h.maxmind != nil && h.isoMatches(h.maxmind, ip, primaryISO) {
			if ct, err := h.maxmind.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
				return ct, nil
			}
		}
		return ConnectionType{}, nil
	}
	// Non-CN: MaxMind primary then geocn -> czdbV4 -> czdbV6 -> qqwry
	if h.maxmind != nil {
		if ct, err := h.maxmind.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
			return ct, nil
		}
	}
	if h.geocn != nil && h.isoMatches(h.geocn, ip, primaryISO) {
		if ct, err := h.geocn.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
			return ct, nil
		}
	}
	if h.czdbV4 != nil && h.isoMatches(h.czdbV4, ip, primaryISO) {
		if ct, err := h.czdbV4.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
			return ct, nil
		}
	}
	if h.czdbV6 != nil && h.isoMatches(h.czdbV6, ip, primaryISO) {
		if ct, err := h.czdbV6.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
			return ct, nil
		}
	}
	if h.qqwry != nil && h.isoMatches(h.qqwry, ip, primaryISO) {
		if ct, err := h.qqwry.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
			return ct, nil
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

// isoMatches checks whether reader r reports the same country ISO as primaryISO.
func (h *HybridReader) isoMatches(r Reader, ip net.IP, primaryISO string) bool {
	if r == nil {
		return false
	}
	if primaryISO == "" {
		return true
	}
	c, err := r.Country(ip)
	if err != nil {
		return false
	}
	iso := strings.ToUpper(c.ISO)
	return iso != "" && iso == primaryISO
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
	geocnEmpty := h.geocn == nil || h.geocn.IsEmpty()
	return maxmindEmpty && qqwryEmpty && geocnEmpty
}
