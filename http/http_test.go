package http

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/xos/geoip/iputil/geo"
)

func lookupAddr(net.IP) (string, error) { return "localhost", nil }
func lookupPort(net.IP, uint64) error   { return nil }

type testDb struct{}

func (t *testDb) Country(net.IP) (geo.Country, error) {
	return geo.Country{Name: "Elbonia", ISO: "EB", IsEU: new(bool)}, nil
}

func (t *testDb) City(net.IP) (geo.City, error) {
	return geo.City{Name: "Bornyasherk", RegionName: "North Elbonia", RegionCode: "1234", MetroCode: 1234, PostalCode: "1234", Latitude: 63.416667, Longitude: 10.416667, Timezone: "Europe/Bornyasherk"}, nil
}

func (t *testDb) ASN(net.IP) (geo.ASN, error) {
	return geo.ASN{AutonomousSystemNumber: 59795, AutonomousSystemOrganization: "Hosting4Real"}, nil
}

func (t *testDb) ISP(net.IP) (geo.ISP, error) {
	return geo.ISP{ISP: "Hosting4Real"}, nil
}

func (t *testDb) ConnectionType(net.IP) (geo.ConnectionType, error) {
	return geo.ConnectionType{ConnectionType: "Corporate"}, nil
}

func (t *testDb) Proxy(net.IP) (geo.Proxy, error) {
	return geo.Proxy{
		IsProxy:     false,
		ProxyType:   "",
		Country:     "",
		CountryCode: "",
		Domain:      "",
		UsageType:   "",
		LastSeen:    "",
		Threat:      "",
		Provider:    "",
		FraudScore:  "",
	}, nil
}

func (t *testDb) IsEmpty() bool { return false }

// testHybridDb implements the interface needed for lang parameter testing
type testHybridDb struct {
	maxmind geo.Reader
	czdbV4  geo.Reader
	czdbV6  geo.Reader
	qqwry   geo.Reader
}

func (h *testHybridDb) GetMaxmind() geo.Reader { return h.maxmind }
func (h *testHybridDb) GetCzdbV4() geo.Reader  { return h.czdbV4 }
func (h *testHybridDb) GetCzdbV6() geo.Reader  { return h.czdbV6 }
func (h *testHybridDb) GetQQWry() geo.Reader   { return h.qqwry }

func (h *testHybridDb) Country(net.IP) (geo.Country, error) {
	return geo.Country{Name: "Default", ISO: "XX"}, nil
}

func (h *testHybridDb) City(net.IP) (geo.City, error) {
	return geo.City{Name: "Default City"}, nil
}

func (h *testHybridDb) ASN(net.IP) (geo.ASN, error) {
	return geo.ASN{AutonomousSystemNumber: 0}, nil
}

func (h *testHybridDb) ISP(net.IP) (geo.ISP, error) {
	return geo.ISP{ISP: "Default ISP"}, nil
}

func (h *testHybridDb) ConnectionType(net.IP) (geo.ConnectionType, error) {
	return geo.ConnectionType{ConnectionType: "Default"}, nil
}

func (h *testHybridDb) Proxy(net.IP) (geo.Proxy, error) {
	return geo.Proxy{IsProxy: false}, nil
}

func (h *testHybridDb) IsEmpty() bool {
	return false
}

// Mock readers for different databases
type maxmindDb struct{}

func (m *maxmindDb) Country(net.IP) (geo.Country, error) {
	return geo.Country{Name: "United States", ISO: "US"}, nil
}

func (m *maxmindDb) City(net.IP) (geo.City, error) {
	return geo.City{Name: "New York"}, nil
}

func (m *maxmindDb) ASN(net.IP) (geo.ASN, error) {
	return geo.ASN{AutonomousSystemNumber: 12345}, nil
}

func (m *maxmindDb) ISP(net.IP) (geo.ISP, error) {
	return geo.ISP{ISP: "MaxMind ISP"}, nil
}

