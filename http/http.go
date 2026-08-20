package http

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"net/http/pprof"

	"github.com/xos/geoip/iputil"
	"github.com/xos/geoip/iputil/geo"

	"math/big"
	"net"
	"net/http"
	"strconv"
)

const (
	jsonMediaType = "application/json; charset=utf-8"
	textMediaType = "text/plain"
)

type Server struct {
	Template      string
	IPHeaders     []string
	LookupAddr    func(net.IP) (string, error)
	LookupPort    func(net.IP, uint64) error
	cache         *Cache
	gr            geo.Reader
	profile       bool
	AllowCustomIP bool
}

type Response struct {
	IP             net.IP   `json:"ip"`
	IPDecimal      *big.Int `json:"ip_decimal"`
	Country        string   `json:"country,omitempty"`
	CountryCode    string   `json:"country_code,omitempty"`
	CountryEU      *bool    `json:"country_eu,omitempty"`
	RegionName     string   `json:"region,omitempty"`
	RegionCode     string   `json:"region_code,omitempty"`
	MetroCode      uint     `json:"metro_code,omitempty"`
	PostalCode     string   `json:"zip_code,omitempty"`
	City           string   `json:"city,omitempty"`
	District       string   `json:"district,omitempty"`
	Latitude       float64  `json:"latitude,omitempty"`
	Longitude      float64  `json:"longitude,omitempty"`
	Timezone       string   `json:"time_zone,omitempty"`
	ASN            string   `json:"asn,omitempty"`
	ISP            string   `json:"isp,omitempty"`
	ORG            string   `json:"org,omitempty"`
	IO             string   `json:"isp_org,omitempty"`
	ISPO           string   `json:"isp_asn_org,omitempty"`
	IN             string   `json:"isp_asn,omitempty"`
	ConnectionType string   `json:"connection_type,omitempty"`
	Network        string   `json:"network,omitempty"`
	ProxyType      string   `json:"proxy_type,omitempty"`
	Domain         string   `json:"domain,omitempty"`
	UsageType      string   `json:"usage_type,omitempty"`
	LastSeen       string   `json:"last_seen,omitempty"`
	Threat         string   `json:"threat,omitempty"`
	Provider       string   `json:"provider,omitempty"`
	FraudScore     string   `json:"fraud_score,omitempty"`
	Hostname       string   `json:"hostname,omitempty"`
	UserAgent      string   `json:"user_agent,omitempty"`
}

type UA struct {
	Product string `json:"product,omitempty"`
}

type PortResponse struct {
	IP        net.IP `json:"ip"`
	Port      uint64 `json:"port"`
	Reachable bool   `json:"reachable"`
}

func New(db geo.Reader, cache *Cache, profile bool, allowCustomIP bool) *Server {
	return &Server{cache: cache, gr: db, profile: profile, AllowCustomIP: allowCustomIP}
}

func ipFromForwardedForHeader(v string) string {
	// 处理 X-Forwarded-For 格式: client, proxy1, proxy2
	// 返回第一个（最左边的）IP，这通常是真实客户端IP
	v = strings.TrimSpace(v)
	sep := strings.Index(v, ",")
	if sep == -1 {
		return v
	}
	return strings.TrimSpace(v[:sep])
}

// isPrivateIP 检查IP是否为私有IP
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// IPv4 私有地址范围
	if ip4 := ip.To4(); ip4 != nil {
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
	}

	// IPv6 私有地址
	if ip.To16() != nil && ip.To4() == nil {
		// fc00::/7 (Unique Local Address)
		if ip[0] == 0xfc || ip[0] == 0xfd {
			return true
		}
	}

	return false
}

// ipFromRequest detects the IP address for this transaction.
//
// * `headers` - the specific HTTP headers to trust
// * `r` - the incoming HTTP request
// * `customIP` - whether to allow the IP to be pulled from query parameters
func ipFromRequest(headers []string, r *http.Request, customIP bool) (net.IP, error) {
	remoteIP := ""

	// 1. 首先检查查询参数中的自定义IP
	if customIP && r.URL != nil {
		if v, ok := r.URL.Query()["ip"]; ok {
			remoteIP = v[0]
		}
	}

	// 2. 如果没有自定义IP，按优先级检查HTTP头部
	if remoteIP == "" {
		for _, header := range headers {
			headerValue := r.Header.Get(header)
			if headerValue == "" {
				continue
			}

			// 调试日志（可选，生产环境可以移除）
			// log.Printf("检查头部 %s: %s", header, headerValue)

			// 处理不同类型的头部
			canonicalHeader := http.CanonicalHeaderKey(header)
			switch canonicalHeader {
			case "X-Forwarded-For", "Forwarded-For":
				// 处理可能包含多个IP的情况
				remoteIP = ipFromForwardedForHeader(headerValue)
			case "Forwarded":
				// RFC 7239 格式: for=192.0.2.60;proto=http;by=203.0.113.43
				// 提取 for= 后面的IP
				if strings.Contains(headerValue, "for=") {
					parts := strings.Split(headerValue, ";")
					for _, part := range parts {
						part = strings.TrimSpace(part)
						if strings.HasPrefix(part, "for=") {
							forValue := strings.TrimPrefix(part, "for=")
							// 移除可能的引号和端口号
							forValue = strings.Trim(forValue, "\"")
							if colonIndex := strings.LastIndex(forValue, ":"); colonIndex > 0 {
								// 检查是否是IPv6地址（包含多个冒号）
								if strings.Count(forValue, ":") == 1 {
									forValue = forValue[:colonIndex] // 移除端口号
								}
							}
							remoteIP = forValue
							break
						}
					}
				}
			default:
				// 其他头部直接使用值
				remoteIP = strings.TrimSpace(headerValue)
			}

			// 验证获取到的IP是否有效且不是私有IP
			if remoteIP != "" {
				if testIP := net.ParseIP(remoteIP); testIP != nil && !isPrivateIP(testIP) {
					break // 找到有效的公网IP，停止搜索
				}
				// 如果是私有IP，继续查找下一个头部
				remoteIP = ""
			}
		}
	}

	// 3. 如果所有头部都没有找到有效IP，使用RemoteAddr
	if remoteIP == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return nil, err
		}
		remoteIP = host
	}

	// 4. 解析最终的IP地址
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return nil, fmt.Errorf("could not parse IP: %s", remoteIP)
	}

	return ip, nil
}

