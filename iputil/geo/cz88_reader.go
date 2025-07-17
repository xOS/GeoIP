package geo

import "strings"

// splitCZDBFields 支持多种分隔符，保证字段分割准确
func splitCZDBFields(raw string) (country, region, city, street, isp string) {
	parts := strings.Split(raw, "\t")
	var regionCity string
	if len(parts) == 2 {
		regionCity = parts[0]
		isp = parts[1]
	} else {
		regionCity = raw
	}
	// 支持多种分隔符
	seps := []string{"-", "–", "—", "－", "_", "·"}
	var regionParts []string
	for _, sep := range seps {
		if strings.Contains(regionCity, sep) {
			regionParts = strings.Split(regionCity, sep)
			break
		}
	}
	if len(regionParts) == 0 {
		regionParts = []string{regionCity}
	}
	if len(regionParts) > 0 {
		country = regionParts[0]
	}
	if len(regionParts) > 1 {
		region = regionParts[1]
	}
	if len(regionParts) > 2 {
		city = regionParts[2]
	}
	if len(regionParts) > 3 {
		street = regionParts[3]
	}
	return
}
