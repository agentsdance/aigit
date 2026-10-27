package llm

import (
	"fmt"
	"strings"
)

// BuildCommitPrompt shares the API generators' instructions with a calling agent.
func BuildCommitPrompt(diff, language string, emoji bool) string {
	return fmt.Sprintf("%s\n%s", getPrompt(language, emoji), diff)
}

// FormatAgentCommit validates an agent's structured response before rendering it.
// API generators retain their existing, more permissive response parsing.
func FormatAgentCommit(data CommitData, useEmoji bool) (string, error) {
	data.Type = strings.TrimSpace(data.Type)
	data.Scope = strings.TrimSpace(data.Scope)
	data.Description = strings.TrimSpace(data.Description)
	if data.Type == "" || data.Description == "" {
		return "", fmt.Errorf("type and description are required")
	}
	if strings.ContainsAny(data.Type+data.Scope+data.Description, "\r\n\x00") {
		return "", fmt.Errorf("type, scope and description must be single-line strings without NUL characters")
	}
	for _, line := range data.Body {
		if strings.ContainsRune(line, '\x00') {
			return "", fmt.Errorf("body must not contain NUL characters")
		}
	}
	// The shared renderer adds bullet prefixes in place; keep the caller's data intact.
	data.Body = append([]string(nil), data.Body...)
	return renderCommitMessage(&data, useEmoji), nil
}
