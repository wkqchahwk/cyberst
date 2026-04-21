package multiagent

import (
	"encoding/json"
	"strings"
)

// English note.
// English note.
func UnwrapPlanExecuteUserText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return s
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return s
	}
	for _, key := range []string{
		"response", "answer", "message", "content", "output",
		"final_answer", "reply", "text", "result_text",
	} {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		str, ok := v.(string)
		if !ok {
			continue
		}
		if t := strings.TrimSpace(str); t != "" {
			return t
		}
	}
	return s
}