// debugHeaders 输出请求中的所有IP相关头部（调试用）
func debugHeaders(r *http.Request) {
	relevantHeaders := []string{
		"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For",
		"X-Client-IP", "X-Forwarded", "X-Cluster-Client-IP",
		"Forwarded-For", "Forwarded", "Remote-Addr",
	}

	log.Printf("=== IP Headers Debug for %s ===", r.URL.Path)
	log.Printf("RemoteAddr: %s", r.RemoteAddr)

	for _, header := range relevantHeaders {
		if value := r.Header.Get(header); value != "" {
			log.Printf("%s: %s", header, value)
		}
	}
	log.Printf("=== End Debug ===")
}

func userAgentFromRequest(r *http.Request) string {
	userAgentRaw := r.UserAgent()
	return userAgentRaw
}

func (s *Server) newResponse(r *http.Request) (Response, error) {
	ip, err := ipFromRequest(s.IPHeaders, r, s.AllowCustomIP)
	if err != nil {
		return Response{}, err
	}
	lang := r.URL.Query().Get("lang")
	response, ok := s.cache.GetWithLang(ip, lang)
	if ok {
		// Do not cache user agent
		response.UserAgent = userAgentFromRequest(r)
		return response, nil
	}
	ipDecimal := iputil.ToDecimal(ip)
	var country geo.Country
	var city geo.City
	var asn geo.ASN
	var isp geo.ISP
	var connectiontype geo.ConnectionType
	var proxy geo.Proxy
	var networkCIDR string

	if hr, ok := s.gr.(interface {
		GetMaxmind() geo.Reader
		GetGeoCN() geo.Reader
		GetCzdbV4() geo.Reader
		GetCzdbV6() geo.Reader
		GetQQWry() geo.Reader
	}); ok && (lang == "zh" || lang == "en") {
		// 使用 lang 参数时，优先使用指定库，然后用其他库补全缺失字段
		if lang == "zh" {
			// 当 lang=zh 时，对于中国大陆IP，GeoCN.mmdb优先级最高
			// 判断是否为中国大陆IP
			var isChinaIP bool
			var maxmindCity geo.City
			hasMaxmindCity := false
			if hr.GetMaxmind() != nil {
				if maxmindCountry, err := hr.GetMaxmind().Country(ip); err == nil && strings.ToUpper(maxmindCountry.ISO) == "CN" {
					isChinaIP = true
				}
				if c, err := hr.GetMaxmind().City(ip); err == nil {
					maxmindCity = c
					hasMaxmindCity = true
				}
			}

			if isChinaIP {
				// 中国大陆IP：GeoCN.mmdb优先级最高，只有当GeoCN.mmdb字段为空时才用其他库填充
				if hr.GetGeoCN() != nil {
					country, _ = hr.GetGeoCN().Country(ip)
					city, _ = hr.GetGeoCN().City(ip)
					asn, _ = hr.GetGeoCN().ASN(ip)
					isp, _ = hr.GetGeoCN().ISP(ip)
					connectiontype, _ = hr.GetGeoCN().ConnectionType(ip)
					proxy, _ = hr.GetGeoCN().Proxy(ip)

				}

				// 只有当GeoCN.mmdb的字段为空时，才用其他库补充
				if country.Name == "" || country.ISO == "" {
					var fallbackCountry geo.Country
					if ip.To4() != nil && hr.GetCzdbV4() != nil {
						fallbackCountry, _ = hr.GetCzdbV4().Country(ip)
					} else if ip.To16() != nil && ip.To4() == nil && hr.GetCzdbV6() != nil {
						fallbackCountry, _ = hr.GetCzdbV6().Country(ip)
					} else if hr.GetQQWry() != nil {
						fallbackCountry, _ = hr.GetQQWry().Country(ip)
					}
					if fallbackCountry.Name == "" && hr.GetMaxmind() != nil {
						fallbackCountry, _ = hr.GetMaxmind().Country(ip)
					}

					if country.Name == "" && fallbackCountry.Name != "" {
						country.Name = fallbackCountry.Name
					}
					if country.ISO == "" && fallbackCountry.ISO != "" {
						country.ISO = fallbackCountry.ISO
					}
					if country.IsEU == nil && fallbackCountry.IsEU != nil {
						country.IsEU = fallbackCountry.IsEU
					}
				}

				// 补充城市信息
				if city.Name == "" || city.RegionName == "" || city.Latitude == 0 || city.Longitude == 0 {
					var fallbackCity geo.City
					if ip.To4() != nil && hr.GetCzdbV4() != nil {
						fallbackCity, _ = hr.GetCzdbV4().City(ip)
					} else if ip.To16() != nil && ip.To4() == nil && hr.GetCzdbV6() != nil {
						fallbackCity, _ = hr.GetCzdbV6().City(ip)
					} else if hr.GetQQWry() != nil {
						fallbackCity, _ = hr.GetQQWry().City(ip)
					}
					if (fallbackCity.Name == "" || fallbackCity.Latitude == 0) && hasMaxmindCity {
						if fallbackCity.Name == "" && maxmindCity.Name != "" {
							fallbackCity.Name = maxmindCity.Name
						}
						if fallbackCity.RegionName == "" && maxmindCity.RegionName != "" {
							fallbackCity.RegionName = maxmindCity.RegionName
						}
						if fallbackCity.Latitude == 0 && (maxmindCity.Latitude != 0 || maxmindCity.Longitude != 0) {
							fallbackCity.Latitude = maxmindCity.Latitude
							fallbackCity.Longitude = maxmindCity.Longitude
							fallbackCity.Timezone = maxmindCity.Timezone
						}
					}

					if city.Name == "" && fallbackCity.Name != "" {
						city.Name = fallbackCity.Name
					}
					if city.RegionName == "" && fallbackCity.RegionName != "" {
						city.RegionName = fallbackCity.RegionName
					}
					if city.RegionCode == "" && fallbackCity.RegionCode != "" {
						city.RegionCode = fallbackCity.RegionCode
					}
					if city.Latitude == 0 && fallbackCity.Latitude != 0 {
						city.Latitude = fallbackCity.Latitude
						city.Longitude = fallbackCity.Longitude
					}
					if city.Timezone == "" && fallbackCity.Timezone != "" {
						city.Timezone = fallbackCity.Timezone
					}
					if city.PostalCode == "" && fallbackCity.PostalCode != "" {
						city.PostalCode = fallbackCity.PostalCode
					}
				}

				if hasMaxmindCity {
					if maxmindCity.Latitude != 0 || maxmindCity.Longitude != 0 {
						city.Latitude = maxmindCity.Latitude
						city.Longitude = maxmindCity.Longitude
					}
					if maxmindCity.Timezone != "" {
						city.Timezone = maxmindCity.Timezone
					}
				}

				// 补充ISP信息
				if isp.ISP == "" || isp.Organization == "" || isp.ASN == 0 {
					var fallbackISP geo.ISP
					if ip.To4() != nil && hr.GetCzdbV4() != nil {
						fallbackISP, _ = hr.GetCzdbV4().ISP(ip)
					} else if ip.To16() != nil && ip.To4() == nil && hr.GetCzdbV6() != nil {
						fallbackISP, _ = hr.GetCzdbV6().ISP(ip)
					} else if hr.GetQQWry() != nil {
						fallbackISP, _ = hr.GetQQWry().ISP(ip)
					}
					if (fallbackISP.ISP == "" || fallbackISP.ASN == 0) && hr.GetMaxmind() != nil {
						maxmindISP, _ := hr.GetMaxmind().ISP(ip)
						if fallbackISP.ISP == "" && maxmindISP.ISP != "" {
							fallbackISP.ISP = maxmindISP.ISP
						}
						if fallbackISP.Organization == "" && maxmindISP.Organization != "" {
							fallbackISP.Organization = maxmindISP.Organization
						}
						if fallbackISP.ASN == 0 && maxmindISP.ASN != 0 {
							fallbackISP.ASN = maxmindISP.ASN
						}
						if fallbackISP.ORG == "" && maxmindISP.ORG != "" {
							fallbackISP.ORG = maxmindISP.ORG
						}
					}

					if isp.ISP == "" && fallbackISP.ISP != "" {
						isp.ISP = fallbackISP.ISP
					}
					if isp.Organization == "" && fallbackISP.Organization != "" {
						isp.Organization = fallbackISP.Organization
					}
					if isp.ASN == 0 && fallbackISP.ASN != 0 {
						isp.ASN = fallbackISP.ASN
					}
					if isp.ORG == "" && fallbackISP.ORG != "" {
						isp.ORG = fallbackISP.ORG
					}
				}

				// 补充ASN信息
				if asn.AutonomousSystemNumber == 0 || asn.AutonomousSystemOrganization == "" {
					var fallbackASN geo.ASN
					if ip.To4() != nil && hr.GetCzdbV4() != nil {
						fallbackASN, _ = hr.GetCzdbV4().ASN(ip)
					} else if ip.To16() != nil && ip.To4() == nil && hr.GetCzdbV6() != nil {
						fallbackASN, _ = hr.GetCzdbV6().ASN(ip)
					} else if hr.GetQQWry() != nil {
						fallbackASN, _ = hr.GetQQWry().ASN(ip)
					}
					if fallbackASN.AutonomousSystemNumber == 0 && hr.GetMaxmind() != nil {
						fallbackASN, _ = hr.GetMaxmind().ASN(ip)
					}

					if asn.AutonomousSystemNumber == 0 && fallbackASN.AutonomousSystemNumber != 0 {
						asn.AutonomousSystemNumber = fallbackASN.AutonomousSystemNumber
					}
					if asn.AutonomousSystemOrganization == "" && fallbackASN.AutonomousSystemOrganization != "" {
						asn.AutonomousSystemOrganization = fallbackASN.AutonomousSystemOrganization
					}
				}

				// 补充连接类型信息
				if connectiontype.ConnectionType == "" {
					var fallbackConnType geo.ConnectionType
					if ip.To4() != nil && hr.GetCzdbV4() != nil {
						fallbackConnType, _ = hr.GetCzdbV4().ConnectionType(ip)
					} else if ip.To16() != nil && ip.To4() == nil && hr.GetCzdbV6() != nil {
						fallbackConnType, _ = hr.GetCzdbV6().ConnectionType(ip)
					} else if hr.GetQQWry() != nil {
						fallbackConnType, _ = hr.GetQQWry().ConnectionType(ip)
					}
					if fallbackConnType.ConnectionType == "" && hr.GetMaxmind() != nil {
						fallbackConnType, _ = hr.GetMaxmind().ConnectionType(ip)
					}

					if connectiontype.ConnectionType == "" && fallbackConnType.ConnectionType != "" {
						connectiontype.ConnectionType = fallbackConnType.ConnectionType
					}
				}

			} else {
				// 非中国大陆IP：使用MaxMind为主
				if hr.GetMaxmind() != nil {
					country, _ = hr.GetMaxmind().Country(ip)
					city, _ = hr.GetMaxmind().City(ip)
					asn, _ = hr.GetMaxmind().ASN(ip)
					isp, _ = hr.GetMaxmind().ISP(ip)
					connectiontype, _ = hr.GetMaxmind().ConnectionType(ip)
					proxy, _ = hr.GetMaxmind().Proxy(ip)

				}

			}

			// 应用翻译（如果有lang参数且为中文）
			if lang == "zh" {
				country.Name = GetTranslatedCountryName(country.ISO, lang, country.Name)
				city.RegionName = GetTranslatedRegionName(country.ISO, city.RegionName, lang, city.RegionName)
				city.Name = GetTranslatedCityName(country.ISO, city.Name, lang, city.Name)
			}
		} else if lang == "en" {
			// 当 lang=en 时，只使用 MaxMind 库，不使用其他库补全
			if hr.GetMaxmind() != nil {
				country, _ = hr.GetMaxmind().Country(ip)
				city, _ = hr.GetMaxmind().City(ip)
				asn, _ = hr.GetMaxmind().ASN(ip)
				isp, _ = hr.GetMaxmind().ISP(ip)
				connectiontype, _ = hr.GetMaxmind().ConnectionType(ip)
				proxy, _ = hr.GetMaxmind().Proxy(ip)

				// 保留 MaxMind 原始结果，不做基于坐标的国家纠正，避免误判
			}

			// 用纯真库补全缺失的字段（如果有的话）
			var fallbackReader geo.Reader
			if hr.GetCzdbV4() != nil {
				fallbackReader = hr.GetCzdbV4()
			} else if hr.GetCzdbV6() != nil {
				fallbackReader = hr.GetCzdbV6()
			} else if hr.GetQQWry() != nil {
				fallbackReader = hr.GetQQWry()
			}

			if fallbackReader != nil {
				if isp.ISP == "" {
					fallbackISP, _ := fallbackReader.ISP(ip)
					if fallbackISP.ISP != "" {
						isp = fallbackISP
					}
				}
			}

		}
	} else {
		// 之前的中国大陆 IP 逻辑不变（当没有 lang 参数时）
		country, _ = s.gr.Country(ip)
		city, _ = s.gr.City(ip)
		asn, _ = s.gr.ASN(ip)
		isp, _ = s.gr.ISP(ip)
		connectiontype, _ = s.gr.ConnectionType(ip)
		proxy, _ = s.gr.Proxy(ip)

		// 如果有lang参数且为中文，应用翻译
		if lang == "zh" {
			country.Name = GetTranslatedCountryName(country.ISO, lang, country.Name)
			city.RegionName = GetTranslatedRegionName(country.ISO, city.RegionName, lang, city.RegionName)
			city.Name = GetTranslatedCityName(country.ISO, city.Name, lang, city.Name)
		}
		// 如果lang参数为其他值或空字符串，默认返回英文结果，不应用任何翻译
	}

	// 统一翻译策略：
	// - 若 lang=zh，强制中文翻译
	// - 若 lang=en，保持原文
	// - 若 lang 缺省或为其他值，且国家为 CN，则使用中文翻译；否则保持原文
	shouldTranslateZH := false
	switch strings.ToLower(lang) {
	case "zh":
		shouldTranslateZH = true
	case "en":
		shouldTranslateZH = false
	default:
		if strings.ToUpper(country.ISO) == "CN" {
			shouldTranslateZH = true
		}
	}
	if shouldTranslateZH {
		country.Name = GetTranslatedCountryName(country.ISO, "zh", country.Name)
		city.RegionName = GetTranslatedRegionName(country.ISO, city.RegionName, "zh", city.RegionName)
		city.Name = GetTranslatedCityName(country.ISO, city.Name, "zh", city.Name)
	}
	if n, err := s.gr.Network(ip); err == nil && n != nil {
		networkCIDR = n.String()
	}
	var hostname string
	if s.LookupAddr != nil {
		hostname, _ = s.LookupAddr(ip)
	}
	var autonomousSystemNumber string
	if asn.AutonomousSystemNumber > 0 {
		autonomousSystemNumber = fmt.Sprintf("AS%d", asn.AutonomousSystemNumber)
	}
	if asn.AutonomousSystemNumber == 0 && isp.ASN > 0 {
		autonomousSystemNumber = fmt.Sprintf("AS%d", isp.ASN)
	}
	if asn.AutonomousSystemOrganization == "" {
		asn.AutonomousSystemOrganization = isp.Organization
	}
	// ISP 字段回退逻辑：如果 ISP 为空，使用 org 字段内容
	if isp.ISP == "" || isp.ISP == "-" {
		if isp.Organization != "" && isp.Organization != "-" {
			isp.ISP = isp.Organization
		} else if asn.AutonomousSystemOrganization != "" && asn.AutonomousSystemOrganization != "-" {
			isp.ISP = asn.AutonomousSystemOrganization
		}
	}
	var ispASN string
	if isp.ASN > 0 {
		ispASN = fmt.Sprintf("AS%d", isp.ASN)
	}
	// 清理代理字段，避免显示 "-"
	proxyType := cleanProxyField(proxy.ProxyType)
	domain := cleanProxyField(proxy.Domain)
	usageType := cleanProxyField(proxy.UsageType)
	lastSeen := cleanProxyField(proxy.LastSeen)
	threat := cleanProxyField(proxy.Threat)
	provider := cleanProxyField(proxy.Provider)
	fraudScore := cleanProxyField(proxy.FraudScore)

	response = Response{
		IP:             ip,
		IPDecimal:      ipDecimal,
		Country:        country.Name,
		CountryCode:    country.ISO,
		CountryEU:      country.IsEU,
		RegionName:     city.RegionName,
		RegionCode:     city.RegionCode,
		MetroCode:      city.MetroCode,
		PostalCode:     city.PostalCode,
		City:           city.Name,
		District:       city.District,
		Latitude:       city.Latitude,
		Longitude:      city.Longitude,
		Timezone:       city.Timezone,
		ASN:            autonomousSystemNumber,
		IN:             ispASN,
		ISPO:           isp.ORG,
		IO:             isp.Organization,
		ISP:            isp.ISP,
		ORG:            asn.AutonomousSystemOrganization,
		ConnectionType: connectiontype.ConnectionType,
		Network:        networkCIDR,
		ProxyType:      proxyType,
		Domain:         domain,
		UsageType:      usageType,
		LastSeen:       lastSeen,
		Threat:         threat,
		Provider:       provider,
		FraudScore:     fraudScore,
		Hostname:       hostname,
		UserAgent:      userAgentFromRequest(r),
	}
	s.cache.SetWithLang(ip, lang, response)
	return response, nil
}

