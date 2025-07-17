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
	response, ok := s.cache.Get(ip)
	if ok {
		// Do not cache user agent
		response.UserAgent = userAgentFromRequest(r)
		return response, nil
	}
	ipDecimal := iputil.ToDecimal(ip)
	country, _ := s.gr.Country(ip)
	city, _ := s.gr.City(ip)
	asn, _ := s.gr.ASN(ip)
	isp, _ := s.gr.ISP(ip)
	connectiontype, _ := s.gr.ConnectionType(ip)
	proxy, _ := s.gr.Proxy(ip)
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
	s.cache.Set(ip, response)
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

// cleanProxyField 清理代理字段，只要有内容就显示
func cleanProxyField(field string) string {
	// 只有当字段为 "-" 或空字符串时才返回空字符串（触发 omitempty）
	// 其他任何内容都显示
	if field == "-" || field == "" {
		return ""
	}
	return field
}