func (m *maxmindDb) ConnectionType(net.IP) (geo.ConnectionType, error) {
	return geo.ConnectionType{ConnectionType: "Cable/DSL"}, nil
}

func (m *maxmindDb) Proxy(net.IP) (geo.Proxy, error) {
	return geo.Proxy{IsProxy: false}, nil
}

func (m *maxmindDb) IsEmpty() bool {
	return false
}

type czdbDb struct{}

func (c *czdbDb) Country(net.IP) (geo.Country, error) {
	return geo.Country{Name: "中国", ISO: "CN"}, nil
}

func (c *czdbDb) City(net.IP) (geo.City, error) {
	return geo.City{Name: "北京"}, nil
}

func (c *czdbDb) ASN(net.IP) (geo.ASN, error) {
	return geo.ASN{AutonomousSystemNumber: 54321}, nil
}

func (c *czdbDb) ISP(net.IP) (geo.ISP, error) {
	return geo.ISP{ISP: "纯真 ISP"}, nil
}

func (c *czdbDb) ConnectionType(net.IP) (geo.ConnectionType, error) {
	return geo.ConnectionType{ConnectionType: "Broadband"}, nil
}

func (c *czdbDb) Proxy(net.IP) (geo.Proxy, error) {
	return geo.Proxy{IsProxy: false}, nil
}

func (c *czdbDb) IsEmpty() bool {
	return false
}

func testServer() *Server {
	return &Server{cache: NewCache(100), gr: &testDb{}, LookupAddr: lookupAddr, LookupPort: lookupPort}
}

func testHybridServer() *Server {
	hybrid := &testHybridDb{
		maxmind: &maxmindDb{},
		czdbV4:  &czdbDb{},
		czdbV6:  nil,
		qqwry:   nil,
	}
	return &Server{cache: NewCache(100), gr: hybrid, LookupAddr: lookupAddr, LookupPort: lookupPort}
}

func httpGet(url string, acceptMediaType string, userAgent string) (string, int, error) {
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", 0, err
	}
	if acceptMediaType != "" {
		r.Header.Set("Accept", acceptMediaType)
	}
	r.Header.Set("User-Agent", userAgent)
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", 0, err
	}
	return string(data), res.StatusCode, nil
}

func httpPost(url, body string) (*http.Response, string, error) {
	r, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	return res, string(data), nil
}

func TestCLIHandlers(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	s := httptest.NewServer(testServer().Handler())

	var tests = []struct {
		url             string
		out             string
		status          int
		userAgent       string
		acceptMediaType string
	}{
		{s.URL, "127.0.0.1\n", 200, "curl/7.43.0", ""},
		{s.URL, "127.0.0.1\n", 200, "foo/bar", textMediaType},
		{s.URL + "/ip", "127.0.0.1\n", 200, "", ""},
		{s.URL + "/country", "Elbonia\n", 200, "", ""},
		{s.URL + "/country_code", "EB\n", 200, "", ""},
		{s.URL + "/coordinates", "63.416667,10.416667\n", 200, "", ""},
		{s.URL + "/city", "Bornyasherk\n", 200, "", ""},
		{s.URL + "/foo", "404 page not found", 404, "", ""},
		{s.URL + "/asn", "AS59795\n", 200, "", ""},
	}

	for _, tt := range tests {
		out, status, err := httpGet(tt.url, tt.acceptMediaType, tt.userAgent)
		if err != nil {
			t.Fatal(err)
		}
		if status != tt.status {
			t.Errorf("Expected %d, got %d", tt.status, status)
		}
		if out != tt.out {
			t.Errorf("Expected %q, got %q", tt.out, out)
		}
	}
}