func (s *Server) newPortResponse(r *http.Request) (PortResponse, error) {
	lastElement := filepath.Base(r.URL.Path)
	port, err := strconv.ParseUint(lastElement, 10, 16)
	if err != nil || port < 1 || port > 65535 {
		return PortResponse{Port: port}, fmt.Errorf("invalid port: %s", lastElement)
	}
	ip, err := ipFromRequest(s.IPHeaders, r, s.AllowCustomIP)
	if err != nil {
		return PortResponse{Port: port}, err
	}
	err = s.LookupPort(ip, port)
	return PortResponse{
		IP:        ip,
		Port:      port,
		Reachable: err == nil,
	}, nil
}

func (s *Server) CLIUAHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintln(w, response.UserAgent)
	return nil
}

func (s *Server) CLIHandler(w http.ResponseWriter, r *http.Request) *appError {
	ip, err := ipFromRequest(s.IPHeaders, r, true)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintln(w, ip.String())
	return nil
}

func (s *Server) CLICountryHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintln(w, response.Country)
	return nil
}

func (s *Server) CLICountryCodeHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintln(w, response.CountryCode)
	return nil
}

func (s *Server) CLICityHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintln(w, response.City)
	return nil
}

func (s *Server) CLICoordinatesHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s,%s\n", formatCoordinate(response.Latitude), formatCoordinate(response.Longitude))
	return nil
}

