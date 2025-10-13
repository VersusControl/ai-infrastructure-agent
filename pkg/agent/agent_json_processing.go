package agent

import (
	"encoding/json"
	"strings"
)

// ========== Interface defines ==========

// JSONProcessingInterface defines JSON parsing and processing functionality for AI agent responses
//
// Available Functions:
//   - extractJSON()                : Extract complete JSON object from text with nested brace handling
//   - extractJSONAlternative()     : Alternative JSON extraction method for complex text
//   - extractJSONFromPosition()    : Extract JSON starting from specific position
//   - isValidJSON()                : Validate if string is valid JSON
//   - attemptTruncatedJSONParse()  : Attempt to parse potentially truncated JSON responses
//   - findLastValidJSON()          : Find the last valid JSON object in text
//   - cleanJSONComments()          : Remove JavaScript-style comments from JSON text
//
// Usage Example:
//   1. jsonStr := agent.extractJSON(aiResponse)
//   2. decision := agent.parseAgentDecision(jsonStr, decisionID, request)
//   3. agent.validateAgentDecisionJSON(decision)

// ========== JSON Processing and Validation Functions ==========

// extractJSON extracts a complete JSON object from text, handling nested braces
func (a *StateAwareAgent) extractJSON(text string) string {
	// Find the first opening brace
	start := strings.Index(text, "{")
	if start == -1 {
		return ""
	}

	// Count braces to find the matching closing brace
	braceCount := 0
	inString := false
	escaped := false

	for i := start; i < len(text); i++ {
		char := text[i]

		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
				if braceCount == 0 {
					return text[start : i+1]
				}
			}
		}
	}

	return ""
}

// extractJSONAlternative tries alternative methods to extract JSON from text
func (a *StateAwareAgent) extractJSONAlternative(text string) string {
	// Method 1: Look for JSON in markdown code blocks
	if strings.Contains(text, "```json") {
		start := strings.Index(text, "```json")
		if start != -1 {
			start += 7 // Skip "```json"
			end := strings.Index(text[start:], "```")
			if end != -1 {
				jsonCandidate := strings.TrimSpace(text[start : start+end])
				if a.isValidJSON(jsonCandidate) {
					return jsonCandidate
				}
			}
		}
	}

	// Method 2: Look for JSON in regular code blocks
	if strings.Contains(text, "```") {
		start := strings.Index(text, "```")
		if start != -1 {
			start += 3 // Skip "```"
			// Skip language identifier if present
			if newlinePos := strings.Index(text[start:], "\n"); newlinePos != -1 {
				start += newlinePos + 1
			}
			end := strings.Index(text[start:], "```")
			if end != -1 {
				jsonCandidate := strings.TrimSpace(text[start : start+end])
				if a.isValidJSON(jsonCandidate) {
					return jsonCandidate
				}
			}
		}
	}

	// Method 3: Look for any JSON-like structure (starts with { and reasonable content)
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			// Try to extract from this position
			extracted := a.extractJSONFromPosition(text, i)
			if extracted != "" && a.isValidJSON(extracted) {
				return extracted
			}
		}
	}

	return ""
}

// extractJSONFromPosition extracts JSON starting from a specific position
func (a *StateAwareAgent) extractJSONFromPosition(text string, start int) string {
	braceCount := 0
	inString := false
	escaped := false

	for i := start; i < len(text); i++ {
		char := text[i]

		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
				if braceCount == 0 {
					return text[start : i+1]
				}
			}
		}
	}

	return ""
}

// isValidJSON checks if a string is valid JSON
func (a *StateAwareAgent) isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

// attemptTruncatedJSONParse attempts to parse potentially truncated JSON responses
func (a *StateAwareAgent) attemptTruncatedJSONParse(response string) string {
	// Check if this looks like the start of a valid JSON but might be incomplete
	if !strings.HasPrefix(response, "{") {
		return ""
	}

	// Try to parse as-is first
	if a.isValidJSON(response) {
		return response
	}

	// If response appears truncated, try to complete it minimally
	// Look for common truncation patterns
	truncatedResponse := strings.TrimSpace(response)

	// If it ends with a quote but no closing brace, it might be truncated mid-string
	if strings.HasSuffix(truncatedResponse, "\"") {
		// Try adding closing quote and braces
		completed := truncatedResponse + "\"}}}"
		if a.isValidJSON(completed) {
			a.Logger.Info("Successfully completed truncated JSON response")
			return completed
		}
	}

	// If it ends without proper closure, try adding closing braces
	if !strings.HasSuffix(truncatedResponse, "}") {
		// Count open braces and try to close them
		openBraces := strings.Count(truncatedResponse, "{") - strings.Count(truncatedResponse, "}")
		if openBraces > 0 {
			completed := truncatedResponse + strings.Repeat("}", openBraces)
			if a.isValidJSON(completed) {
				a.Logger.Info("Successfully completed truncated JSON response by adding closing braces")
				return completed
			}
		}
	}

	// Try to find the last complete key-value pair and truncate there
	lastValidJson := a.findLastValidJSON(truncatedResponse)
	if lastValidJson != "" {
		a.Logger.Info("Found last valid JSON portion from truncated response")
		return lastValidJson
	}

	return ""
}

// findLastValidJSON attempts to find the last valid JSON by progressively removing content
func (a *StateAwareAgent) findLastValidJSON(text string) string {
	// Work backwards to find valid JSON
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '}' {
			candidate := text[:i+1]
			if a.isValidJSON(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// cleanJSONComments removes JavaScript-style comments from JSON that AI models sometimes include
func (a *StateAwareAgent) cleanJSONComments(jsonStr string) string {
	lines := strings.Split(jsonStr, "\n")
	var cleanedLines []string

	for _, line := range lines {
		// Find the position of // comment (but not inside strings)
		inString := false
		escaped := false
		commentPos := -1

		for i, char := range line {
			if escaped {
				escaped = false
				continue
			}

			if char == '\\' {
				escaped = true
				continue
			}

			if char == '"' {
				inString = !inString
				continue
			}

			// If we're not inside a string and find //, mark it as comment start
			if !inString && char == '/' && i+1 < len(line) && line[i+1] == '/' {
				commentPos = i
				break
			}
		}

		// Remove the comment part if found
		if commentPos != -1 {
			line = strings.TrimSpace(line[:commentPos])
		}

		// Skip empty lines that resulted from comment removal
		if strings.TrimSpace(line) != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, "\n")
}
