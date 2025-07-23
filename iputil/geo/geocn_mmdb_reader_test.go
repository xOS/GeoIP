package geo

import (
	"net"
	"os"
	"testing"
)

// TestGeoCNMMDBReader 测试GeoCN.mmdb读取器的基本功能
func TestGeoCNMMDBReader(t *testing.T) {
	// 检查GeoCNMMDBReader是否实现了Reader接口
	var _ Reader = &GeoCNMMDBReader{}
	
	t.Log("GeoCNMMDBReader correctly implements Reader interface")
}

// TestGeoCNMMDBReaderMethods 测试GeoCN.mmdb读取器的方法
func TestGeoCNMMDBReaderMethods(t *testing.T) {
	// 创建一个空的GeoCNMMDBReader实例进行测试
	reader := &GeoCNMMDBReader{}
	
	// 测试IsEmpty方法
	if !reader.IsEmpty() {
		t.Error("Expected reader to be empty")
	}
	
	// 测试Network方法
	network, err := reader.Network(net.ParseIP("1.1.1.1"))
	if err != nil {
		// 这是预期的，因为空reader无法提供网络信息
	} else if network != nil {
		t.Errorf("Expected nil network, got %+v", network)
	}
	
	// 测试Country方法
	country, err := reader.Country(net.ParseIP("1.1.1.1"))
	if err != nil {
		// 这是预期的，因为空reader无法提供国家信息
	} else if country.Name != "" || country.ISO != "" {
		t.Errorf("Expected empty country, got %+v", country)
	}
	
	// 测试City方法
	city, err := reader.City(net.ParseIP("1.1.1.1"))
	if err != nil {
		// 这是预期的，因为空reader无法提供城市信息
	} else if city.Name != "" || city.RegionName != "" {
		t.Errorf("Expected empty city, got %+v", city)
	}
	
	// 测试ASN方法
	asn, err := reader.ASN(net.ParseIP("1.1.1.1"))
	if err != nil {
		// 这是预期的，因为空reader无法提供ASN信息
	} else if asn.AutonomousSystemNumber != 0 {
		t.Errorf("Expected zero ASN, got %+v", asn)
	}
	
	// 测试ISP方法
	isp, err := reader.ISP(net.ParseIP("1.1.1.1"))
	if err != nil {
		// 这是预期的，因为空reader无法提供ISP信息
	} else if isp.ISP != "" || isp.Organization != "" {
		t.Errorf("Expected empty ISP, got %+v", isp)
	}
	
	// 测试ConnectionType方法
	connType, err := reader.ConnectionType(net.ParseIP("1.1.1.1"))
	if err != nil {
		// 这是预期的，因为空reader无法提供连接类型信息
	} else if connType.ConnectionType != "" {
		t.Errorf("Expected empty connection type, got %+v", connType)
	}
	
	// 测试Proxy方法
	proxy, err := reader.Proxy(net.ParseIP("1.1.1.1"))
	if err != nil {
		// 这是预期的，因为空reader无法提供代理信息
	} else if proxy.IsProxy {
		t.Errorf("Expected non-proxy, got %+v", proxy)
	}
	
	// 测试Network方法
	network, err = reader.Network(net.ParseIP("1.1.1.1"))
	if err != nil {
		// 这是预期的，因为空reader无法提供网络信息
	} else if network != nil {
		t.Errorf("Expected nil network, got %+v", network)
	}
	
	t.Log("All GeoCNMMDBReader methods work correctly")
}