func TestDisabledHandlers(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	server := testServer()
	server.LookupPort = nil
	server.LookupAddr = nil
	server.gr, _ = geo.Open("", "", "", "", "")
	s := httptest.NewServer(server.Handler())

	var tests = []struct {
		url    string
		out    string
		status int
	}{
		{s.URL + "/port/1337", "404 page not found", 404},
		{s.URL + "/country", "404 page not found", 404},
		{s.URL + "/country-code", "404 page not found", 404},
		{s.URL + "/city", "404 page not found", 404},
		{s.URL + "/json", "{\n  \"ip\": \"127.0.0.1\",\n  \"ip_decimal\": 2130706433,\n  \"is_proxy\": false\n}", 200},
	}

	for _, tt := range tests {
		out, status, err := httpGet(tt.url, "", "")
		if err != nil {
			t.Fatal(err)
		}
		if status != tt.status {
			t.Errorf("Expected %d, got %d", tt.status, status)
		}
		if out != tt.out {
			t.Errorf("Expected %q, got %q", tt.out, out)
		}
	}
}

func TestJSONHandlers(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	s := httptest.NewServer(testServer().Handler())

	var tests = []struct {
		url    string
		out    string
		status int
	}{
		{s.URL, "{\n  \"ip\": \"127.0.0.1\",\n  \"ip_decimal\": 2130706433,\n  \"country\": \"Elbonia\",\n  \"country_code\": \"EB\",\n  \"country_eu\": false,\n  \"region\": \"North Elbonia\",\n  \"region_code\": \"1234\",\n  \"metro_code\": 1234,\n  \"zip_code\": \"1234\",\n  \"city\": \"Bornyasherk\",\n  \"latitude\": 63.416667,\n  \"longitude\": 10.416667,\n  \"time_zone\": \"Europe/Bornyasherk\",\n  \"asn\": \"AS59795\",\n  \"isp\": \"Hosting4Real\",\n  \"org\": \"Hosting4Real\",\n  \"connection_type\": \"Corporate\",\n  \"is_proxy\": false,\n  \"hostname\": \"localhost\",\n  \"user_agent\": \"curl/7.2.6.0\"\n}", 200},
		{s.URL + "/port/foo", "{\n  \"status\": 400,\n  \"error\": \"invalid port: foo\"\n}", 400},
		{s.URL + "/port/0", "{\n  \"status\": 400,\n  \"error\": \"invalid port: 0\"\n}", 400},
		{s.URL + "/port/65537", "{\n  \"status\": 400,\n  \"error\": \"invalid port: 65537\"\n}", 400},
		{s.URL + "/port/31337", "{\n  \"ip\": \"127.0.0.1\",\n  \"port\": 31337,\n  \"reachable\": true\n}", 200},
		{s.URL + "/port/80", "{\n  \"ip\": \"127.0.0.1\",\n  \"port\": 80,\n  \"reachable\": true\n}", 200},            // checking that our test server is reachable on port 80
		{s.URL + "/port/80?ip=1.3.3.7", "{\n  \"ip\": \"127.0.0.1\",\n  \"port\": 80,\n  \"reachable\": true\n}", 200}, // ensuring that the "ip" parameter is not usable to check remote host ports
		{s.URL + "/foo", "{\n  \"status\": 404,\n  \"error\": \"404 page not found\"\n}", 404},
		{s.URL + "/health", `{"status":"OK"}`, 200},
	}

	for _, tt := range tests {
		out, status, err := httpGet(tt.url, jsonMediaType, "curl/7.2.6.0")
		if err != nil {
			t.Fatal(err)
		}
		if status != tt.status {
			t.Errorf("Expected %d for %s, got %d", tt.status, tt.url, status)
		}
		if out != tt.out {
			t.Errorf("Expected %q for %s, got %q", tt.out, tt.url, out)
		}
	}
}

