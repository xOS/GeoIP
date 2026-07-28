package geo

import (
	"fmt"
	"log"
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
		log.Printf("[ERROR] [GeoIP] 数据库文件不存在 / Database file does not exist: path=%q", path)
		return DatabaseTypeUnknown, fmt.Errorf("database file does not exist: %s", path)
	} else if err != nil {
		log.Printf("[ERROR] [GeoIP] 无法访问数据库文件 / Cannot access database file: path=%q err=%v", path, err)
		return DatabaseTypeUnknown, fmt.Errorf("cannot access database file %s: %v", path, err)
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
		log.Printf("[ERROR] [GeoIP] 不支持的数据库格式 / Unsupported database format: path=%q", path)
		return DatabaseTypeUnknown, fmt.Errorf("unknown database type: %s", path)
	}
}

// CreateReader creates a Reader based on the database type
func CreateReader(path string, czdbKey ...string) (Reader, error) {
	if path == "" {
		return &EmptyReader{}, nil
	}

	log.Printf("[INFO] [GeoIP] CreateReader: 正在识别并创建数据库读取器 / Attempting to detect and load database: path=%q", path)
	dbType, err := DetectDatabaseType(path)
	if err != nil {
		log.Printf("[ERROR] [GeoIP] CreateReader: 识别数据库格式失败 / Failed to detect database type: path=%q err=%v", path, err)
		return nil, err
	}

	switch dbType {
	case DatabaseTypeMaxMindMMDB:
		// Check if this is a GeoCN.mmdb file
		if strings.Contains(strings.ToLower(filepath.Base(path)), "geocn") {
			reader, err := NewGeoCNMMDBReader(path)
			if err != nil {
				log.Printf("[ERROR] [GeoIP] GeoCN MMDB 读取失败 / Failed to load GeoCN MMDB: path=%q err=%v", path, err)
			} else {
				log.Printf("[INFO] [GeoIP] 成功加载 GeoCN MMDB / Successfully loaded GeoCN MMDB: path=%q", path)
			}
			return reader, err
		}
		// For other MaxMind MMDB, we need to determine what type it is
		// This is a simplified approach - in practice you might want to check the database metadata
		reader, err := createMaxMindReader(path)
		if err != nil {
			log.Printf("[ERROR] [GeoIP] MaxMind MMDB 读取失败 / Failed to load MaxMind MMDB: path=%q err=%v", path, err)
		} else {
			log.Printf("[INFO] [GeoIP] 成功加载 MaxMind MMDB / Successfully loaded MaxMind MMDB: path=%q", path)
		}
		return reader, err
	case DatabaseTypeIP2LocationBIN:
		reader, err := NewIP2LocationBinReader(path)
		if err != nil {
			log.Printf("[ERROR] [GeoIP] IP2Location BIN 读取失败 / Failed to load IP2Location BIN: path=%q err=%v", path, err)
		} else {
			log.Printf("[INFO] [GeoIP] 成功加载 IP2Location BIN / Successfully loaded IP2Location BIN: path=%q", path)
		}
		return reader, err
	case DatabaseTypeIP2ProxyBIN:
		reader, err := NewIP2ProxyBinReader(path)
		if err != nil {
			log.Printf("[ERROR] [GeoIP] IP2Proxy BIN 读取失败 / Failed to load IP2Proxy BIN: path=%q err=%v", path, err)
		} else {
			log.Printf("[INFO] [GeoIP] 成功加载 IP2Proxy BIN / Successfully loaded IP2Proxy BIN: path=%q", path)
		}
		return reader, err
	case DatabaseTypeQQWryIPDB:
		reader, err := NewQQWryIPDBReader(path)
		if err != nil {
			log.Printf("[ERROR] [GeoIP] QQWry IPDB 读取失败 / Failed to load QQWry IPDB: path=%q err=%v", path, err)
		} else {
			log.Printf("[INFO] [GeoIP] 成功加载 QQWry IPDB / Successfully loaded QQWry IPDB: path=%q", path)
		}
		return reader, err
	case DatabaseTypeQQWryDAT:
		reader, err := NewQQWryDatReader(path)
		if err != nil {
			log.Printf("[ERROR] [GeoIP] QQWry DAT 读取失败 / Failed to load QQWry DAT: path=%q err=%v", path, err)
		} else {
			log.Printf("[INFO] [GeoIP] 成功加载 QQWry DAT / Successfully loaded QQWry DAT: path=%q", path)
		}
		return reader, err
	case DatabaseTypeCZDB:
		key := ""
		if len(czdbKey) > 0 {
			key = czdbKey[0]
		}
		if key == "" {
			err := fmt.Errorf("czdb key required for file: %s (key is empty)", path)
			log.Printf("[ERROR] [GeoIP] CZDB 异常: 缺少解密密钥 / Missing CZDB key: path=%q", path)
			return nil, err
		}
		reader, err := NewCZDBReader(path, key, db.MEMORY)
		if err != nil {
			log.Printf("[ERROR] [GeoIP] CZDB 读取失败，请检查文件完整性及解密密钥(czdb_key)是否正确 / Failed to open CZDB (check key or file): path=%q key=%q err=%v", path, key, err)
			return nil, fmt.Errorf("failed to open czdb: %s, key: %q, error: %v", path, key, err)
		}
		log.Printf("[INFO] [GeoIP] 成功加载 CZDB / Successfully loaded CZDB: path=%q", path)
		return reader, nil
	default:
		err := fmt.Errorf("unsupported database type for file: %s", path)
		log.Printf("[ERROR] [GeoIP] 不支持的数据库格式 / Unsupported database type: path=%q", path)
		return nil, err
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
func (e *EmptyReader) Network(net.IP) (*net.IPNet, error)            { return nil, nil }
func (e *EmptyReader) ASN(net.IP) (ASN, error)                       { return ASN{}, nil }
func (e *EmptyReader) ISP(net.IP) (ISP, error)                       { return ISP{}, nil }
func (e *EmptyReader) ConnectionType(net.IP) (ConnectionType, error) { return ConnectionType{}, nil }
func (e *EmptyReader) Proxy(net.IP) (Proxy, error)                   { return Proxy{}, nil }

func (e *EmptyReader) IsEmpty() bool { return true }

// OpenAuto automatically detects database types and creates appropriate readers
func OpenAuto(databases ...string) (Reader, error) {
	log.Printf("[INFO] [GeoIP] 自动检测模式(Auto): 准备加载数据库列表 / Auto-detection mode loading databases: %v", databases)
	var readers []Reader
	var hasData bool

	for _, dbPath := range databases {
		if dbPath == "" {
			continue
		}

		// 兼容 czdb 密钥参数（此处可后续扩展为结构体/配置）
		reader, err := CreateReader(dbPath)
		if err != nil {
			log.Printf("[ERROR] [GeoIP] 自动模式(Auto)加载数据库失败 / Auto mode failed to create reader for %s: %v", dbPath, err)
			return nil, fmt.Errorf("failed to create reader for %s: %v", dbPath, err)
		}

		if !reader.IsEmpty() {
			readers = append(readers, reader)
			hasData = true
		}
	}

	if !hasData {
		log.Printf("[WARNING] [GeoIP] 自动模式(Auto): 未加载到任何有效的数据库文件 / No valid databases loaded in Auto mode")
		return &EmptyReader{}, nil
	}

	if len(readers) == 1 {
		log.Printf("[INFO] [GeoIP] 自动模式(Auto): 成功初始化单数据库读取器 / Initialized single reader in Auto mode")
		return readers[0], nil
	}

	log.Printf("[INFO] [GeoIP] 自动模式(Auto): 成功组合 %d 个数据库读取器 / Initialized combined reader with %d databases", len(readers), len(readers))
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
	absPath, _ := filepath.Abs(path)
	log.Printf("[INFO] [GeoIP] 准备加载数据库 / Loading database: label=%q path=%q (abs=%q)", label, path, absPath)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[ERROR] [GeoIP] 数据库文件不存在 / Database file does not exist: label=%q path=%q err=%v", label, path, err)
		} else {
			log.Printf("[ERROR] [GeoIP] 访问数据库文件失败 / Failed to access database file: label=%q path=%q err=%v", label, path, err)
		}
	} else {
		log.Printf("[DEBUG] [GeoIP] 数据库文件检查通过，正在打开 / Database file stat OK, opening: label=%q path=%q", label, path)
	}
	db, err := openFunc(path)
	if err != nil {
		log.Printf("[ERROR] [GeoIP] 数据库读取失败，格式不正确或损坏 / Failed to open/read database: label=%q path=%q err=%v", label, path, err)
	} else {
		log.Printf("[INFO] [GeoIP] 成功加载数据库 / Successfully loaded database: label=%q path=%q", label, path)
	}
	return db, err
}
