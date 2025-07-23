package geo

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/tagphi/czdb-search-golang/pkg/db"
)

// DatabaseType represents the type of database
type DatabaseType int

const (
	DatabaseTypeUnknown DatabaseType = iota
	DatabaseTypeMaxMindMMDB
	DatabaseTypeIP2LocationBIN
	DatabaseTypeIP2ProxyBIN
	DatabaseTypeQQWryIPDB
	DatabaseTypeQQWryDAT
	DatabaseTypeCZDB // 新增 czdb 类型
)

// DatabaseInfo contains information about a database file
type DatabaseInfo struct {
	Type     DatabaseType
	Path     string
	IsProxy  bool
	Provider string
}

// DetectDatabaseType detects the type of database based on file extension and content
func DetectDatabaseType(path string) (DatabaseType, error) {
	if path == "" {
		return DatabaseTypeUnknown, nil
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DatabaseTypeUnknown, fmt.Errorf("database file does not exist: %s", path)
	}

	ext := strings.ToLower(filepath.Ext(path))
	filename := strings.ToLower(filepath.Base(path))

	// Detect by file extension
		switch ext {
		case ".mmdb":
			// GeoCN.mmdb 是 MaxMind MMDB 格式
			return DatabaseTypeMaxMindMMDB, nil
		case ".czdb":
			return DatabaseTypeCZDB, nil // 新增 czdb 检测
	case ".ipdb":
		// Check if it's qqwry.ipdb format
		if strings.Contains(filename, "qqwry") || strings.Contains(filename, "ipdb") {
			return DatabaseTypeQQWryIPDB, nil
		}
		return DatabaseTypeQQWryIPDB, nil // Default to qqwry for .ipdb files
	case ".dat":
		// Check if it's qqwry.dat format
		if strings.Contains(filename, "qqwry") {
			return DatabaseTypeQQWryDAT, nil
		}
		return DatabaseTypeQQWryDAT, nil // Default to qqwry for .dat files
	case ".bin":
		// Check if it's IP2Proxy or IP2Location BIN
		if strings.Contains(filename, "ip2proxy") || strings.Contains(filename, "proxy") {
			return DatabaseTypeIP2ProxyBIN, nil
		}
		// Check for IP2Location patterns
		if strings.Contains(filename, "ip2location") ||
			strings.Contains(filename, "db1") || strings.Contains(filename, "db2") ||
			strings.Contains(filename, "db3") || strings.Contains(filename, "db4") ||
			strings.Contains(filename, "db5") || strings.Contains(filename, "db6") ||
			strings.Contains(filename, "db7") || strings.Contains(filename, "db8") ||
			strings.Contains(filename, "db9") || strings.Contains(filename, "db10") ||
			strings.Contains(filename, "db11") || strings.Contains(filename, "db12") ||
			strings.Contains(filename, "db13") || strings.Contains(filename, "db14") ||
			strings.Contains(filename, "db15") || strings.Contains(filename, "db16") ||
			strings.Contains(filename, "db17") || strings.Contains(filename, "db18") ||
			strings.Contains(filename, "db19") || strings.Contains(filename, "db20") ||
			strings.Contains(filename, "db21") || strings.Contains(filename, "db22") ||
			strings.Contains(filename, "db23") || strings.Contains(filename, "db24") ||
			strings.Contains(filename, "db25") || strings.Contains(filename, "db26") {
			return DatabaseTypeIP2LocationBIN, nil
		}
		// Default to IP2Location for other BIN files
		return DatabaseTypeIP2LocationBIN, nil
	default:
		// Try to detect by filename patterns
		if strings.Contains(filename, "ip2proxy") || strings.Contains(filename, "proxy") {
			return DatabaseTypeIP2ProxyBIN, nil
		}
		if strings.Contains(filename, "ip2location") || strings.Contains(filename, "geolite") || strings.Contains(filename, "geoip") {
			if strings.Contains(filename, ".mmdb") {
				return DatabaseTypeMaxMindMMDB, nil
			}
			return DatabaseTypeIP2LocationBIN, nil
		}
		return DatabaseTypeUnknown, fmt.Errorf("unknown database type: %s", path)
	}
}