func (s *Server) CLIASNHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.ASN)
	return nil
}

func (s *Server) CLIISPHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.ISP)
	return nil
}

func (s *Server) CLIORGHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.ORG)
	return nil
}

func (s *Server) CLIConnHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.ConnectionType)
	return nil
}

func (s *Server) CLINetworkHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.Network)
	return nil
}

func (s *Server) CLIProxyTypeHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.ProxyType)
	return nil
}

func (s *Server) CLIDomainHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.Domain)
	return nil
}

func (s *Server) CLIUsageTypeHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.UsageType)
	return nil
}

func (s *Server) CLILastSeenHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.LastSeen)
	return nil
}

func (s *Server) CLIThreatHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.Threat)
	return nil
}

func (s *Server) CLIProviderHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.Provider)
	return nil
}

func (s *Server) CLIFraudScoreHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	fmt.Fprintf(w, "%s\n", response.FraudScore)
	return nil
}

func (s *Server) JSONHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	b, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return internalServerError(err).AsJSON()
	}
	w.Header().Set("Content-Type", jsonMediaType)
	w.Write(b)
	return nil
}

func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) *appError {
	w.Header().Set("Content-Type", jsonMediaType)
	w.Write([]byte(`{"status":"OK"}`))
	return nil
}

func (s *Server) PortHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newPortResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	b, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return internalServerError(err).AsJSON()
	}
	w.Header().Set("Content-Type", jsonMediaType)
	w.Write(b)
	return nil
}

