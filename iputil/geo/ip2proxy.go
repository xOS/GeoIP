package geo

import (
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
)

// IP2ProxyRecord represents a single record from IP2Proxy CSV database
type IP2ProxyRecord struct {
	IPFrom      uint32
	IPTo        uint32
	ProxyType   string
	CountryCode string
	CountryName string
	RegionName  string
	CityName    string
	ISP         string
	Domain      string
	UsageType   string
	ASN         string
	AS          string
	LastSeen    string
	Threat      string
	Provider    string
	FraudScore  string
}

// IP2ProxyReader implements the Reader interface for IP2Proxy CSV databases
type IP2ProxyReader struct {
	records []IP2ProxyRecord
}

// NewIP2ProxyReader creates a new IP2Proxy reader from CSV file
func NewIP2ProxyReader(csvPath string) (*IP2ProxyReader, error) {
	if csvPath == "" {
		return &IP2ProxyReader{}, nil
	}

	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open IP2Proxy CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comment = '#'
	reader.TrimLeadingSpace = true

	var records []IP2ProxyRecord

	// Skip header if present
	firstRecord, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %v", err)
	}

	// Check if first record is header
	if strings.ToLower(firstRecord[0]) != "ip_from" {
		// First record is data, not header, so process it
		record, err := parseIP2ProxyRecord(firstRecord)
		if err != nil {
			return nil, fmt.Errorf("failed to parse first record: %v", err)
		}
		records = append(records, record)
	}

	// Read remaining records
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record: %v", err)
		}

		parsedRecord, err := parseIP2ProxyRecord(record)
		if err != nil {
			return nil, fmt.Errorf("failed to parse record: %v", err)
		}
		records = append(records, parsedRecord)
	}

	// Sort records by IPTo for binary search
	sort.Slice(records, func(i, j int) bool {
		return records[i].IPTo < records[j].IPTo
	})

	return &IP2ProxyReader{records: records}, nil
}

// parseIP2ProxyRecord parses a CSV record into IP2ProxyRecord
func parseIP2ProxyRecord(record []string) (IP2ProxyRecord, error) {
	if len(record) < 4 {
		return IP2ProxyRecord{}, fmt.Errorf("invalid record format, expected at least 4 fields, got %d", len(record))
	}

	ipFrom, err := strconv.ParseUint(strings.Trim(record[0], `"`), 10, 32)
	if err != nil {
		return IP2ProxyRecord{}, fmt.Errorf("invalid ip_from: %v", err)
	}

	ipTo, err := strconv.ParseUint(strings.Trim(record[1], `"`), 10, 32)
	if err != nil {
		return IP2ProxyRecord{}, fmt.Errorf("invalid ip_to: %v", err)
	}

	result := IP2ProxyRecord{
		IPFrom:      uint32(ipFrom),
		IPTo:        uint32(ipTo),
		ProxyType:   getField(record, 2),
		CountryCode: getField(record, 3),
		CountryName: getField(record, 4),
		RegionName:  getField(record, 5),
		CityName:    getField(record, 6),
		ISP:         getField(record, 7),
		Domain:      getField(record, 8),
		UsageType:   getField(record, 9),
		ASN:         getField(record, 10),
		AS:          getField(record, 11),
		LastSeen:    getField(record, 12),
		Threat:      getField(record, 13),
		Provider:    getField(record, 14),
		FraudScore:  getField(record, 15),
	}

	return result, nil
}

// getField safely extracts a field from CSV record, handling missing fields
func getField(record []string, index int) string {
	if index >= len(record) {
		return ""
	}
	field := strings.Trim(record[index], `"`)
	if field == "-" {
		return ""
	}
	return field
}

// ipToUint32 converts net.IP to uint32 for IPv4
func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 + uint32(ip[1])<<16 + uint32(ip[2])<<8 + uint32(ip[3])
}

