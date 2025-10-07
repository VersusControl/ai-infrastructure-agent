package utilities

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ========== Helper Functions ==========

// Helper function to safely extract string from map
func GetStringFromMap(m map[string]interface{}, key string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func GetStringSlice(params map[string]interface{}, key string) []string {
	if val, ok := params[key].([]interface{}); ok {
		result := make([]string, len(val))
		for i, v := range val {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result
	}

	// Try []string directly
	if val, ok := params[key].([]string); ok {
		return val
	}

	// Try JSON string (from dependency resolution of arrays)
	if val, ok := params[key].(string); ok && val != "" {
		var jsonArray []string
		if err := json.Unmarshal([]byte(val), &jsonArray); err == nil {
			return jsonArray
		}
	}

	return []string{}
}

func GetStringMap(params map[string]interface{}, key string) map[string]string {
	if val, ok := params[key].(map[string]interface{}); ok {
		result := make(map[string]string)
		for k, v := range val {
			if str, ok := v.(string); ok {
				result[k] = str
			}
		}
		return result
	}
	return map[string]string{}
}

func GetInt32FromMap(params map[string]interface{}, key string, defaultVal int32) int32 {
	if val, ok := params[key].(float64); ok {
		return int32(val)
	}
	if val, ok := params[key].(int); ok {
		return int32(val)
	}
	if val, ok := params[key].(int32); ok {
		return val
	}
	return defaultVal
}

func GetBoolFromMap(params map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultVal
}

// getMapKeys returns the keys of a map for debugging purposes
func GetMapKeys(m map[string]interface{}) []string {
	if m == nil {
		return []string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// isInternalField checks if a field name is for internal system use
func IsInternalField(fieldName string) bool {
	internalFields := []string{"status", "message", "timestamp", "duration", "metadata", "error"}
	fieldLower := strings.ToLower(fieldName)

	for _, internal := range internalFields {
		if fieldLower == internal {
			return true
		}
	}
	return false
}

func Title(text string) string {
	c := cases.Title(language.English)
	return c.String(text)
}

// FindProjectRoot walks up the directory tree to find the project root (where go.mod exists)
func FindProjectRoot(startPath string) string {
	currentPath := startPath
	for {
		// Check if go.mod exists in current directory
		goModPath := filepath.Join(currentPath, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return currentPath
		}

		// Move up one directory
		parentPath := filepath.Dir(currentPath)

		// If we've reached the root or can't go further up, stop
		if parentPath == currentPath || parentPath == "/" {
			break
		}

		currentPath = parentPath
	}
	return ""
}
