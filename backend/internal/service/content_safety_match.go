package service

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

var contentSafetyQuotes = strings.NewReplacer("’", "'", "‘", "'", "ʼ", "'", "＇", "'")

func normalizeContentSafetyText(text string) string {
	return strings.ToLower(contentSafetyQuotes.Replace(text))
}

// These literal masked spellings also occur in upstream refusal messages.
var contentSafetyRefusalPhrases = []string{
	"i can't help with", "i cannot help with", "i c***t help with", "i ca*** help with",
}

func contentSafetyRefusal(text string) string {
	for _, phrase := range contentSafetyRefusalPhrases {
		rest := text
		for {
			i := strings.Index(rest, phrase)
			if i < 0 {
				break
			}
			rest = rest[i+len(phrase):]
			suffix := strings.TrimSpace(rest)
			for _, prefix := range []string{"that", "this", "you", "your", "the request", "the prompt", "it"} {
				if strings.HasPrefix(suffix, prefix) {
					return phrase + " " + prefix
				}
			}
		}
	}
	return ""
}

type contentSafetyProbe struct {
	response *http.Response
	tails    map[string]string
}

func (p *contentSafetyProbe) match(status int, body []byte) string {
	if status == http.StatusBadRequest || status == http.StatusForbidden {
		text := normalizeContentSafetyText(contentSafetyErrorText(body))
		phrases := []string{"generated image rejected by content moderation", "generated video rejected by content moderation"}
		if status == http.StatusForbidden {
			if hit := contentSafetyRefusal(text); hit != "" {
				return hit
			}
			phrases = []string{
				"content violates usage guidelines", "safety_check_type_", "content moderation", "content policy",
				"image is sensitive", "text is sensitive", "prohibited content", "forbidden content",
				"request blocked by policy", "request rejected by policy", "request violates policy",
				"prompt violates policy", "input violates policy",
			}
		}
		for _, phrase := range phrases {
			if strings.Contains(text, phrase) {
				return phrase
			}
		}
		return ""
	}
	if status < 200 || status >= 300 {
		return ""
	}
	// Buffered callers may receive SSE even when they requested JSON.
	if bodyHasSSEFraming(body) {
		var parser openAICompatSSEFrameParser
		for _, line := range strings.Split(string(body), "\n") {
			if frame, ok := parser.AddLine(line); ok {
				if hit := p.matchOutput([]byte(frame.Data)); hit != "" {
					return hit
				}
			}
		}
		if frame, ok := parser.Finish(); ok {
			return p.matchOutput([]byte(frame.Data))
		}
		return ""
	}
	return p.matchOutput(body)
}

func (p *contentSafetyProbe) matchOutput(body []byte) string {
	root := gjson.ParseBytes(body)
	var hit string
	observe := func(key, text string, delta bool) {
		if hit != "" || text == "" {
			return
		}
		text = normalizeContentSafetyText(text)
		if delta {
			if p.tails == nil {
				p.tails = make(map[string]string)
			}
			text = p.tails[key] + text
		}
		hit = contentSafetyRefusal(text)
		if delta && hit == "" {
			// Keep only the possible refusal suffix, even during long streams.
			for _, phrase := range contentSafetyRefusalPhrases {
				if i := strings.LastIndex(text, phrase); i >= 0 && strings.TrimSpace(text[i+len(phrase):]) == "" {
					text = phrase + " "
					break
				}
			}
			if len(text) > 128 {
				text = text[len(text)-128:]
			}
			if len(p.tails) < 64 || p.tails[key] != "" {
				p.tails[key] = text
			}
		}
	}
	if !gjson.ValidBytes(body) {
		observe("plain", string(body), false)
		return hit
	}
	if root.Type == gjson.String {
		observe("plain", root.String(), false)
		return hit
	}
	observe("error", contentSafetyErrorText(body), false)
	observe("output_text", root.Get("output_text").String(), false)
	observe("text", root.Get("text").String(), false)
	observe("refusal", root.Get("refusal").String(), false)
	observeContent := func(key string, content gjson.Result, delta bool) {
		if content.Type == gjson.String {
			observe(key, content.String(), delta)
			return
		}
		if content.IsArray() {
			content.ForEach(func(index, part gjson.Result) bool {
				if part.Get("type").String() == "text" || part.Get("type").String() == "output_text" {
					observe(fmt.Sprintf("%s:%d", key, index.Int()), part.Get("text").String(), delta)
				}
				return hit == ""
			})
		}
	}
	eventType := root.Get("type").String()
	if eventType == "response.output_text.delta" || eventType == "response.refusal.delta" {
		key := fmt.Sprintf("response:%d:%d:%s", root.Get("output_index").Int(), root.Get("content_index").Int(), eventType)
		observe(key, root.Get("delta").String(), true)
	}
	root.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		key := fmt.Sprintf("choice:%d", choice.Get("index").Int())
		for _, field := range []string{"content", "refusal"} {
			observeContent(key+field, choice.Get("delta."+field), true)
			observeContent(key+field, choice.Get("message."+field), false)
		}
		return hit == ""
	})
	observeParts := func(item gjson.Result) {
		item.Get("content").ForEach(func(_, part gjson.Result) bool {
			observe("output", part.Get("text").String(), false)
			observe("refusal", part.Get("refusal").String(), false)
			return hit == ""
		})
	}
	for _, value := range []gjson.Result{root, root.Get("response")} {
		value.Get("output").ForEach(func(_, item gjson.Result) bool { observeParts(item); return hit == "" })
	}
	if eventType == "response.output_item.done" {
		observeParts(root.Get("item"))
	}
	if eventType == "response.content_part.done" {
		observe("part", root.Get("part.text").String(), false)
		observe("refusal", root.Get("part.refusal").String(), false)
	}
	return hit
}

func contentSafetyErrorText(body []byte) string {
	if !gjson.ValidBytes(body) {
		return string(body)
	}
	root := gjson.ParseBytes(body)
	if root.Type == gjson.String {
		return root.String()
	}
	var values []string
	var collect func(gjson.Result)
	collect = func(value gjson.Result) {
		if value.Type == gjson.String {
			values = append(values, value.String())
			return
		}
		if value.IsObject() || value.IsArray() {
			value.ForEach(func(_, child gjson.Result) bool { collect(child); return true })
		}
	}
	// Do not bill a quoted refusal in unrelated metadata or echoed input.
	for _, field := range []string{"error", "message", "detail", "code", "type"} {
		collect(root.Get(field))
	}
	return strings.Join(values, "\n")
}
