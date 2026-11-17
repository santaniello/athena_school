package mindmap

import (
	"bytes"
	"strings"
)

// stripCodeFence removes a Markdown code fence some models wrap around JSON.
func stripCodeFence(content string) []byte {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "```") {
		return []byte(trimmed)
	}
	trimmed = strings.TrimPrefix(trimmed, "```")
	if _, afterInfo, found := strings.Cut(trimmed, "\n"); found {
		trimmed = afterInfo
	}
	return bytes.TrimSpace([]byte(strings.TrimSuffix(strings.TrimSpace(trimmed), "```")))
}