func TestCacheHandler(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	srv := testServer()
	srv.profile = true
	s := httptest.NewServer(srv.Handler())
	got, _, err := httpGet(s.URL+"/debug/cache/", jsonMediaType, "")
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"size\": 0,\n  \"capacity\": 100,\n  \"evictions\": 0\n}"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCacheResizeHandler(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	srv := testServer()
	srv.profile = true
	s := httptest.NewServer(srv.Handler())
	_, got, err := httpPost(s.URL+"/debug/cache/resize", "10")
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"message\": \"Changed cache capacity to 10.\"\n}"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestIPFromRequest(t *testing.T) {
	var tests = []struct {
		remoteAddr     string
		headerKey      string
		headerValue    string
		trustedHeaders []string
		out            string
	}{
		{"127.0.0.1:9999", "", "", nil, "127.0.0.1"},                                                                // No header given
		{"127.0.0.1:9999", "X-Real-IP", "1.3.3.7", nil, "127.0.0.1"},                                                // Trusted header is empty
		{"127.0.0.1:9999", "X-Real-IP", "1.3.3.7", []string{"X-Foo-Bar"}, "127.0.0.1"},                              // Trusted header does not match
		{"127.0.0.1:9999", "X-Real-IP", "1.3.3.7", []string{"X-Real-IP", "X-Forwarded-For"}, "1.3.3.7"},             // Trusted header matches
		{"127.0.0.1:9999", "X-Forwarded-For", "1.3.3.7", []string{"X-Real-IP", "X-Forwarded-For"}, "1.3.3.7"},       // Second trusted header matches
		{"127.0.0.1:9999", "X-Forwarded-For", "1.3.3.7,4.2.4.2", []string{"X-Forwarded-For"}, "1.3.3.7"},            // X-Forwarded-For with multiple entries (commas separator)
		{"127.0.0.1:9999", "X-Forwarded-For", "1.3.3.7, 4.2.4.2", []string{"X-Forwarded-For"}, "1.3.3.7"},           // X-Forwarded-For with multiple entries (space+comma separator)
		{"127.0.0.1:9999", "X-Forwarded-For", "", []string{"X-Forwarded-For"}, "127.0.0.1"},                         // Empty header
		{"127.0.0.1:9999?ip=1.2.3.4", "", "", nil, "1.2.3.4"},                                                       // passed in "ip" parameter
		{"127.0.0.1:9999?ip=1.2.3.4", "X-Forwarded-For", "1.3.3.7,4.2.4.2", []string{"X-Forwarded-For"}, "1.2.3.4"}, // ip parameter wins over X-Forwarded-For with multiple entries
	}
	for _, tt := range tests {
		u, err := url.Parse("http://" + tt.remoteAddr)
		if err != nil {
			t.Fatal(err)
		}
		r := &http.Request{
			RemoteAddr: u.Host,
			Header:     http.Header{},
			URL:        u,
		}
		r.Header.Add(tt.headerKey, tt.headerValue)
		ip, err := ipFromRequest(tt.trustedHeaders, r, true)
		if err != nil {
			t.Fatal(err)
		}
		out := net.ParseIP(tt.out)
		if !ip.Equal(out) {
			t.Errorf("Expected %s, got %s", out, ip)
		}
	}
}

func TestLangParameter(t *testing.T) {
	s := testHybridServer()

	tests := []struct {
		name            string
		lang            string
		ip              string
		expectedCountry string
		expectedISP     string
	}{
		{
			name:            "lang=zh should use CZDB",
			lang:            "zh",
			ip:              "114.114.114.114",
			expectedCountry: "美国", // 测试中纯真库和MaxMind冲突，以MaxMind为准并翻译为中文
			expectedISP:     "纯真 ISP",
		},
		{
			name:            "lang=en should use MaxMind",
			lang:            "en",
			ip:              "1.1.1.1",
			expectedCountry: "United States",
			expectedISP:     "MaxMind ISP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://example.com/json?ip=" + tt.ip
			if tt.lang != "" {
				url += "&lang=" + tt.lang
			}

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Accept", "application/json")

			w := httptest.NewRecorder()
			appErr := s.JSONHandler(w, req)
			if appErr != nil {
				t.Fatal(appErr.Error)
			}

			body := w.Body.String()
			if !strings.Contains(body, tt.expectedCountry) {
				t.Errorf("Expected country %s not found in response: %s", tt.expectedCountry, body)
			}
			if !strings.Contains(body, tt.expectedISP) {
				t.Errorf("Expected ISP %s not found in response: %s", tt.expectedISP, body)
			}
		})
	}
}