// CreateReader creates a Reader based on the database type
func CreateReader(path string, czdbKey ...string) (Reader, error) {
	if path == "" {
		return &EmptyReader{}, nil
	}

	dbType, err := DetectDatabaseType(path)
	if err != nil {
		return nil, err
	}

	switch dbType {
	case DatabaseTypeMaxMindMMDB:
		// Check if this is a GeoCN.mmdb file
		if strings.Contains(strings.ToLower(filepath.Base(path)), "geocn") {
			return NewGeoCNMMDBReader(path)
		}
		// For other MaxMind MMDB, we need to determine what type it is
		// This is a simplified approach - in practice you might want to check the database metadata
		return createMaxMindReader(path)
	case DatabaseTypeIP2LocationBIN:
		return NewIP2LocationBinReader(path)
	case DatabaseTypeIP2ProxyBIN:
		return NewIP2ProxyBinReader(path)
	case DatabaseTypeQQWryIPDB:
		return NewQQWryIPDBReader(path)
	case DatabaseTypeQQWryDAT:
		return NewQQWryDatReader(path)
	case DatabaseTypeCZDB:
		key := ""
		if len(czdbKey) > 0 {
			key = czdbKey[0]
		}
		if key == "" {
			return nil, fmt.Errorf("czdb key required for file: %s (key is empty)", path)
		}
		reader, err := NewCZDBReader(path, key, db.MEMORY)
		if err != nil {
			return nil, fmt.Errorf("failed to open czdb: %s, key: %q, error: %v", path, key, err)
		}
		return reader, nil
	default:
		return nil, fmt.Errorf("unsupported database type for file: %s", path)
	}
}

// createMaxMindReader creates a MaxMind reader (simplified version)
func createMaxMindReader(path string) (Reader, error) {
	// This is a simplified approach - we create a basic geoip reader
	// In practice, you might want to detect the specific MaxMind database type
	return OpenWithProxy(path, "", "", "", "", "")
}

// EmptyReader is a no-op reader for empty database paths
type EmptyReader struct{}

func (e *EmptyReader) Country(net.IP) (Country, error)               { return Country{}, nil }
func (e *EmptyReader) City(net.IP) (City, error)                     { return City{}, nil }
func (e *EmptyReader) ASN(net.IP) (ASN, error)                       { return ASN{}, nil }
func (e *EmptyReader) ISP(net.IP) (ISP, error)                       { return ISP{}, nil }
func (e *EmptyReader) ConnectionType(net.IP) (ConnectionType, error) { return ConnectionType{}, nil }
func (e *EmptyReader) Proxy(net.IP) (Proxy, error)                   { return Proxy{}, nil }
func (e *EmptyReader) Network(net.IP) (*net.IPNet, error)            { return nil, fmt.Errorf("empty reader") }
func (e *EmptyReader) IsEmpty() bool                                 { return true }

// OpenAuto automatically detects database types and creates appropriate readers
func OpenAuto(databases ...string) (Reader, error) {
	var readers []Reader
	var hasData bool

	for _, dbPath := range databases {
		if dbPath == "" {
			continue
		}

		// 兼容 czdb 密钥参数（此处可后续扩展为结构体/配置）
		reader, err := CreateReader(dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create reader for %s: %v", dbPath, err)
		}

		if !reader.IsEmpty() {
			readers = append(readers, reader)
			hasData = true
		}
	}

	if !hasData {
		return &EmptyReader{}, nil
	}

	if len(readers) == 1 {
		return readers[0], nil
	}

	// Combine multiple readers
	return createCombinedReader(readers), nil
}

// createCombinedReader creates a combined reader from multiple readers
func createCombinedReader(readers []Reader) Reader {
	if len(readers) == 0 {
		return &EmptyReader{}
	}
	if len(readers) == 1 {
		return readers[0]
	}

	// For now, use the first two readers in a combined reader
	// In a more sophisticated implementation, you might want to prioritize by type
	return NewCombinedReader(readers[0], readers[1])
}

// GetDatabaseInfo returns information about a database file
func GetDatabaseInfo(path string) (*DatabaseInfo, error) {
	dbType, err := DetectDatabaseType(path)
	if err != nil {
		return nil, err
	}

	info := &DatabaseInfo{
		Type: dbType,
		Path: path,
	}

	switch dbType {
			case DatabaseTypeIP2ProxyBIN:
				info.IsProxy = true
				info.Provider = "IP2Location"
			case DatabaseTypeIP2LocationBIN:
				info.IsProxy = false
				info.Provider = "IP2Location"
			case DatabaseTypeMaxMindMMDB:
				info.IsProxy = false
				info.Provider = "MaxMind"
			case DatabaseTypeQQWryIPDB:
				info.IsProxy = false
				info.Provider = "IPIP.net"
			}

	return info, nil
}

func openDatabaseWithDebug(path string, openFunc func(string) (interface{}, error), label string) (interface{}, error) {
	_, _ = filepath.Abs(path)
	_, err := os.Stat(path)
	if err != nil {
	} else {
	}
	db, err := openFunc(path)
	if err != nil {
	} else {
	}
	return db, err
}
