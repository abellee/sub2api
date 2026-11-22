package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

// Hold one bounded SSE event until we can distinguish an error from output.
// Also hold the message preamble so a failed attempt cannot start a second
// message in the client's stream. Large preambles are flushed, ending replay.
type anthropicFailoverEvents struct {
	lines         []string
	preamble      []string
	preambleBytes int
	maxBytes      int
	outputStarted bool
}

func (b *anthropicFailoverEvents) push(line string) ([]string, *sseStreamErrorEventError) {
	b.lines = append(b.lines, line)
	if line != "" {
		return nil, nil
	}
	lines := b.lines
	b.lines = nil
	var event string
	var data []string
	for _, part := range lines {
		if strings.HasPrefix(part, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(part, "event:"))
		}
		if value, ok := extractAnthropicSSEDataLine(part); ok {
			data = append(data, value)
		}
	}
	payload := strings.Join(data, "\n")
	kind := gjson.Get(payload, "type").String()
	if event == "error" || kind == "error" || gjson.Get(payload, "error").IsObject() {
		if strings.TrimSpace(payload) == "" {
			payload = `{"type":"error","error":{"type":"upstream_error","message":"Upstream stream failed"}}`
		}
		b.preamble = nil
		return nil, &sseStreamErrorEventError{RawData: payload}
	}
	if kind == "" {
		kind = event
	}
	if !b.outputStarted {
		switch kind {
		case "ping", "":
			// Keepalive and comments do not commit model output.
			return lines, nil
		case "message_start", "content_block_start":
			for _, part := range lines {
				b.preambleBytes += len(part) + 1
			}
			b.preamble = append(b.preamble, lines...)
			if b.preambleBytes <= b.maxBytes {
				return nil, nil
			}
			lines, b.preamble = b.preamble, nil
		default:
			lines = append(b.preamble, lines...)
			b.preamble = nil
		}
		b.outputStarted = true
	}
	return lines, nil
}