// lookupIP searches for IP in the sorted records using binary search
func (r *IP2ProxyReader) lookupIP(ip net.IP) *IP2ProxyRecord {
	if len(r.records) == 0 {
		return nil
	}

	ipNum := ipToUint32(ip)
	if ipNum == 0 {
		return nil // Invalid IPv4 or IPv6 not supported yet
	}

	// Binary search for the record
	left, right := 0, len(r.records)-1

	for left <= right {
		mid := (left + right) / 2
		record := &r.records[mid]

		if ipNum >= record.IPFrom && ipNum <= record.IPTo {
			return record
		} else if ipNum < record.IPFrom {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	return nil
}

// Country implements the Reader interface
func (r *IP2ProxyReader) Country(ip net.IP) (Country, error) {
	record := r.lookupIP(ip)
	if record == nil {
		return Country{}, nil
	}

	return Country{
		Name: record.CountryName,
		ISO:  record.CountryCode,
		IsEU: nil, // IP2Proxy doesn't provide EU information
	}, nil
}

// City implements the Reader interface
func (r *IP2ProxyReader) City(ip net.IP) (City, error) {
	record := r.lookupIP(ip)
	if record == nil {
		return City{}, nil
	}

	return City{
		Name:       record.CityName,
		RegionName: record.RegionName,
		RegionCode: "", // IP2Proxy doesn't provide region codes
		Latitude:   0,  // IP2Proxy doesn't provide coordinates in basic format
		Longitude:  0,  // IP2Proxy doesn't provide coordinates in basic format
		PostalCode: "", // IP2Proxy doesn't provide postal codes
		Timezone:   "", // IP2Proxy doesn't provide timezone
		MetroCode:  0,  // IP2Proxy doesn't provide metro codes
	}, nil
}

// ASN implements the Reader interface
func (r *IP2ProxyReader) ASN(ip net.IP) (ASN, error) {
	record := r.lookupIP(ip)
	if record == nil {
		return ASN{}, nil
	}

	var asnNumber uint
	if record.ASN != "" {
		if parsed, err := strconv.ParseUint(record.ASN, 10, 32); err == nil {
			asnNumber = uint(parsed)
		}
	}

	return ASN{
		AutonomousSystemNumber:       asnNumber,
		AutonomousSystemOrganization: record.AS,
	}, nil
}

// ISP implements the Reader interface
func (r *IP2ProxyReader) ISP(ip net.IP) (ISP, error) {
	record := r.lookupIP(ip)
	if record == nil {
		return ISP{}, nil
	}

	var asnNumber uint
	if record.ASN != "" {
		if parsed, err := strconv.ParseUint(record.ASN, 10, 32); err == nil {
			asnNumber = uint(parsed)
		}
	}

	return ISP{
		ASN:          asnNumber,
		ORG:          record.AS,
		ISP:          record.ISP,
		Organization: record.AS,
	}, nil
}

// ConnectionType implements the Reader interface
func (r *IP2ProxyReader) ConnectionType(ip net.IP) (ConnectionType, error) {
	record := r.lookupIP(ip)
	if record == nil {
		return ConnectionType{}, nil
	}

	return ConnectionType{
		ConnectionType: record.UsageType,
	}, nil
}

// Proxy implements the Reader interface
func (r *IP2ProxyReader) Proxy(ip net.IP) (Proxy, error) {
	record := r.lookupIP(ip)
	if record == nil {
		return Proxy{IsProxy: false}, nil
	}

	return Proxy{
		IsProxy:     true,
		ProxyType:   record.ProxyType,
		Country:     record.CountryName,
		CountryCode: record.CountryCode,
		Domain:      record.Domain,
		UsageType:   record.UsageType,
		LastSeen:    record.LastSeen,
		Threat:      record.Threat,
		Provider:    record.Provider,
		FraudScore:  record.FraudScore,
	}, nil
}

// IsEmpty implements the Reader interface
func (r *IP2ProxyReader) IsEmpty() bool {
	return len(r.records) == 0
}
