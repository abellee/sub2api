package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentSafetyMatchRefusalMatrix(t *testing.T) {
	for _, intro := range []string{"sorry, i ", "i'm sorry, i ", "i "} {
		for _, verb := range []string{"can't", "cannot", "c***t", "ca***"} {
			for _, suffix := range []string{"that", "this", "you", "your", "the request", "the prompt", "it", "thatsoever", "yourselves", "item"} {
				text := strings.ToUpper(intro + verb + " help with " + suffix + " request.")
				text = strings.ReplaceAll(text, "'", "’")
				for _, status := range []int{200, 201, 299, 403} {
					t.Run(fmt.Sprintf("%d/%s/%s/%s", status, intro, verb, suffix), func(t *testing.T) {
						body, err := json.Marshal(map[string]string{"message": text})
						require.NoError(t, err)
						probe := contentSafetyProbe{}
						require.NotEmpty(t, probe.match(status, body))
					})
				}
			}
		}
	}
}

func TestContentSafetyMatchStatusAndPolicyPhrases(t *testing.T) {
	phrases := []string{
		"content violates usage guidelines", "SAFETY_CHECK_TYPE_IMAGE", "content moderation", "content policy",
		"image is sensitive", "text is sensitive", "prohibited content", "forbidden content",
		"content policy violation", "content policy rejection", "content policy rejected",
		"content moderation rejection", "content moderation rejected", "content moderation blocked",
		"request blocked by content moderation", "request rejected by content moderation",
		"request blocked by policy", "request rejected by policy", "request violates policy",
		"prompt violates content policy", "prompt violates policy", "input violates content policy", "input violates policy",
	}
	for _, phrase := range phrases {
		for _, status := range []int{200, 400, 401, 403, 429, 500} {
			t.Run(fmt.Sprintf("%d/%s", status, phrase), func(t *testing.T) {
				body, _ := json.Marshal(map[string]any{"error": map[string]string{"message": strings.ToUpper(phrase)}})
				probe := contentSafetyProbe{}
				require.Equal(t, status == 403, probe.match(status, body) != "")
			})
		}
	}
	for _, phrase := range []string{"generated image rejected by content moderation", "generated video rejected by content moderation"} {
		probe := contentSafetyProbe{}
		require.NotEmpty(t, probe.match(400, []byte(strings.ToUpper(phrase))))
		require.Empty(t, probe.match(200, []byte(phrase)))
	}
	for _, tc := range []struct {
		status int
		body   string
	}{
		{200, `{"message":"I can't help with writing a story"}`},
		{403, `{"message":"I cannot help with a request"}`},
		{403, `{"message":"Forbidden"}`},
		{403, `{"message":"permission denied", "metadata":"content moderation"}`},
		{200, `{"choices":[{"message":{"content":"Hello"}}],"input":"I can't help with that"}`},
		{403, `{"message":"Failed check: SAFETY_CHECK_TYPE"}`},
		{200, `{"choices":[{"message":{"content":[{"type":"image_url","image_url":{"url":"https://example.com/I can't help with that"}}]}}]}`},
		{400, `{"message":"I can't help with that"}`},
		{500, `{"message":"I can't help with that"}`},
	} {
		probe := contentSafetyProbe{}
		require.Empty(t, probe.match(tc.status, []byte(tc.body)), tc.body)
	}
	probe := contentSafetyProbe{}
	require.NotEmpty(t, probe.match(403, []byte(`{"error":{"message":"failed check: safety_check_\u0074ype_image"}}`)))
}

func TestContentSafetyMatchOutputFormatsAndSplitStreams(t *testing.T) {
	for _, body := range []string{
		`{"choices":[{"message":{"content":"I'm sorry, I can't help with that request."}}]}`,
		`{"choices":[{"message":{"refusal":"I cannot help with this"}}]}`,
		`{"output":[{"type":"message","content":[{"type":"output_text","text":"I can’t help with your request"}]}]}`,
		`{"response":{"output":[{"content":[{"type":"refusal","refusal":"I can't help with it"}]}]}}`,
		`{"output_text":"I cannot help with the request"}`,
		`{"choices":[{"message":{"content":[{"type":"text","text":"I can't help with that"}]}}]}`,
		`{"type":"response.refusal.done","refusal":"I cannot help with this"}`,
		`{"type":"response.output_item.done","item":{"content":[{"type":"output_text","text":"I cannot help with this"}]}}`,
		`{"type":"response.content_part.done","part":{"type":"refusal","refusal":"I cannot help with this"}}`,
		`"I can't help with that"`,
		"I can't help with that",
	} {
		probe := contentSafetyProbe{}
		require.NotEmpty(t, probe.match(200, []byte(body)), body)
	}
	for _, protocol := range []string{"responses", "chat"} {
		probe := contentSafetyProbe{}
		for i, text := range []string{"I'm sorry, I ca", "n’t help wi", "th ", "thatsoever"} {
			var value any = map[string]any{"type": "response.output_text.delta", "delta": text}
			if protocol == "chat" {
				value = map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]string{"content": text}}}}
			}
			body, _ := json.Marshal(value)
			require.Equal(t, i == 3, probe.match(200, body) != "", protocol)
		}
	}
	probe := contentSafetyProbe{}
	require.Empty(t, probe.match(200, []byte(`{"choices":[{"index":0,"delta":{"content":"I can't help with "}}]}`)))
	require.Empty(t, probe.match(200, []byte(`{"choices":[{"index":1,"delta":{"content":"that"}}]}`)), "different choices must not combine")
	require.Empty(t, probe.match(200, []byte(`{"choices":[{"index":0,"delta":{"content":"writing a story"}}]}`)))
	sse := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"I can't help \"}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"with that\"}\n\n"
	probe = contentSafetyProbe{}
	require.NotEmpty(t, probe.match(200, []byte(sse)))
}
