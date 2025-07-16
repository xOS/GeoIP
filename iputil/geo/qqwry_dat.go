package geo

import (
	"fmt"
	"net"
	"strings"

	"github.com/yinheli/qqwry"
)

// QQWryDatReader implements Reader interface for qqwry.dat format
type QQWryDatReader struct {
	db *qqwry.QQwry
}

// NewQQWryDatReader creates a new qqwry.dat reader
func NewQQWryDatReader(path string) (Reader, error) {
	db := qqwry.NewQQwry(path)
	if db == nil {
		return nil, fmt.Errorf("failed to open qqwry.dat file: %s", path)
	}

	return &QQWryDatReader{db: db}, nil
}

// Country returns country information from qqwry.dat
func (q *QQWryDatReader) Country(ip net.IP) (Country, error) {
	q.db.Find(ip.String())

	country := Country{}
	// qqwry.dat typically shows China for most Chinese IPs
	// Parse the location string to extract country info
	location := q.db.City
	if location == "" {
		location = q.db.Country
	}

	// Most qqwry.dat entries are for China
	if strings.Contains(location, "中国") ||
		strings.Contains(location, "北京") ||
		strings.Contains(location, "上海") ||
		strings.Contains(location, "广东") ||
		strings.Contains(location, "浙江") ||
		strings.Contains(location, "江苏") ||
		strings.Contains(location, "四川") ||
		strings.Contains(location, "湖北") ||
		strings.Contains(location, "湖南") ||
		strings.Contains(location, "河北") ||
		strings.Contains(location, "河南") ||
		strings.Contains(location, "山东") ||
		strings.Contains(location, "山西") ||
		strings.Contains(location, "陕西") ||
		strings.Contains(location, "辽宁") ||
		strings.Contains(location, "吉林") ||
		strings.Contains(location, "黑龙江") ||
		strings.Contains(location, "福建") ||
		strings.Contains(location, "安徽") ||
		strings.Contains(location, "江西") ||
		strings.Contains(location, "重庆") ||
		strings.Contains(location, "天津") ||
		strings.Contains(location, "内蒙") ||
		strings.Contains(location, "广西") ||
		strings.Contains(location, "西藏") ||
		strings.Contains(location, "宁夏") ||
		strings.Contains(location, "新疆") ||
		strings.Contains(location, "青海") ||
		strings.Contains(location, "甘肃") ||
		strings.Contains(location, "云南") ||
		strings.Contains(location, "贵州") ||
		strings.Contains(location, "海南") ||
		strings.Contains(location, "香港") ||
		strings.Contains(location, "澳门") ||
		strings.Contains(location, "台湾") {
		country.Name = "中国"
		country.ISO = "CN"
	} else {
		// For non-China IPs, use the country field as country name
		country.Name = q.db.Country
		// Try to guess ISO code from common patterns
		if strings.Contains(location, "美国") || strings.Contains(location, "United States") {
			country.ISO = "US"
		} else if strings.Contains(location, "日本") || strings.Contains(location, "Japan") {
			country.ISO = "JP"
		} else if strings.Contains(location, "韩国") || strings.Contains(location, "Korea") {
			country.ISO = "KR"
		} else if strings.Contains(location, "俄罗斯") || strings.Contains(location, "Russia") {
			country.ISO = "RU"
		} else if strings.Contains(location, "德国") || strings.Contains(location, "Germany") {
			country.ISO = "DE"
		} else if strings.Contains(location, "英国") || strings.Contains(location, "United Kingdom") {
			country.ISO = "GB"
		} else if strings.Contains(location, "法国") || strings.Contains(location, "France") {
			country.ISO = "FR"
		} else if strings.Contains(location, "澳大利亚") || strings.Contains(location, "Australia") {
			country.ISO = "AU"
		}
	}

	return country, nil
}

