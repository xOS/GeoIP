package http

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

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
	Template   string
	IPHeaders  []string
	LookupAddr func(net.IP) (string, error)
	LookupPort func(net.IP, uint64) error
	cache      *Cache
	gr         geo.Reader
	profile    bool
	Sponsor    bool
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
	Street         string   `json:"street,omitempty"`
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
	IsProxy        bool     `json:"is_proxy"`
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

func New(db geo.Reader, cache *Cache, profile bool) *Server {
	return &Server{cache: cache, gr: db, profile: profile}
}

func ipFromForwardedForHeader(v string) string {
	sep := strings.Index(v, ",")
	if sep == -1 {
		return v
	}
	return v[:sep]
}

// ipFromRequest detects the IP address for this transaction.
//
// * `headers` - the specific HTTP headers to trust
// * `r` - the incoming HTTP request
// * `customIP` - whether to allow the IP to be pulled from query parameters
func ipFromRequest(headers []string, r *http.Request, customIP bool) (net.IP, error) {
	remoteIP := ""
	if customIP && r.URL != nil {
		if v, ok := r.URL.Query()["ip"]; ok {
			remoteIP = v[0]
		}
	}
	if remoteIP == "" {
		for _, header := range headers {
			remoteIP = r.Header.Get(header)
			if http.CanonicalHeaderKey(header) == "X-Forwarded-For" {
				remoteIP = ipFromForwardedForHeader(remoteIP)
			}
			if remoteIP != "" {
				break
			}
		}
	}
	if remoteIP == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return nil, err
		}
		remoteIP = host
	}
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return nil, fmt.Errorf("could not parse IP: %s", remoteIP)
	}
	return ip, nil
}

func userAgentFromRequest(r *http.Request) string {
	userAgentRaw := r.UserAgent()
	return userAgentRaw
}

