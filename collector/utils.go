package collector

import (
	"strconv"
	"strings"
)

func sumOrAppend(kv map[string]float64, k string, v float64) {
	if _, ok := kv[k]; ok {
		kv[k] += v
	} else {
		kv[k] = v
	}
}

func parseMetricValue(flag string, value string) (float64, error) {
	var valueInt int64
	var valueFloat float64
	var err error

	switch flag {
	case reqRateReqPerSecField, reqRatePubCacheHitsPerSecField, reqRatePrivateCacheHitsPerSecField, reqRateStaticHitsPerSecField /*, extappReqPerSecField*/ :
		valueFloat, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, err
		}
	default:
		valueInt, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, err
		}
		valueFloat = float64(valueInt)
	}

	return valueFloat, nil
}

func parseKeyValPair(keyVal string, separator string) (string, string) {
	parts := strings.Split(keyVal, separator)
	return parts[0], parts[1]
}

func parseKeyValLineToMap(line string) map[string]string {
	mapped := make(map[string]string)
	parts := strings.Split(line, ", ")
	for _, pair := range parts {
		separator := ": "
		if strings.Contains(pair, separator) {
			key, value := parseKeyValPair(pair, separator)
			mapped[key] = value
		}
	}
	return mapped
}

// ParseFlagsToMap converts array of strings to a boolean map
func ParseFlagsToMap(s []string) map[string]bool {
	m := map[string]bool{}
	for _, v := range s {
		m[v] = true
	}
	return m
}