func (s *Server) cacheResizeHandler(w http.ResponseWriter, r *http.Request) *appError {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	capacity, err := strconv.Atoi(string(body))
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	if err := s.cache.Resize(capacity); err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	data := struct {
		Message string `json:"message"`
	}{fmt.Sprintf("Changed cache capacity to %d.", capacity)}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return internalServerError(err).AsJSON()
	}
	w.Header().Set("Content-Type", jsonMediaType)
	w.Write(b)
	return nil
}

func (s *Server) cacheHandler(w http.ResponseWriter, r *http.Request) *appError {
	cacheStats := s.cache.Stats()
	var data = struct {
		Size      int    `json:"size"`
		Capacity  int    `json:"capacity"`
		Evictions uint64 `json:"evictions"`
	}{
		cacheStats.Size,
		cacheStats.Capacity,
		cacheStats.Evictions,
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return internalServerError(err).AsJSON()
	}
	w.Header().Set("Content-Type", jsonMediaType)
	w.Write(b)
	return nil
}

func (s *Server) StaticFileHandler(w http.ResponseWriter, r *http.Request) *appError {
	// 获取请求的文件名
	filename := r.URL.Path[1:] // 移除开头的 "/"
	if filename == "" {
		return notFound(nil)
	}

	// 安全检查：防止路径遍历攻击
	if strings.Contains(filename, "..") {
		return notFound(nil)
	}

	// 构建文件路径
	filePath := filepath.Join(s.Template, filename)

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return notFound(nil)
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return notFound(nil)
	}

	// 确保不是目录
	if fileInfo.IsDir() {
		return notFound(nil)
	}

	// 设置适当的 Content-Type
	switch filepath.Ext(filename) {
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// 设置缓存头
	w.Header().Set("Cache-Control", "public, max-age=86400") // 缓存1天

	// 直接提供文件内容
	http.ServeContent(w, r, filename, fileInfo.ModTime(), file)
	return nil
}