func (s *Server) newResponse(r *http.Request) (Response, error) {
	ip, err := ipFromRequest(s.IPHeaders, r, true)
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
	if hr, ok := s.gr.(interface {
		GetMaxmind() geo.Reader
		GetCzdbV4() geo.Reader
		GetCzdbV6() geo.Reader
		GetQQWry() geo.Reader
	}); ok && (lang == "zh" || lang == "en") {
		// 使用 lang 参数时，优先使用指定库，然后用其他库补全缺失字段
		if lang == "zh" {
			// 当 lang=zh 时，优先使用纯真新库，但要确保数据一致性
			// 根据IP类型选择合适的主要数据库
			var primaryReader geo.Reader
			if ip.To4() != nil {
				// IPv4地址：优先级 CzdbV4 > QQWry
				if hr.GetCzdbV4() != nil {
					primaryReader = hr.GetCzdbV4()
				} else if hr.GetQQWry() != nil {
					primaryReader = hr.GetQQWry()
				}
			} else {
				// IPv6地址：优先级 CzdbV6 > CzdbV4 > QQWry
				if hr.GetCzdbV6() != nil {
					primaryReader = hr.GetCzdbV6()
				} else if hr.GetCzdbV4() != nil {
					primaryReader = hr.GetCzdbV4()
				} else if hr.GetQQWry() != nil {
					primaryReader = hr.GetQQWry()
				}
			}

			// 同时获取 MaxMind 数据用于对比和补全
			var maxmindCountry geo.Country
			var maxmindCity geo.City
			var maxmindASN geo.ASN
			var maxmindISP geo.ISP
			var maxmindConnectionType geo.ConnectionType

			if hr.GetMaxmind() != nil {
				maxmindCountry, _ = hr.GetMaxmind().Country(ip)
				maxmindCity, _ = hr.GetMaxmind().City(ip)
				maxmindASN, _ = hr.GetMaxmind().ASN(ip)
				maxmindISP, _ = hr.GetMaxmind().ISP(ip)
				maxmindConnectionType, _ = hr.GetMaxmind().ConnectionType(ip)

			}

			// 从主要数据库获取数据
			if primaryReader != nil {
				country, _ = primaryReader.Country(ip)
				city, _ = primaryReader.City(ip)
				asn, _ = primaryReader.ASN(ip)
				isp, _ = primaryReader.ISP(ip)
				connectiontype, _ = primaryReader.ConnectionType(ip)
				proxy, _ = primaryReader.Proxy(ip)
			}

			// 确保从MaxMind获取完整的字段信息（如果主要数据库没有的话）
			if asn.AutonomousSystemNumber == 0 && maxmindASN.AutonomousSystemNumber != 0 {
				asn = maxmindASN
			}
			// 合并ISP信息，优先保留纯真库的中文信息
			// 只有当纯真库完全没有ISP信息时，才使用MaxMind的信息
			if isp.ISP == "" && maxmindISP.ISP != "" {
				isp = maxmindISP
			} else {
				// 如果纯真库有ISP信息，只补充缺失的技术字段
				if isp.ASN == 0 && maxmindISP.ASN != 0 {
					isp.ASN = maxmindISP.ASN
				}
				// 补充缺失的组织信息字段（即使纯真库有ISP名称）
				if isp.ORG == "" && maxmindISP.ORG != "" {
					isp.ORG = maxmindISP.ORG
				}
				if isp.Organization == "" && maxmindISP.Organization != "" {
					isp.Organization = maxmindISP.Organization
				}
			}
			if connectiontype.ConnectionType == "" && maxmindConnectionType.ConnectionType != "" {
				connectiontype = maxmindConnectionType
			}

			// 数据一致性检查：如果纯真库和 MaxMind 的国家判断差异很大，优先相信 MaxMind 的地理位置
			if maxmindCountry.ISO != "" && country.ISO != maxmindCountry.ISO {

				// 检测到国家代码冲突，为了确保数据一致性，完全使用 MaxMind 的地理信息
				// 但保留纯真库的 ISP 等技术信息（如果更详细的话）

				// 保存纯真库的数据
				czdbCountryName := country.Name
				czdbCountryISO := country.ISO // 保存纯真库的原始国家代码
				czdbCityName := city.Name
				czdbRegionName := city.RegionName
				czdbStreet := city.Street
				czdbISP := isp
				czdbASN := asn
				czdbConnectionType := connectiontype
				czdbProxy := proxy

				// 使用 MaxMind 的完整地理信息确保一致性
				country = maxmindCountry
				city = maxmindCity

				// 重要：为用户提供翻译后的地理信息
				if lang != "" {
					country.Name = GetTranslatedCountryName(country.ISO, lang, country.Name)
					city.RegionName = GetTranslatedRegionName(country.ISO, city.RegionName, lang, city.RegionName)
					city.Name = GetTranslatedCityName(country.ISO, city.Name, lang, city.Name)
				}

				// 验证国家代码与坐标的一致性
				// 当遇到地区识别冲突时，地区识别以 MaxMind 库为准
				if !validateCountryByCoordinates(country.ISO, city.Latitude, city.Longitude) {
					// 坐标与国家代码不匹配，完全使用 MaxMind 的地理信息
					// 但对于 lang=zh，尝试提供中文国家名
					country.ISO = maxmindCountry.ISO
					country.IsEU = maxmindCountry.IsEU

					if lang != "" {
						// 为用户提供翻译后的地理信息
						country.Name = GetTranslatedCountryName(maxmindCountry.ISO, lang, maxmindCountry.Name)
						city.RegionName = GetTranslatedRegionName(maxmindCountry.ISO, city.RegionName, lang, city.RegionName)
						city.Name = GetTranslatedCityName(maxmindCountry.ISO, city.Name, lang, city.Name)
					} else {
						country.Name = maxmindCountry.Name
					}

					// 完全使用 MaxMind 的地理信息确保一致性
					city = maxmindCity
				}

				// 只有在没有地理冲突且国家判断一致的情况下，才恢复纯真库的中文地名
				// 检查纯真库和 MaxMind 的国家判断是否一致
				// 使用纯真库的原始国家代码进行比较
				czdbCountryMatches := (czdbCountryISO == maxmindCountry.ISO) ||
					(czdbCountryName != "" && GetTranslatedCountryName(maxmindCountry.ISO, "zh", "") == czdbCountryName)

				if lang == "zh" && validateCountryByCoordinates(country.ISO, city.Latitude, city.Longitude) && czdbCountryMatches {
					// 地理位置一致且国家判断一致，可以安全地使用纯真库的中文地名
					if czdbCountryName != "" && czdbCountryName != country.Name {
						country.Name = czdbCountryName
					}
					if czdbCityName != "" && czdbCityName != city.Name {
						city.Name = czdbCityName
					}
					if czdbRegionName != "" && czdbRegionName != city.RegionName {
						city.RegionName = czdbRegionName
					}
					if czdbStreet != "" && czdbStreet != city.Street {
						city.Street = czdbStreet
					}
				} else if lang == "zh" {
					// 有冲突时，使用 MaxMind 的数据但提供翻译
					country.Name = GetTranslatedCountryName(country.ISO, lang, country.Name)
					city.RegionName = GetTranslatedRegionName(country.ISO, city.RegionName, lang, city.RegionName)
					city.Name = GetTranslatedCityName(country.ISO, city.Name, lang, city.Name)
				}

				// 恢复技术信息：优先使用纯真库的详细信息，如果没有则使用 MaxMind 的
				if czdbISP.ISP != "" {
					isp = czdbISP
				} else {
					isp = maxmindISP
				}

				if czdbASN.AutonomousSystemNumber != 0 {
					asn = czdbASN
				} else {
					asn = maxmindASN
				}

				if czdbConnectionType.ConnectionType != "" {
					connectiontype = czdbConnectionType
				} else {
					connectiontype = maxmindConnectionType
				}

				proxy = czdbProxy // 代理信息优先使用纯真库的
			}

			// 补全缺失的技术信息（ASN、ISP等）
			if asn.AutonomousSystemNumber == 0 && maxmindASN.AutonomousSystemNumber != 0 {
				asn = maxmindASN
			}
			if isp.ISP == "" && maxmindISP.ISP != "" {
				isp = maxmindISP
			}
			if connectiontype.ConnectionType == "" && maxmindConnectionType.ConnectionType != "" {
				connectiontype = maxmindConnectionType
			}

			// 补充纯真库缺失的地理信息字段
			if city.RegionName == "" && maxmindCity.RegionName != "" {
				city.RegionName = maxmindCity.RegionName
			}
			if city.Name == "" && maxmindCity.Name != "" {
				city.Name = maxmindCity.Name
			}
			if city.PostalCode == "" && maxmindCity.PostalCode != "" {
				city.PostalCode = maxmindCity.PostalCode
			}

			// 如果纯真库没有地理坐标，使用 MaxMind 的
			if city.Latitude == 0 && city.Longitude == 0 && maxmindCity.Latitude != 0 && maxmindCity.Longitude != 0 {
				city.Latitude = maxmindCity.Latitude
				city.Longitude = maxmindCity.Longitude
				city.Timezone = maxmindCity.Timezone
			}

			// 补全MaxMind特有的字段
			if country.IsEU == nil && maxmindCountry.IsEU != nil {
				country.IsEU = maxmindCountry.IsEU
			}
			if city.RegionCode == "" && maxmindCity.RegionCode != "" {
				city.RegionCode = maxmindCity.RegionCode
			}

			// 没有冲突时，也需要应用翻译（如果有lang参数）
			if lang != "" {
				country.Name = GetTranslatedCountryName(country.ISO, lang, country.Name)
				city.RegionName = GetTranslatedRegionName(country.ISO, city.RegionName, lang, city.RegionName)
				city.Name = GetTranslatedCityName(country.ISO, city.Name, lang, city.Name)
			}
		} else if lang == "en" {
			// 当 lang=en 时，优先使用 MaxMind 库，然后用纯真库补全
			if hr.GetMaxmind() != nil {
				country, _ = hr.GetMaxmind().Country(ip)
				city, _ = hr.GetMaxmind().City(ip)
				asn, _ = hr.GetMaxmind().ASN(ip)
				isp, _ = hr.GetMaxmind().ISP(ip)
				connectiontype, _ = hr.GetMaxmind().ConnectionType(ip)
				proxy, _ = hr.GetMaxmind().Proxy(ip)

				// 验证国家代码与坐标的一致性
				if !validateCountryByCoordinates(country.ISO, city.Latitude, city.Longitude) {
					// 坐标与国家代码不匹配，使用坐标推断正确的国家
					correctedISO, correctedName := getCountryByCoordinates(city.Latitude, city.Longitude, lang)
					if correctedISO != "" {
						country.ISO = correctedISO
						country.Name = correctedName
					}
				}
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

		// 如果有lang参数，应用翻译
		if lang != "" {
			country.Name = GetTranslatedCountryName(country.ISO, lang, country.Name)
			city.RegionName = GetTranslatedRegionName(country.ISO, city.RegionName, lang, city.RegionName)
			city.Name = GetTranslatedCityName(country.ISO, city.Name, lang, city.Name)
		}
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
		Street:         city.Street,
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
		IsProxy:        proxy.IsProxy,
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
	ip, err := ipFromRequest(s.IPHeaders, r, false)
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

func (s *Server) CLIProxyHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error()).AsJSON()
	}
	if response.IsProxy {
		fmt.Fprintln(w, "true")
	} else {
		fmt.Fprintln(w, "false")
	}
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

func (s *Server) DefaultHandler(w http.ResponseWriter, r *http.Request) *appError {
	response, err := s.newResponse(r)
	if err != nil {
		return badRequest(err).WithMessage(err.Error())
	}
	t, err := template.ParseGlob(s.Template + "/*")
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
		Sponsor      bool
	}{
		response,
		r.Host,
		response.Latitude + 0.05,
		response.Latitude - 0.05,
		response.Longitude - 0.05,
		response.Longitude + 0.05,
		string(json),
		s.LookupPort != nil,
		s.Sponsor,
	}
	if err := t.Execute(w, &data); err != nil {
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
		r.Route("GET", "/proxy", s.CLIProxyHandler)
		r.Route("GET", "/proxy_type", s.CLIProxyTypeHandler)
		r.Route("GET", "/domain", s.CLIDomainHandler)
		r.Route("GET", "/usage_type", s.CLIUsageTypeHandler)
		r.Route("GET", "/last_seen", s.CLILastSeenHandler)
		r.Route("GET", "/threat", s.CLIThreatHandler)
		r.Route("GET", "/provider", s.CLIProviderHandler)
		r.Route("GET", "/fraud_score", s.CLIFraudScoreHandler)
		r.Route("GET", "/ua", s.CLIUAHandler)
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

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

func formatCoordinate(c float64) string {
	return strconv.FormatFloat(c, 'f', 6, 64)
}

// validateCountryByCoordinates 根据坐标验证国家代码是否合理
func validateCountryByCoordinates(countryCode string, lat, lon float64) bool {
	if lat == 0 && lon == 0 {
		return true // 没有坐标信息，无法验证
	}

	// 定义一些主要国家的大致坐标范围
	countryBounds := map[string][4]float64{
		// [minLat, maxLat, minLon, maxLon]
		"US": {24.0, 71.0, -180.0, -66.0},  // 美国（包括阿拉斯加和夏威夷）
		"GB": {49.9, 60.9, -8.2, 1.8},      // 英国
		"UK": {49.9, 60.9, -8.2, 1.8},      // 英国
		"CN": {18.0, 54.0, 73.0, 135.0},    // 中国
		"CA": {41.7, 83.1, -141.0, -52.6},  // 加拿大
		"AU": {-43.6, -10.7, 113.3, 153.6}, // 澳大利亚
		"DE": {47.3, 55.1, 5.9, 15.0},      // 德国
		"FR": {41.3, 51.1, -5.1, 9.6},      // 法国
		"JP": {24.0, 46.0, 123.0, 146.0},   // 日本
		"KR": {33.0, 38.6, 124.6, 131.9},   // 韩国
		"RU": {41.2, 81.9, 19.6, -169.0},   // 俄罗斯（跨越180度经线）
		"IN": {6.7, 35.7, 68.0, 97.4},      // 印度
		"BR": {-33.8, 5.3, -74.0, -28.8},   // 巴西
		"IT": {35.5, 47.1, 6.6, 18.5},      // 意大利
		"ES": {27.6, 43.8, -18.2, 4.3},     // 西班牙
	}

	bounds, exists := countryBounds[countryCode]
	if !exists {
		return true // 没有定义边界，假设正确
	}

	minLat, maxLat, minLon, maxLon := bounds[0], bounds[1], bounds[2], bounds[3]

	// 特殊处理跨越180度经线的情况（如俄罗斯）
	if minLon > maxLon {
		return lat >= minLat && lat <= maxLat && (lon >= minLon || lon <= maxLon)
	}

	return lat >= minLat && lat <= maxLat && lon >= minLon && lon <= maxLon
}

// getCountryByCoordinates 根据坐标推断国家代码和名称
func getCountryByCoordinates(lat, lon float64, lang string) (string, string) {
	var countryCode, englishName string

	// 检查主要国家的坐标范围
	if lat >= 49.9 && lat <= 60.9 && lon >= -8.2 && lon <= 1.8 {
		// 英国范围
		countryCode, englishName = "GB", "United Kingdom"
	} else if lat >= 33.0 && lat <= 38.6 && lon >= 124.6 && lon <= 131.9 {
		// 韩国范围
		countryCode, englishName = "KR", "South Korea"
	} else if lat >= -43.6 && lat <= -10.7 && lon >= 113.3 && lon <= 153.6 {
		// 澳大利亚范围
		countryCode, englishName = "AU", "Australia"
	} else {
		// 默认返回空值，表示无法确定
		return "", ""
	}

	// 使用翻译管理器获取翻译后的名称
	if lang != "" {
		translatedName := GetTranslatedCountryName(countryCode, lang, englishName)
		return countryCode, translatedName
	}

	return countryCode, englishName
}

// getChineseCountryName 返回国家的中文名称，如果没有对应的中文名则返回英文名
func getChineseCountryName(countryCode, englishName string) string {
	chineseNames := map[string]string{
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

	if chineseName, exists := chineseNames[countryCode]; exists {
		return chineseName
	}
	return englishName
}

// getChineseRegionName 返回地区的中文名称
func getChineseRegionName(countryCode, regionName string) string {
	// 美国州名翻译
	if countryCode == "US" {
		usStates := map[string]string{
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
		if chinese, exists := usStates[regionName]; exists {
			return chinese
		}
	}

	// 加拿大省份翻译
	if countryCode == "CA" {
		caProvinces := map[string]string{
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
		if chinese, exists := caProvinces[regionName]; exists {
			return chinese
		}
	}

	return regionName
}

// getChineseCityName 返回城市的中文名称
func getChineseCityName(countryCode, cityName string) string {
	// 美国主要城市翻译
	if countryCode == "US" {
		usCities := map[string]string{
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
		if chinese, exists := usCities[cityName]; exists {
			return chinese
		}
	}

	// 其他国家主要城市翻译
	globalCities := map[string]string{
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
	if chinese, exists := globalCities[cityName]; exists {
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