// City returns city information from qqwry.dat
func (q *QQWryDatReader) City(ip net.IP) (City, error) {
	q.db.Find(ip.String())

	city := City{}

	// Parse country and city fields to extract city and region
	country := q.db.Country
	area := q.db.City

	// Try to extract province/region and city from the location strings
	location := country + " " + area

	// Common Chinese provinces
	provinces := map[string]string{
		"北京":  "北京市",
		"上海":  "上海市",
		"天津":  "天津市",
		"重庆":  "重庆市",
		"广东":  "广东省",
		"浙江":  "浙江省",
		"江苏":  "江苏省",
		"四川":  "四川省",
		"湖北":  "湖北省",
		"湖南":  "湖南省",
		"河北":  "河北省",
		"河南":  "河南省",
		"山东":  "山东省",
		"山西":  "山西省",
		"陕西":  "陕西省",
		"辽宁":  "辽宁省",
		"吉林":  "吉林省",
		"黑龙江": "黑龙江省",
		"福建":  "福建省",
		"安徽":  "安徽省",
		"江西":  "江西省",
		"内蒙":  "内蒙古自治区",
		"广西":  "广西壮族自治区",
		"西藏":  "西藏自治区",
		"宁夏":  "宁夏回族自治区",
		"新疆":  "新疆维吾尔自治区",
		"青海":  "青海省",
		"甘肃":  "甘肃省",
		"云南":  "云南省",
		"贵州":  "贵州省",
		"海南":  "海南省",
		"香港":  "香港特别行政区",
		"澳门":  "澳门特别行政区",
		"台湾":  "台湾省",
	}

	// Find province/region
	for shortName := range provinces {
		if strings.Contains(location, shortName) {
			city.RegionName = shortName
			break
		}
	}

	// Extract city name (this is a simplified approach)
	// qqwry.dat format varies, so we'll use the city field as city name if it looks like a city
	if area != "" && area != "CZ88.NET" && !strings.Contains(area, "网络") && !strings.Contains(area, "电信") && !strings.Contains(area, "联通") && !strings.Contains(area, "移动") {
		// Remove common ISP names and keep potential city names
		cityName := area
		cityName = strings.ReplaceAll(cityName, "市", "")
		cityName = strings.ReplaceAll(cityName, "县", "")
		cityName = strings.ReplaceAll(cityName, "区", "")
		if len(cityName) > 0 && len(cityName) < 10 { // Reasonable city name length
			city.Name = cityName
		}
	}

	return city, nil
}

// ASN returns ASN information (not available in qqwry.dat)
func (q *QQWryDatReader) ASN(ip net.IP) (ASN, error) {
	return ASN{}, nil
}

// ISP returns ISP information from qqwry.dat
func (q *QQWryDatReader) ISP(ip net.IP) (ISP, error) {
	q.db.Find(ip.String())

	isp := ISP{}

	// Extract ISP information from country and city fields
	location := q.db.Country + " " + q.db.City

	// Common Chinese ISPs
	if strings.Contains(location, "电信") {
		isp.ISP = "电信"
		isp.Organization = "中国电信"
	} else if strings.Contains(location, "联通") {
		isp.ISP = "联通"
		isp.Organization = "中国联通"
	} else if strings.Contains(location, "移动") {
		isp.ISP = "移动"
		isp.Organization = "中国移动"
	} else if strings.Contains(location, "铁通") {
		isp.ISP = "铁通"
		isp.Organization = "中国铁通"
	} else if strings.Contains(location, "教育网") || strings.Contains(location, "CERNET") {
		isp.ISP = "教育网"
		isp.Organization = "中国教育和科研计算机网"
	} else if strings.Contains(location, "长城宽带") {
		isp.ISP = "长城宽带"
		isp.Organization = "长城宽带网络服务有限公司"
	} else if strings.Contains(location, "方正宽带") {
		isp.ISP = "方正宽带"
		isp.Organization = "方正宽带网络服务有限公司"
	} else {
		// Use city field as ISP if it contains ISP-like information
		if q.db.City != "" && q.db.City != "CZ88.NET" {
			isp.ISP = q.db.City
			isp.Organization = q.db.City
		}
	}

	return isp, nil
}

// ConnectionType returns connection type (not available in qqwry.dat)
func (q *QQWryDatReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	return ConnectionType{}, nil
}

// Proxy returns proxy information (not available in qqwry.dat)
func (q *QQWryDatReader) Proxy(ip net.IP) (Proxy, error) {
	return Proxy{}, nil
}

// IsEmpty returns whether the database is empty
func (q *QQWryDatReader) IsEmpty() bool {
	return q.db == nil
}
