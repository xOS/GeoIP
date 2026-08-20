package geo

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/oschwald/maxminddb-golang"
)

// GeoCNDivisionCodesFile is the path to the gb2260 division codes mapping file
var GeoCNDivisionCodesFile string = "division_codes.json"

// GeoCNMMDBReader implements the Reader interface for GeoCN.mmdb database
type GeoCNMMDBReader struct {
	reader      *maxminddb.Reader
	divisionMap map[string][]string
}

// GeoCNRecord represents the structure of records in GeoCN.mmdb
type GeoCNRecord struct {
	// Standard MaxMind fields (kept for backward compatibility with standard GeoCN DBs)
	Province  interface{} `maxminddb:"province"`
	City      interface{} `maxminddb:"city"`
	Districts interface{} `maxminddb:"districts"`
	District  interface{} `maxminddb:"district"`
	County    interface{} `maxminddb:"county"`
	Network   string      `maxminddb:"network"`

	// ljxi/GeoCN specific fields
	DivisionCode interface{} `maxminddb:"division_code"`
	ISP          interface{} `maxminddb:"isp"`
	Type         interface{} `maxminddb:"type"`
}

// extractGeoString robustly extracts a string from an interface{} which might be a string or a slice
func extractGeoString(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case []interface{}:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	case []string:
		if len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

// extractGeoCode extracts an integer division code as string from interface{}
func extractGeoCode(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return ""
}

// NewGeoCNMMDBReader creates a new GeoCN MMDB reader
func NewGeoCNMMDBReader(path string) (*GeoCNMMDBReader, error) {
	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}

	divMap := make(map[string][]string)
	if GeoCNDivisionCodesFile != "" {
		if b, err := os.ReadFile(GeoCNDivisionCodesFile); err == nil {
			_ = json.Unmarshal(b, &divMap)
		}
	}

	return &GeoCNMMDBReader{
		reader:      reader,
		divisionMap: divMap,
	}, nil
}

// Country returns country information for the given IP
func (g *GeoCNMMDBReader) Country(ip net.IP) (Country, error) {
	// GeoCN is a China-specific database, typically doesn't contain country ISO codes
	return Country{}, nil
}

// City returns city information for the given IP
func (g *GeoCNMMDBReader) City(ip net.IP) (City, error) {
	// Check if reader is nil
	if g.reader == nil {
		return City{}, fmt.Errorf("reader is nil")
	}

	var record GeoCNRecord
	err := g.reader.Lookup(ip, &record)
	if err != nil {
		return City{}, err
	}

	city := City{}
	city.Name = extractGeoString(record.City)
	
	prov := extractGeoString(record.Province)
	city.RegionName = prov

	dist := extractGeoString(record.Districts)
	if dist == "" {
		dist = extractGeoString(record.District)
	}
	if dist == "" {
		dist = extractGeoString(record.County)
	}
	city.District = dist

	// GB2260 division code translation fallback (primary for modern GeoCN)
	if city.RegionName == "" && city.Name == "" && city.District == "" && g.divisionMap != nil {
		divCode := extractGeoCode(record.DivisionCode)
		if divCode != "" {
			if names, ok := g.divisionMap[divCode]; ok && len(names) >= 3 {
				city.RegionName = names[0]
				city.Name = names[1]
				city.District = names[2]
			}
		}
	}

	return city, nil
}

// ASN returns ASN information for the given IP
func (g *GeoCNMMDBReader) ASN(ip net.IP) (ASN, error) {
	// GeoCN.mmdb doesn't typically contain ASN information
	// This would need to be obtained from a separate database
	return ASN{}, nil
}

// ISP returns ISP information for the given IP
func (g *GeoCNMMDBReader) ISP(ip net.IP) (ISP, error) {
	// Check if reader is nil
	if g.reader == nil {
		return ISP{}, fmt.Errorf("reader is nil")
	}

	var record GeoCNRecord
	err := g.reader.Lookup(ip, &record)
	if err != nil {
		return ISP{}, err
	}

	isp := ISP{}
	ispStr := extractGeoString(record.ISP)
	isp.ISP = ispStr

	// If we have ISP info, also set the organization
	if ispStr != "" {
		isp.Organization = ispStr
	}

	return isp, nil
}

// ConnectionType returns connection type information for the given IP
func (g *GeoCNMMDBReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	// Check if reader is nil
	if g.reader == nil {
		return ConnectionType{}, fmt.Errorf("reader is nil")
	}

	var record GeoCNRecord
	err := g.reader.Lookup(ip, &record)
	if err != nil {
		return ConnectionType{}, err
	}

	connType := ConnectionType{}
	connType.ConnectionType = extractGeoString(record.Type)

	return connType, nil
}

// Proxy returns proxy information for the given IP
func (g *GeoCNMMDBReader) Proxy(ip net.IP) (Proxy, error) {
	return Proxy{IsProxy: false}, nil
}

// Network returns the network (CIDR) containing the IP if available.
// This is a best-effort helper for tests; it returns nil when not available.
func (g *GeoCNMMDBReader) Network(ip net.IP) (*net.IPNet, error) {
	if g.reader == nil {
		return nil, nil
	}
	var record GeoCNRecord
	network, _, err := g.reader.LookupNetwork(ip, &record)
	if err != nil {
		return nil, err
	}
	if network != nil {
		return network, nil
	}
	// Fallback to explicit Network string if defined in standard DBs
	if record.Network != "" {
		_, n, err := net.ParseCIDR(record.Network)
		if err == nil {
			return n, nil
		}
	}
	return nil, nil
}

// IsEmpty returns true if the reader is empty
func (g *GeoCNMMDBReader) IsEmpty() bool {
	return g.reader == nil
}

// Close closes the underlying database reader
func (g *GeoCNMMDBReader) Close() error {
	if g.reader != nil {
		return g.reader.Close()
	}
	return nil
}
