package recipe

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// SafePath prevents directory traversal attacks
func SafePath(base, suffix string) string {
	// Basic sanitization
	suffix = filepath.Clean(suffix)
	if strings.HasPrefix(suffix, "..") {
		suffix = strings.ReplaceAll(suffix, "..", "")
	}
	return filepath.Join(base, suffix)
}

// ReplaceVars replaces {{varName}} in inputString with variables from ctx
func ReplaceVars(inputString string, ctx DeployerCtx) string {
	for varName, val := range ctx {
		// Ignore internal variables like dbConnection which aren't strings
		if varName == "dbConnection" {
			continue
		}
		strVal := fmt.Sprintf("%v", val)
		re := regexp.MustCompile(fmt.Sprintf(`\{\{%s\}\}`, regexp.QuoteMeta(varName)))
		inputString = re.ReplaceAllString(inputString, strVal)
	}
	return inputString
}

// GetString gets a string from a map, returning empty string if it doesn't exist or isn't a string
func GetString(m map[string]any, key string) string {
	val, ok := m[key]
	if !ok {
		return ""
	}
	str, ok := val.(string)
	if !ok {
		return ""
	}
	return str
}

func GetInteger(m map[string]any, key string, default_value int) int {
	val, ok := m[key]
	if !ok || val == nil {
		return default_value
	}

	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	case string: // e.g., "3306"
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return i
		}
	}

	return -1
}

func GetBool(ctx DeployerCtx, key string) bool {
	val, ok := ctx[key]
	if !ok || val == nil {
		return false
	}

	switch v := val.(type) {
	case bool:
		return v
	case string:
		v = strings.TrimSpace(strings.ToLower(v))
		if v == "" {
			return false
		}
		if v == "yes" || v == "y" {
			return true
		}
		b, _ := strconv.ParseBool(v)
		return b
	default:
		return false
	}
}