func (s *Server) DefaultHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error())
	}
	t, err := template.ParseFiles(s.Template+"/index.html", s.Template+"/script.html", s.Template+"/styles.html")
	if err != nil {
		return internalServerError(err)
	}
	json, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return internalServerError(err)
	}

	var data = struct {
		Response
		Host         string
		BoxLatTop    float64
		BoxLatBottom float64
		BoxLonLeft   float64
		BoxLonRight  float64
		JSON         string
		Port         bool
	}{
		response,
		r.Host,
		response.Latitude + 0.05,
		response.Latitude - 0.05,
		response.Longitude - 0.05,
		response.Longitude + 0.05,
		string(json),
		s.LookupPort != nil,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "index.html", &data); err != nil {
		return internalServerError(err)
	}
	return nil
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) *appError {
	err := notFound(nil).WithMessage("404 page not found")
	if r.Header.Get("accept") == jsonMediaType {
		err = err.AsJSON()
	}
	return err
}

func Parse(s string) UA {
	parts := strings.SplitN(s, "/", 2)
	return UA{
		Product: parts[0],
	}
}

func cliMatcher(r *http.Request) bool {
	ua := Parse(r.UserAgent())
	switch ua.Product {
	case "curl", "HTTPie", "httpie-go", "Wget", "fetch libfetch", "Go", "Go-http-client", "ddclient", "Mikrotik", "xh":
		return true
	}
	return false
}

type appHandler func(http.ResponseWriter, *http.Request) *appError

func wrapHandlerFunc(f http.HandlerFunc) appHandler {
	return func(w http.ResponseWriter, r *http.Request) *appError {
		f.ServeHTTP(w, r)
		return nil
	}
}

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// For HEAD requests, wrap the writer to suppress the response body
	if r.Method == "HEAD" {
		w = &headResponseWriter{w}
	}
	if e := fn(w, r); e != nil { // e is *appError
		if e.Code/100 == 5 {
			log.Println(e.Error)
		}
		// When Content-Type for error is JSON, we need to marshal the response into JSON
		if e.IsJSON() {
			var data = struct {
				Code  int    `json:"status"`
				Error string `json:"error"`
			}{e.Code, e.Message}
			b, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				panic(err)
			}
			e.Message = string(b)
		}
		// Set Content-Type of response if set in error
		if e.ContentType != "" {
			w.Header().Set("Content-Type", e.ContentType)
		}
		w.WriteHeader(e.Code)
		fmt.Fprint(w, e.Message)
	}
}

