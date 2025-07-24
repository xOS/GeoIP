package geo

import (
	"net"

	"github.com/tagphi/czdb-search-golang/pkg/db"
)

// CZDBReader implements Reader interface for cz88 .czdb format (v4/v6)
type CZDBReader struct {
	searcher *db.DBSearcher
}

// NewCZDBReader creates a new czdb reader (v4/v6)
func NewCZDBReader(path, key string, mode ...db.SearchType) (*CZDBReader, error) {
	searchMode := db.MEMORY
	if len(mode) > 0 {
		searchMode = mode[0]
	}
	searcher, err := db.InitDBSearcher(path, key, searchMode)
	if err != nil {
		return nil, err
	}
	return &CZDBReader{searcher: searcher}, nil
}

func (c *CZDBReader) Country(ip net.IP) (Country, error) {
	raw, err := db.Search(ip.String(), c.searcher)
	if err != nil {
		return Country{}, err
	}
	country, _, _, _, _ := splitCZDBFields(raw)
	return Country{Name: country, ISO: "CN"}, nil
}

func (c *CZDBReader) City(ip net.IP) (City, error) {
	raw, err := db.Search(ip.String(), c.searcher)
	if err != nil {
		return City{}, err
	}
	_, region, city, district, _ := splitCZDBFields(raw)
	return City{RegionName: region, Name: city, District: district}, nil
}

func (c *CZDBReader) ASN(ip net.IP) (ASN, error) {
	return ASN{}, nil
}

func (c *CZDBReader) ISP(ip net.IP) (ISP, error) {
	raw, err := db.Search(ip.String(), c.searcher)
	if err != nil {
		return ISP{}, err
	}
	_, _, _, _, isp := splitCZDBFields(raw)
	return ISP{ISP: isp, Organization: isp}, nil
}

func (c *CZDBReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	return ConnectionType{}, nil
}

func (c *CZDBReader) Proxy(ip net.IP) (Proxy, error) {
	return Proxy{}, nil
}



func (c *CZDBReader) IsEmpty() bool {
	return c.searcher == nil
}

func (c *CZDBReader) Close() {
	if c.searcher != nil {
		db.CloseDBSearcher(c.searcher)
	}
}
