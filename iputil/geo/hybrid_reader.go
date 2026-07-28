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

// provinceEquiv defines mapping of Chinese province names, pinyin, and ISO 3166-2 codes
var provinceEquiv = []map[string]bool{
	{"北京": true, "beijing": true, "bj": true, "11": true},
	{"天津": true, "tianjin": true, "tj": true, "12": true},
	{"河北": true, "hebei": true, "he": true, "13": true},
	{"山西": true, "shanxi": true, "sx": true, "14": true},
	{"内蒙古": true, "内蒙": true, "neimenggu": true, "inner mongolia": true, "nm": true, "15": true},
	{"辽宁": true, "liaoning": true, "ln": true, "21": true},
	{"吉林": true, "jilin": true, "jl": true, "22": true},
	{"黑龙江": true, "heilongjiang": true, "hl": true, "23": true},
	{"上海": true, "shanghai": true, "sh": true, "31": true},
	{"江苏": true, "jiangsu": true, "js": true, "32": true},
	{"浙江": true, "zhejiang": true, "zj": true, "33": true},
	{"安徽": true, "anhui": true, "ah": true, "34": true},
	{"福建": true, "fujian": true, "fj": true, "35": true},
	{"江西": true, "jiangxi": true, "jx": true, "36": true},
	{"山东": true, "shandong": true, "sd": true, "37": true},
	{"河南": true, "henan": true, "ha": true, "41": true},
	{"湖北": true, "hubei": true, "hb": true, "42": true},
	{"湖南": true, "hunan": true, "hn": true, "43": true},
	{"广东": true, "guangdong": true, "gd": true, "44": true},
	{"广西": true, "guangxi": true, "gx": true, "45": true},
	{"海南": true, "hainan": true, "hi": true, "46": true},
	{"重庆": true, "chongqing": true, "cq": true, "50": true},
	{"四川": true, "sichuan": true, "sc": true, "51": true},
	{"贵州": true, "guizhou": true, "gz": true, "52": true},
	{"云南": true, "yunnan": true, "yn": true, "53": true},
	{"西藏": true, "xizang": true, "tibet": true, "xz": true, "54": true},
	{"陕西": true, "shaanxi": true, "sn": true, "61": true},
	{"甘肃": true, "gansu": true, "gs": true, "62": true},
	{"青海": true, "qinghai": true, "qh": true, "63": true},
	{"宁夏": true, "ningxia": true, "nx": true, "64": true},
	{"新疆": true, "xinjiang": true, "xj": true, "65": true},
	{"香港": true, "hong kong": true, "hongkong": true, "hk": true, "81": true},
	{"澳门": true, "macao": true, "macau": true, "mo": true, "82": true},
	{"台湾": true, "taiwan": true, "tw": true, "71": true},
}

func cleanRegionStr(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, "省")
	s = strings.TrimSuffix(s, "市")
	s = strings.TrimSuffix(s, "自治区")
	s = strings.TrimSuffix(s, "特别行政区")
	s = strings.TrimSuffix(s, "回族")
	s = strings.TrimSuffix(s, "壮族")
	s = strings.TrimSuffix(s, "维吾尔")
	return s
}

func isCompatibleRegion(r1, r2, code1, code2 string) bool {
	if r1 == "" || r2 == "" {
		return true
	}
	c1 := cleanRegionStr(r1)
	c2 := cleanRegionStr(r2)
	if c1 == c2 {
		return true
	}
	idx1 := -1
	idx2 := -1
	for i, equiv := range provinceEquiv {
		if equiv[c1] || (code1 != "" && equiv[strings.ToLower(code1)]) {
			idx1 = i
		}
		if equiv[c2] || (code2 != "" && equiv[strings.ToLower(code2)]) {
			idx2 = i
		}
	}
	if idx1 != -1 && idx2 != -1 {
		return idx1 == idx2
	}
	if isChinese(c1) && isChinese(c2) && c1 != c2 {
		return false
	}
	return true
}

func isChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func mergeCityWithPriority(primary City, supplements ...City) City {
	all := append([]City{primary}, supplements...)
	var best City
	bestIdx := -1
	// 1. First try to find the highest-priority candidate that has exact City-level precision (Name != "")
	for i, c := range all {
		if c.Name != "" && c.RegionName != "" {
			best = c
			bestIdx = i
			break
		}
	}
	// 2. If no candidate has exact City-level precision, find the highest-priority candidate with Province-level precision (RegionName != "")
	if bestIdx == -1 {
		for i, c := range all {
			if c.RegionName != "" {
				best = c
				bestIdx = i
				break
			}
		}
	}
	// 3. If still nothing, just use primary
	if bestIdx == -1 {
		best = primary
		bestIdx = 0
	}

	out := best
	for i, c := range all {
		if i == bestIdx {
			continue
		}
		if !isCompatibleRegion(out.RegionName, c.RegionName, out.RegionCode, c.RegionCode) {
			continue // Prevent stitching fields from a conflicting geographical region!
		}
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
	var maxmindCity City
	if h.maxmind != nil {
		maxmindCity, _ = h.maxmind.City(ip)
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
			supplements = append(supplements, maxmindCity)
		}
		merged := mergeCityWithPriority(main, supplements...)
		if h.maxmind != nil && (merged.Latitude == 0 && merged.Longitude == 0) &&
			isCompatibleRegion(merged.RegionName, maxmindCity.RegionName, merged.RegionCode, maxmindCity.RegionCode) {
			merged.Latitude = maxmindCity.Latitude
			merged.Longitude = maxmindCity.Longitude
		}
		return merged, nil
	}
	// Non-CN: MaxMind primary; then geocn -> czdbV4 -> czdbV6 -> qqwry (ISO guard)
	if h.maxmind != nil {
		main = maxmindCity
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
	merged := mergeCityWithPriority(main, supplements...)
	if h.maxmind != nil && (merged.Latitude == 0 && merged.Longitude == 0) &&
		isCompatibleRegion(merged.RegionName, maxmindCity.RegionName, merged.RegionCode, maxmindCity.RegionCode) {
		merged.Latitude = maxmindCity.Latitude
		merged.Longitude = maxmindCity.Longitude
	}
	return merged, nil
}

// ASN returns merged ASN information from all sources
func (h *HybridReader) ASN(ip net.IP) (ASN, error) {
	if h.maxmind != nil {
		return h.maxmind.ASN(ip)
	}
	return ASN{}, nil
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
	var primaryISO string
	if h.maxmind != nil {
		if c, err := h.maxmind.Country(ip); err == nil {
			primaryISO = strings.ToUpper(c.ISO)
		}
	}
	if primaryISO == "CN" {
		if h.geocn != nil {
			if ct, err := h.geocn.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
				return ct, nil
			}
		}
		if h.maxmind != nil {
			return h.maxmind.ConnectionType(ip)
		}
		return ConnectionType{}, nil
	}
	if h.maxmind != nil {
		if ct, err := h.maxmind.ConnectionType(ip); err == nil && ct.ConnectionType != "" {
			return ct, nil
		}
	}
	if h.geocn != nil && h.isoMatches(h.geocn, ip, primaryISO) {
		return h.geocn.ConnectionType(ip)
	}
	return ConnectionType{}, nil
}

// Network returns the containing network with ISO guard fallbacks.
func (h *HybridReader) Network(ip net.IP) (*net.IPNet, error) {
	type candidate struct {
		net   *net.IPNet
		order int
	}
	var (
		primaryISO string
		candidates []candidate
		order      int
	)
	cloneNetwork := func(n *net.IPNet) *net.IPNet {
		if n == nil {
			return nil
		}
		masked := n.IP.Mask(n.Mask)
		if masked == nil {
			return nil
		}
		ipCopy := make(net.IP, len(masked))
		copy(ipCopy, masked)
		maskCopy := append(net.IPMask(nil), n.Mask...)
		return &net.IPNet{IP: ipCopy, Mask: maskCopy}
	}
	if h.maxmind != nil {
		if c, err := h.maxmind.Country(ip); err == nil {
			primaryISO = strings.ToUpper(c.ISO)
		}
	}
	appendCandidate := func(reader Reader, enforceISO bool) error {
		if reader == nil {
			order++
			return nil
		}
		if enforceISO && !h.isoMatches(reader, ip, primaryISO) {
			order++
			return nil
		}
		n, err := reader.Network(ip)
		if err != nil {
			return err
		}
		if n != nil {
			candidates = append(candidates, candidate{net: cloneNetwork(n), order: order})
		}
		order++
		return nil
	}
	if err := appendCandidate(h.maxmind, false); err != nil {
		return nil, err
	}
	readers := []Reader{h.geocn, h.czdbV4, h.czdbV6, h.qqwry}
	for _, reader := range readers {
		if err := appendCandidate(reader, true); err != nil {
			return nil, err
		}
	}
	bestOrder := -1
	bestPrefix := -1
	var bestNet *net.IPNet
	for _, cand := range candidates {
		if cand.net == nil || cand.net.Mask == nil {
			continue
		}
		ones, bits := cand.net.Mask.Size()
		if ones < 0 || bits <= 0 {
			continue
		}
		if ones > bestPrefix || (ones == bestPrefix && (bestOrder == -1 || cand.order < bestOrder)) {
			bestPrefix = ones
			bestOrder = cand.order
			bestNet = cand.net
		}
	}
	return bestNet, nil
}

// Proxy returns proxy information using the appropriate reader
func (h *HybridReader) Proxy(ip net.IP) (Proxy, error) {
	if h.maxmind != nil {
		return h.maxmind.Proxy(ip)
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
	if primaryISO == "CN" && (r == h.geocn || r == h.czdbV4 || r == h.czdbV6 || r == h.qqwry) {
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