func (s *Server) Handler() http.Handler {
	r := NewRouter()

	// Health
	r.Route("GET", "/health", s.HealthHandler)

	// JSON
	r.Route("GET", "/", s.JSONHandler).Header("Accept", jsonMediaType)
	r.Route("GET", "/json", s.JSONHandler)

	// CLI
	r.Route("GET", "/", s.CLIHandler).MatcherFunc(cliMatcher)
	r.Route("GET", "/", s.CLIHandler).Header("Accept", textMediaType)
	r.Route("GET", "/ip", s.CLIHandler)
	if !s.gr.IsEmpty() {
		r.Route("GET", "/country", s.CLICountryHandler)
		r.Route("GET", "/country_code", s.CLICountryCodeHandler)
		r.Route("GET", "/city", s.CLICityHandler)
		r.Route("GET", "/coordinates", s.CLICoordinatesHandler)
		r.Route("GET", "/asn", s.CLIASNHandler)
		r.Route("GET", "/isp", s.CLIISPHandler)
		r.Route("GET", "/org", s.CLIORGHandler)
		r.Route("GET", "/connection_type", s.CLIConnHandler)
		r.Route("GET", "/network", s.CLINetworkHandler)
		r.Route("GET", "/proxy_type", s.CLIProxyTypeHandler)
		r.Route("GET", "/domain", s.CLIDomainHandler)
		r.Route("GET", "/usage_type", s.CLIUsageTypeHandler)
		r.Route("GET", "/last_seen", s.CLILastSeenHandler)
		r.Route("GET", "/threat", s.CLIThreatHandler)
		r.Route("GET", "/provider", s.CLIProviderHandler)
		r.Route("GET", "/fraud_score", s.CLIFraudScoreHandler)
		r.Route("GET", "/ua", s.CLIUAHandler)
	}

	// Static files
	if s.Template != "" {
		r.Route("GET", "/favicon.ico", s.StaticFileHandler)
	}

	// Browser
	if s.Template != "" {
		r.Route("GET", "/", s.DefaultHandler)
	}

	// Port testing
	if s.LookupPort != nil {
		r.RoutePrefix("GET", "/port/", s.PortHandler)
	}

	// Profiling
	if s.profile {
		r.Route("POST", "/debug/cache/resize", s.cacheResizeHandler)
		r.Route("GET", "/debug/cache/", s.cacheHandler)
		r.Route("GET", "/debug/pprof/cmdline", wrapHandlerFunc(pprof.Cmdline))
		r.Route("GET", "/debug/pprof/profile", wrapHandlerFunc(pprof.Profile))
		r.Route("GET", "/debug/pprof/symbol", wrapHandlerFunc(pprof.Symbol))
		r.Route("GET", "/debug/pprof/trace", wrapHandlerFunc(pprof.Trace))
		r.RoutePrefix("GET", "/debug/pprof/", wrapHandlerFunc(pprof.Index))
	}

	return r.Handler()
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (s *Server) ListenAndServe(addr string) error {
	handler := s.Handler()
	logger := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{w, http.StatusOK}
		
		// Get real IP if proxy headers are trusted
		clientIP := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			clientIP = strings.Split(xff, ",")[0]
		} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			clientIP = realIP
		}

		handler.ServeHTTP(lrw, r)
		
		// Only log API and page requests, skip favicon to reduce noise
		if r.URL.Path != "/favicon.ico" {
			log.Printf("[ACCESS] %s | %s %s | %d | %v", clientIP, r.Method, r.URL.Path, lrw.statusCode, time.Since(start))
		}
	})
	return http.ListenAndServe(addr, logger)
}

func formatCoordinate(c float64) string {
	return strconv.FormatFloat(c, 'f', 6, 64)
}

// validateCountryByCoordinates 根据坐标验证国家代码是否合理
// 坐标-国家纠错逻辑已移除，避免跨库数据不一致导致误修正

