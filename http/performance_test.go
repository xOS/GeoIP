package http

import (
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// Import the testDb from http_test.go

// BenchmarkLangParameterPerformance 测试lang参数的性能
func BenchmarkLangParameterPerformance(b *testing.B) {
	server := &Server{
		cache: NewCache(1000),
		gr:    &testDb{},
	}

	testIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"114.114.114.114",
		"172.167.205.21",
		"129.154.53.170",
	}

	b.Run("WithoutLang", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ip := net.ParseIP(testIPs[i%len(testIPs)])
			req := &http.Request{
				URL:    &url.URL{RawQuery: ""},
				Header: http.Header{"X-Real-IP": []string{ip.String()}},
			}
			_, _ = server.newResponse(req)
		}
	})

	b.Run("WithLangZh", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ip := net.ParseIP(testIPs[i%len(testIPs)])
			req := &http.Request{
				URL:    &url.URL{RawQuery: "lang=zh"},
				Header: http.Header{"X-Real-IP": []string{ip.String()}},
			}
			_, _ = server.newResponse(req)
		}
	})

	b.Run("WithLangEn", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ip := net.ParseIP(testIPs[i%len(testIPs)])
			req := &http.Request{
				URL:    &url.URL{RawQuery: "lang=en"},
				Header: http.Header{"X-Real-IP": []string{ip.String()}},
			}
			_, _ = server.newResponse(req)
		}
	})
}

// BenchmarkCachePerformance 测试缓存性能
func BenchmarkCachePerformance(b *testing.B) {
	cache := NewCache(1000)
	ip := net.ParseIP("192.0.2.1")
	resp := Response{IP: ip, Country: "Test"}

	b.Run("CacheSet", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.SetWithLang(ip, "zh", resp)
		}
	})

	b.Run("CacheGet", func(b *testing.B) {
		cache.SetWithLang(ip, "zh", resp)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cache.GetWithLang(ip, "zh")
		}
	})

	b.Run("CacheSetDifferentIPs", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			testIP := net.ParseIP("192.0.2." + string(rune(i%254+1)))
			cache.SetWithLang(testIP, "zh", Response{IP: testIP})
		}
	})
}

// BenchmarkStringOperations 测试字符串操作性能
func BenchmarkStringOperations(b *testing.B) {
	b.Run("CountryNameLookup", func(b *testing.B) {
		countryCodes := []string{"US", "GB", "CN", "JP", "KR", "DE", "FR", "CA", "AU", "RU"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			code := countryCodes[i%len(countryCodes)]
			getChineseCountryName(code, "")
		}
	})

	b.Run("CoordinateValidation", func(b *testing.B) {
		testCases := []struct {
			country  string
			lat, lon float64
		}{
			{"US", 40.7128, -74.0060},
			{"GB", 51.5074, -0.1278},
			{"CN", 39.9042, 116.4074},
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tc := testCases[i%len(testCases)]
			validateCountryByCoordinates(tc.country, tc.lat, tc.lon)
		}
	})
}

// TestPerformanceRegression 性能回归测试
func TestPerformanceRegression(t *testing.T) {
	server := &Server{
		cache: NewCache(100),
		gr:    &testDb{},
	}

	ip := net.ParseIP("8.8.8.8")

	// 测试响应时间不应该超过合理阈值
	start := time.Now()
	for i := 0; i < 100; i++ {
		req := &http.Request{
			URL:    &url.URL{RawQuery: "lang=zh"},
			Header: http.Header{"X-Real-IP": []string{ip.String()}},
		}
		_, _ = server.newResponse(req)
	}
	duration := time.Since(start)

	// 100次查询应该在100ms内完成（包括缓存）
	if duration > 100*time.Millisecond {
		t.Errorf("Performance regression: 100 lookups took %v, expected < 100ms", duration)
	}
}
