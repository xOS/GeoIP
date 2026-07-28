package geo

import (
	"testing"
)

func TestMergeCityWithPriority_PrecisionFirst(t *testing.T) {
	// Case 1: Vague Province-only primary (e.g. CZDB says 四川 with empty City) vs Exact City-level supplement (e.g. MaxMind says Zhengzhou, Henan)
	// Precision-First rule: Exact city-level record must win over coarse province-only guess!
	primary := City{
		RegionName: "四川",
	}
	supplement := City{
		Name:       "Zhengzhou",
		RegionName: "Henan",
		RegionCode: "HA",
		PostalCode: "450000",
		Latitude:   34.7599,
		Longitude:  113.6459,
	}

	merged := mergeCityWithPriority(primary, supplement)
	if merged.Name != "Zhengzhou" {
		t.Errorf("Expected Name 'Zhengzhou' (Precision-First win), got '%s'", merged.Name)
	}
	if merged.RegionName != "Henan" {
		t.Errorf("Expected RegionName 'Henan', got '%s'", merged.RegionName)
	}
	if merged.RegionCode != "HA" {
		t.Errorf("Expected RegionCode 'HA', got '%s'", merged.RegionCode)
	}
}

func TestMergeCityWithPriority_DomesticPriorityWhenEqualPrecision(t *testing.T) {
	// Case 2: When both primary (Domestic DB) and supplement (Foreign DB) have equal precision (both exact City-level),
	// Domestic priority rule: Domestic database wins, and conflicting geographic fields from foreign DB are rejected!
	domesticPrimary := City{
		Name:       "成都",
		RegionName: "四川",
	}
	foreignSupplement := City{
		Name:       "Zhengzhou",
		RegionName: "Henan",
		RegionCode: "HA",
		PostalCode: "450000",
	}

	merged := mergeCityWithPriority(domesticPrimary, foreignSupplement)
	if merged.Name != "成都" {
		t.Errorf("Expected Name '成都' (Domestic Priority win), got '%s'", merged.Name)
	}
	if merged.RegionName != "四川" {
		t.Errorf("Expected RegionName '四川', got '%s'", merged.RegionName)
	}
	if merged.RegionCode != "" {
		t.Errorf("Expected empty RegionCode due to region conflict rejection, got '%s'", merged.RegionCode)
	}
}

func TestMergeCityWithPriority_CompatibleSupplementation(t *testing.T) {
	// Case 3: When primary and supplement point to compatible regions (e.g. 四川 and Sichuan),
	// Non-conflicting fields (like PostalCode and RegionCode) are safely supplemented!
	primary := City{
		Name:       "成都",
		RegionName: "四川",
	}
	compatibleSupp := City{
		Name:       "Chengdu",
		RegionName: "Sichuan",
		RegionCode: "SC",
		PostalCode: "610000",
	}

	merged := mergeCityWithPriority(primary, compatibleSupp)
	if merged.Name != "成都" {
		t.Errorf("Expected Name '成都', got '%s'", merged.Name)
	}
	if merged.RegionName != "四川" {
		t.Errorf("Expected RegionName '四川', got '%s'", merged.RegionName)
	}
	if merged.RegionCode != "SC" {
		t.Errorf("Expected RegionCode 'SC', got '%s'", merged.RegionCode)
	}
	if merged.PostalCode != "610000" {
		t.Errorf("Expected PostalCode '610000', got '%s'", merged.PostalCode)
	}
}