var (
	builtinChineseCountries = map[string]string{
		"US": "美国",
		"GB": "英国",
		"UK": "英国",
		"CN": "中国",
		"JP": "日本",
		"KR": "韩国",
		"DE": "德国",
		"FR": "法国",
		"CA": "加拿大",
		"AU": "澳大利亚",
		"RU": "俄罗斯",
		"IN": "印度",
		"BR": "巴西",
		"IT": "意大利",
		"ES": "西班牙",
		"NL": "荷兰",
		"SE": "瑞典",
		"NO": "挪威",
		"DK": "丹麦",
		"FI": "芬兰",
		"CH": "瑞士",
		"AT": "奥地利",
		"BE": "比利时",
		"IE": "爱尔兰",
		"PT": "葡萄牙",
		"GR": "希腊",
		"PL": "波兰",
		"CZ": "捷克",
		"HU": "匈牙利",
		"RO": "罗马尼亚",
		"BG": "保加利亚",
		"HR": "克罗地亚",
		"SI": "斯洛文尼亚",
		"SK": "斯洛伐克",
		"LT": "立陶宛",
		"LV": "拉脱维亚",
		"EE": "爱沙尼亚",
		"TH": "泰国",
		"VN": "越南",
		"MY": "马来西亚",
		"SG": "新加坡",
		"ID": "印度尼西亚",
		"PH": "菲律宾",
		"TW": "台湾",
		"HK": "香港",
		"MO": "澳门",
	}

	builtinUSStates = map[string]string{
		"California":     "加利福尼亚州",
		"New York":       "纽约州",
		"Texas":          "德克萨斯州",
		"Florida":        "佛罗里达州",
		"Illinois":       "伊利诺伊州",
		"Pennsylvania":   "宾夕法尼亚州",
		"Ohio":           "俄亥俄州",
		"Georgia":        "佐治亚州",
		"North Carolina": "北卡罗来纳州",
		"Michigan":       "密歇根州",
		"New Jersey":     "新泽西州",
		"Virginia":       "弗吉尼亚州",
		"Washington":     "华盛顿州",
		"Arizona":        "亚利桑那州",
		"Massachusetts":  "马萨诸塞州",
		"Tennessee":      "田纳西州",
		"Indiana":        "印第安纳州",
		"Missouri":       "密苏里州",
		"Maryland":       "马里兰州",
		"Wisconsin":      "威斯康星州",
	}

	builtinCAProvinces = map[string]string{
		"Ontario":                   "安大略省",
		"Quebec":                    "魁北克省",
		"British Columbia":          "不列颠哥伦比亚省",
		"Alberta":                   "阿尔伯塔省",
		"Manitoba":                  "马尼托巴省",
		"Saskatchewan":              "萨斯喀彻温省",
		"Nova Scotia":               "新斯科舍省",
		"New Brunswick":             "新不伦瑞克省",
		"Newfoundland and Labrador": "纽芬兰和拉布拉多省",
		"Prince Edward Island":      "爱德华王子岛省",
	}

	builtinUSCities = map[string]string{
		"New York":         "纽约",
		"Los Angeles":      "洛杉矶",
		"Chicago":          "芝加哥",
		"Houston":          "休斯顿",
		"Phoenix":          "凤凰城",
		"Philadelphia":     "费城",
		"San Antonio":      "圣安东尼奥",
		"San Diego":        "圣地亚哥",
		"Dallas":           "达拉斯",
		"San Jose":         "圣何塞",
		"Austin":           "奥斯汀",
		"Jacksonville":     "杰克逊维尔",
		"San Francisco":    "旧金山",
		"Columbus":         "哥伦布",
		"Charlotte":        "夏洛特",
		"Fort Worth":       "沃思堡",
		"Indianapolis":     "印第安纳波利斯",
		"Seattle":          "西雅图",
		"Denver":           "丹佛",
		"Boston":           "波士顿",
		"El Paso":          "埃尔帕索",
		"Detroit":          "底特律",
		"Nashville":        "纳什维尔",
		"Portland":         "波特兰",
		"Memphis":          "孟菲斯",
		"Oklahoma City":    "俄克拉荷马城",
		"Las Vegas":        "拉斯维加斯",
		"Louisville":       "路易斯维尔",
		"Baltimore":        "巴尔的摩",
		"Milwaukee":        "密尔沃基",
		"Albuquerque":      "阿尔伯克基",
		"Tucson":           "图森",
		"Fresno":           "弗雷斯诺",
		"Mesa":             "梅萨",
		"Sacramento":       "萨克拉门托",
		"Atlanta":          "亚特兰大",
		"Kansas City":      "堪萨斯城",
		"Colorado Springs": "科罗拉多斯普林斯",
		"Miami":            "迈阿密",
		"Raleigh":          "罗利",
		"Omaha":            "奥马哈",
		"Long Beach":       "长滩",
		"Virginia Beach":   "弗吉尼亚海滩",
		"Oakland":          "奥克兰",
		"Minneapolis":      "明尼阿波利斯",
		"Tampa":            "坦帕",
		"Tulsa":            "塔尔萨",
		"Arlington":        "阿灵顿",
		"New Orleans":      "新奥尔良",
	}

	builtinGlobalCities = map[string]string{
		"London":           "伦敦",
		"Paris":            "巴黎",
		"Tokyo":            "东京",
		"Seoul":            "首尔",
		"Sydney":           "悉尼",
		"Melbourne":        "墨尔本",
		"Toronto":          "多伦多",
		"Vancouver":        "温哥华",
		"Montreal":         "蒙特利尔",
		"Berlin":           "柏林",
		"Munich":           "慕尼黑",
		"Frankfurt":        "法兰克福",
		"Hamburg":          "汉堡",
		"Rome":             "罗马",
		"Milan":            "米兰",
		"Madrid":           "马德里",
		"Barcelona":        "巴塞罗那",
		"Amsterdam":        "阿姆斯特丹",
		"Stockholm":        "斯德哥尔摩",
		"Oslo":             "奥斯陆",
		"Copenhagen":       "哥本哈根",
		"Helsinki":         "赫尔辛基",
		"Zurich":           "苏黎世",
		"Geneva":           "日内瓦",
		"Vienna":           "维也纳",
		"Brussels":         "布鲁塞尔",
		"Dublin":           "都柏林",
		"Lisbon":           "里斯本",
		"Athens":           "雅典",
		"Warsaw":           "华沙",
		"Prague":           "布拉格",
		"Budapest":         "布达佩斯",
		"Bucharest":        "布加勒斯特",
		"Sofia":            "索菲亚",
		"Zagreb":           "萨格勒布",
		"Ljubljana":        "卢布尔雅那",
		"Bratislava":       "布拉迪斯拉发",
		"Vilnius":          "维尔纽斯",
		"Riga":             "里加",
		"Tallinn":          "塔林",
		"Bangkok":          "曼谷",
		"Ho Chi Minh City": "胡志明市",
		"Kuala Lumpur":     "吉隆坡",
		"Singapore":        "新加坡",
		"Jakarta":          "雅加达",
		"Manila":           "马尼拉",
	}
)

// getChineseCountryName 返回国家的中文名称，如果没有对应的中文名则返回英文名
func getChineseCountryName(countryCode, englishName string) string {
	if chineseName, exists := builtinChineseCountries[countryCode]; exists {
		return chineseName
	}
	return englishName
}

// getChineseRegionName 返回地区的中文名称
func getChineseRegionName(countryCode, regionName string) string {
	if countryCode == "US" {
		if chinese, exists := builtinUSStates[regionName]; exists {
			return chinese
		}
	} else if countryCode == "CA" {
		if chinese, exists := builtinCAProvinces[regionName]; exists {
			return chinese
		}
	}
	return regionName
}

// getChineseCityName 返回城市的中文名称
func getChineseCityName(countryCode, cityName string) string {
	if countryCode == "US" {
		if chinese, exists := builtinUSCities[cityName]; exists {
			return chinese
		}
	}
	if chinese, exists := builtinGlobalCities[cityName]; exists {
		return chinese
	}
	return cityName
}

// cleanProxyField 清理代理字段，只要有内容就显示
func cleanProxyField(field string) string {
	// 只有当字段为 "-" 或空字符串时才返回空字符串（触发 omitempty）
	// 其他任何内容都显示
	if field == "-" || field == "" {
		return ""
	}
	return field
}
