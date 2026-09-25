package delivery

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func SyntheticEntries(raw string) []Entry {
	parts := splitSubscriptionEntries(raw)
	entries := make([]Entry, 0, len(parts))
	for index, part := range parts {
		entries = append(entries, Entry{ID: int64(index + 1), Raw: part, Kind: model.KeyKindReal, Label: fmt.Sprintf("proxy-%d", index+1)})
	}
	return entries
}

// splitSubscriptionEntries splits a pasted body into one entry per share link,
// keeping a pretty-printed JSON document together until it parses.
func splitSubscriptionEntries(raw string) []string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	entries := make([]string, 0, len(lines))
	var jsonBuffer strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if jsonBuffer.Len() == 0 && !strings.HasPrefix(trimmed, "{") {
			entries = append(entries, trimmed)
			continue
		}
		if entry, complete := appendJSONLine(&jsonBuffer, trimmed); complete {
			entries = append(entries, entry)
		}
	}
	if jsonBuffer.Len() > 0 {
		entries = append(entries, jsonBuffer.String())
	}
	return entries
}

// appendJSONLine adds one line to the pending JSON document and, once the
// document parses, hands it back and resets the buffer.
func appendJSONLine(buffer *strings.Builder, line string) (string, bool) {
	if buffer.Len() > 0 {
		buffer.WriteByte('\n')
	}
	buffer.WriteString(line)
	if !json.Valid([]byte(buffer.String())) {
		return "", false
	}
	entry := buffer.String()
	buffer.Reset()
	return entry, true
}
