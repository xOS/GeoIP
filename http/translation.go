package http

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// TranslationManager 管理翻译数据
type TranslationManager struct {
	mu           sync.RWMutex
	translations map[string]map[string]string // [language][key] = translation
	enabled      bool
}

// TranslationFile 翻译文件结构 - 只支持简体中文
type TranslationFile struct {
	Countries map[string]string `json:"countries,omitempty"` // [country_code] = chinese_name
	Regions   map[string]string `json:"regions,omitempty"`   // [region_name] = chinese_name
	Cities    map[string]string `json:"cities,omitempty"`    // [city_name] = chinese_name
}

// NewTranslationManager 创建新的翻译管理器
func NewTranslationManager() *TranslationManager {
	return &TranslationManager{
		translations: make(map[string]map[string]string),
		enabled:      false,
	}
}

// LoadFromFile 从文件加载翻译数据
func (tm *TranslationManager) LoadFromFile(filename string) error {
	if filename == "" {
		// 没有翻译文件，使用内置翻译
		tm.loadBuiltinTranslations()
		return nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		// 文件读取失败，回退到内置翻译
		log.Printf("[WARNING] 无法读取翻译文件 / Failed to read translation file: path=%q err=%v (using builtin translations)", filename, err)
		tm.loadBuiltinTranslations()
		return nil
	}

	var translationFile TranslationFile
	if err := json.Unmarshal(data, &translationFile); err != nil {
		// 文件解析失败，回退到内置翻译
		log.Printf("[WARNING] 翻译文件解析失败 / Failed to parse translation file: path=%q err=%v (using builtin translations)", filename, err)
		tm.loadBuiltinTranslations()
		return nil
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 清空现有翻译
	tm.translations = make(map[string]map[string]string)

	// 初始化中文翻译map
	if tm.translations["zh"] == nil {
		tm.translations["zh"] = make(map[string]string)
	}

	// 加载国家翻译
	for countryCode, chineseName := range translationFile.Countries {
		tm.translations["zh"]["country_"+countryCode] = chineseName
	}

	// 加载地区翻译
	for regionName, chineseName := range translationFile.Regions {
		tm.translations["zh"]["region_"+regionName] = chineseName
	}

	// 加载城市翻译
	for cityName, chineseName := range translationFile.Cities {
		tm.translations["zh"]["city_"+cityName] = chineseName
	}

	tm.enabled = true
	log.Printf("[INFO] [GeoIP] 成功加载翻译文件 / Successfully loaded translation file: path=%q", filename)
	return nil
}

// loadBuiltinTranslations 加载内置翻译（保持现有逻辑）
func (tm *TranslationManager) loadBuiltinTranslations() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.translations = make(map[string]map[string]string)
	tm.enabled = true // 内置翻译也算启用

	// 这里不加载任何翻译，保持现有的硬编码翻译逻辑
	log.Println("[INFO] [GeoIP] 使用内置默认翻译逻辑 / Using builtin translation logic")
}

// GetCountryName 获取国家名翻译
func (tm *TranslationManager) GetCountryName(countryCode, lang, fallback string) string {
	if !tm.enabled {
		return fallback
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if langMap, exists := tm.translations[lang]; exists {
		if translation, exists := langMap["country_"+countryCode]; exists {
			return translation
		}
	}

	// 如果翻译文件中没有找到，使用内置翻译逻辑
	if lang == "zh" {
		return getChineseCountryName(countryCode, fallback)
	}

	return fallback
}

// GetRegionName 获取地区名翻译
func (tm *TranslationManager) GetRegionName(countryCode, regionName, lang, fallback string) string {
	if !tm.enabled {
		return fallback
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if langMap, exists := tm.translations[lang]; exists {
		// 先尝试完整的地区名
		if translation, exists := langMap["region_"+regionName]; exists {
			return translation
		}
		// 再尝试国家代码+地区名的组合
		if translation, exists := langMap["region_"+countryCode+"_"+regionName]; exists {
			return translation
		}
	}

	// 如果翻译文件中没有找到，使用内置翻译逻辑
	if lang == "zh" {
		return getChineseRegionName(countryCode, regionName)
	}

	return fallback
}

// GetCityName 获取城市名翻译
func (tm *TranslationManager) GetCityName(countryCode, cityName, lang, fallback string) string {
	if !tm.enabled {
		return fallback
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if langMap, exists := tm.translations[lang]; exists {
		// 先尝试完整的城市名
		if translation, exists := langMap["city_"+cityName]; exists {
			return translation
		}
		// 再尝试国家代码+城市名的组合
		if translation, exists := langMap["city_"+countryCode+"_"+cityName]; exists {
			return translation
		}
	}

	// 如果翻译文件中没有找到，使用内置翻译逻辑
	if lang == "zh" {
		return getChineseCityName(countryCode, cityName)
	}

	return fallback
}

// IsEnabled 检查翻译管理器是否启用
func (tm *TranslationManager) IsEnabled() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.enabled
}

// GetStats 获取翻译统计信息
func (tm *TranslationManager) GetStats() map[string]int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := make(map[string]int)
	for lang, translations := range tm.translations {
		stats[lang] = len(translations)
	}
	return stats
}

// 全局翻译管理器实例
var globalTranslationManager = NewTranslationManager()

// InitTranslations 初始化翻译系统
func InitTranslations(translationFile string) error {
	return globalTranslationManager.LoadFromFile(translationFile)
}

// GetTranslatedCountryName 获取翻译后的国家名
func GetTranslatedCountryName(countryCode, lang, fallback string) string {
	return globalTranslationManager.GetCountryName(countryCode, lang, fallback)
}

// GetTranslatedRegionName 获取翻译后的地区名
func GetTranslatedRegionName(countryCode, regionName, lang, fallback string) string {
	return globalTranslationManager.GetRegionName(countryCode, regionName, lang, fallback)
}

// GetTranslatedCityName 获取翻译后的城市名
func GetTranslatedCityName(countryCode, cityName, lang, fallback string) string {
	return globalTranslationManager.GetCityName(countryCode, cityName, lang, fallback)
}
