package tools

import (
	"fmt"
	"strconv"
	"strings"
)

func parseOptionalInt(raw interface{}) (int, bool, error) {
	value, ok, err := parseOptionalInt64(raw)
	return int(value), ok, err
}

func parseOptionalInt32(raw interface{}) (int32, bool, error) {
	value, ok, err := parseOptionalInt64(raw)
	return int32(value), ok, err
}

func parseOptionalInt64(raw interface{}) (int64, bool, error) {
	if raw == nil {
		return 0, false, nil
	}

	switch v := raw.(type) {
	case float64:
		return int64(v), true, nil
	case float32:
		return int64(v), true, nil
	case int:
		return int64(v), true, nil
	case int32:
		return int64(v), true, nil
	case int64:
		return v, true, nil
	case uint:
		return int64(v), true, nil
	case uint32:
		return int64(v), true, nil
	case uint64:
		return int64(v), true, nil
	default:
		return 0, false, fmt.Errorf("unsupported type %T", raw)
	}
}

func splitStringArg(raw interface{}) []string {
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return nil
	}

	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})

	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			result = append(result, field)
		}
	}
	return result
}

func parseStringFloat64(raw string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(raw), 64)
}