// TestGeoCNMMDBReaderWithDatabase 测试带有数据库的GeoCN.mmdb读取器
func TestGeoCNMMDBReaderWithDatabase(t *testing.T) {
	// 检查data目录下是否存在GeoCN.mmdb文件
	if _, err := os.Stat("data/GeoCN.mmdb"); os.IsNotExist(err) {
		t.Skip("GeoCN.mmdb not found, skipping test")
	}
	
	// 创建GeoCN MMDB读取器
	reader, err := NewGeoCNMMDBReader("data/GeoCN.mmdb")
	if err != nil {
		t.Fatalf("Failed to create GeoCN MMDB reader: %v", err)
	}
	defer reader.Close()
	
	// 测试一些已知的中国IP地址
	testCases := []struct {
		ip          string
		expectRegion bool
		expectISP    bool
	}{
		{"114.114.114.114", true, false}, // China DNS
		{"223.5.5.5", true, true},        // AliDNS
		{"202.96.128.86", true, true},    // Shanghai Telecom
	}
	
	for _, tc := range testCases {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Errorf("Invalid IP address: %s", tc.ip)
			continue
		}
		
		// 测试City信息
		city, err := reader.City(ip)
		if err != nil {
			t.Errorf("City lookup failed for %s: %v", tc.ip, err)
		} else if tc.expectRegion && city.RegionName == "" {
			t.Errorf("Expected region for %s, got empty", tc.ip)
		}
		
		// 测试ISP信息
		isp, err := reader.ISP(ip)
		if err != nil {
			t.Errorf("ISP lookup failed for %s: %v", tc.ip, err)
		} else if tc.expectISP && isp.ISP == "" {
			t.Errorf("Expected ISP for %s, got empty", tc.ip)
		}
		
		// 测试Network信息
		network, err := reader.Network(ip)
		if err != nil {
			t.Errorf("Network lookup failed for %s: %v", tc.ip, err)
		}
		// network可以为空，也可以是一个有效的IPNet
		// 我们只验证它不会返回错误
		t.Logf("Network for %s: %+v", tc.ip, network)
	}
	
	t.Log("GeoCNMMDBReaderWithDatabase test completed")
}

// TestNetworkMethod 测试各种数据库实现的Network方法
func TestNetworkMethod(t *testing.T) {
	// 测试空的GeoCNMMDBReader
	t.Run("EmptyGeoCNMMDBReader", func(t *testing.T) {
		reader := &GeoCNMMDBReader{}
		network, err := reader.Network(net.ParseIP("1.1.1.1"))
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if network != nil {
			t.Errorf("Expected nil network, got %+v", network)
		}
	})
	
	// 测试空的CZDBReader
	t.Run("EmptyCZDBReader", func(t *testing.T) {
		reader := &CZDBReader{}
		network, err := reader.Network(net.ParseIP("1.1.1.1"))
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if network != nil {
			t.Errorf("Expected nil network, got %+v", network)
		}
	})
	
	// 测试空的QQWryIPDBReader
	t.Run("EmptyQQWryIPDBReader", func(t *testing.T) {
		reader := &QQWryIPDBReader{}
		network, err := reader.Network(net.ParseIP("1.1.1.1"))
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if network != nil {
			t.Errorf("Expected nil network, got %+v", network)
		}
	})
	
	// 测试空的IP2LocationBinReader
	t.Run("EmptyIP2LocationBinReader", func(t *testing.T) {
		reader := &IP2LocationBinReader{}
		network, err := reader.Network(net.ParseIP("1.1.1.1"))
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if network != nil {
			t.Errorf("Expected nil network, got %+v", network)
		}
	})
	
	// 测试空的IP2ProxyBinReader
	t.Run("EmptyIP2ProxyBinReader", func(t *testing.T) {
		reader := &IP2ProxyBinReader{}
		network, err := reader.Network(net.ParseIP("1.1.1.1"))
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if network != nil {
			t.Errorf("Expected nil network, got %+v", network)
		}
	})
}

// BenchmarkGeoCNMMDBReader 测试GeoCN.mmdb读取器的性能
func BenchmarkGeoCNMMDBReader(b *testing.B) {
	reader := &GeoCNMMDBReader{}
	ip := net.ParseIP("1.1.1.1")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = reader.Country(ip)
		_, _ = reader.City(ip)
		_, _ = reader.ASN(ip)
		_, _ = reader.ISP(ip)
		_, _ = reader.ConnectionType(ip)
		_, _ = reader.Proxy(ip)
	}
}