package multiagent

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// English note.
// English note.
// English note.
const maxToolCallRecoveryAttempts = 5

// English note.
func toolCallArgumentsJSONRetryHint() *schema.Message {
	return schema.UserMessage(`[] ， function.arguments  JSON，。： tool call  arguments 、 JSON （，，）。 JSON。

[System] Your previous tool call used invalid JSON in function.arguments and was rejected by the API. Regenerate with strictly valid JSON objects only (double-quoted keys, matched braces, no trailing commas).`)
}

// English note.
func toolCallArgumentsJSONRecoveryTimelineMessage(attempt int) string {
	return fmt.Sprintf(
		" JSON。 function.arguments。"+
			" %d/%d 。\n\n"+
			"The API rejected invalid JSON in tool arguments. A system hint was appended. This is full run %d of %d.",
		attempt+1, maxToolCallRecoveryAttempts, attempt+1, maxToolCallRecoveryAttempts,
	)
}

// English note.
func isRecoverableToolCallArgumentsJSONError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if !strings.Contains(s, "json") {
		return false
	}
	if strings.Contains(s, "function.arguments") || strings.Contains(s, "function arguments") {
		return true
	}
	if strings.Contains(s, "invalidparameter") && strings.Contains(s, "json") {
		return true
	}
	if strings.Contains(s, "must be in json format") {
		return true
	}
	return false
}
