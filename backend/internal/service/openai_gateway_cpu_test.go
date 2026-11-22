package service

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAICPUEmptyImagePrefilterPreservesDetection(t *testing.T) {
	cases := []struct {
		name, body string
		want       bool
	}{
		{"nonempty", "{\"input\":[{\"type\":\"input_image\",\"image_url\":\"data:image/png;base64,AAAA\"}]}", false},
		{"empty", `{"input":[{"type":"input_image","image_url":"data:image/png;base64,"}]}`, true},
		{"spaces", `{"input":[{"content":[{"type":"input_image","image_url":"data:image/png;base64,   "}]}]}`, true},
		{"escaped whitespace", `{"input":[{"type":"input_image","image_url":"data:image/png;base64,\t\r\n\u00a0"}]}`, true},
		{"literal unicode whitespace", "{\"input\":[{\"type\":\"input_image\",\"image_url\":\"data:image/png;base64,\u00a0\u3000\"}]}", true},
		{"escaped comma", `{"input":[{"type":"input_image","image_url":"data:image/png;base64\u002c"}]}`, true},
		{"escaped prefix", `{"input":[{"type":"input_image","image_url":"d\u0061ta:image/png;ba\u0073e64,"}]}`, true},
		{"escaped keys and type", `{"in\u0070ut":[{"ty\u0070e":"input_\u0069mage","image_\u0075rl":"data:image/png;base64,"}]}`, true},
		{"escaped slash", `{"input":[{"type":"input_image","image_url":"data:image\/png;base64,"}]}`, true},
		{"text is not an image", `{"input":[{"type":"input_text","text":"input_image data:image/png;base64,"}]}`, false},
		{"remote image", `{"input":[{"type":"input_image","image_url":"https://example.com/image.png"}]}`, false},
		{"later empty image", `{"input":[{"type":"input_image","image_url":"data:image/png;base64,AAAA"},{"type":"input_image","image_url":"data:image/png;base64,"}]}`, true},
		{"nonempty escaped payload", `{"input":[{"type":"input_image","image_url":"data:image/png;base64,\u0041"}]}`, false},
		{"unrelated input", `{"input":[{"type":"input_text","text":"hello"}],"metadata":{"image_url":"data:image/png;base64,"}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(tc.body)
			require.True(t, json.Valid(body), tc.body)
			require.Equal(t, tc.want, openAIJSONValueMayContainEmptyBase64InputImage(parseRawJSONView(body).Get("input")))
			require.Equal(t, tc.want, openAIRequestBodyMayContainEmptyBase64InputImage(body))
			got, changed, err := sanitizeEmptyBase64InputImagesInOpenAIBody(body)
			require.NoError(t, err)
			require.Equal(t, tc.want, changed)
			require.True(t, json.Valid(got))
			require.Equal(t, tc.body, string(body), "caller bytes must remain unchanged")
		})
	}
}

func separateOpenAICPUToolSchemaPasses(body []byte, platform string) ([]byte, bool, error) {
	normalized := body
	changed := false
	if shouldRepairOpenAIResponsesNullToolSchemaType(platform) {
		next, repaired, err := sanitizeOpenAIResponsesToolParameterTypes(normalized)
		if err != nil {
			return nil, false, err
		}
		normalized = next
		changed = changed || repaired
	}
	if shouldSanitizeOpenAIResponsesToolSchemaPatterns(platform) {
		next, repaired, err := sanitizeOpenAIResponsesToolSchemaPatterns(normalized)
		if err != nil {
			return nil, false, err
		}
		normalized = next
		changed = changed || repaired
	}
	return normalized, changed, nil
}

func TestOpenAICPUToolSchemaCombinedMatchesSeparatePasses(t *testing.T) {
	cases := []string{
		`{"tools":[{"type":"function","parameters":{"type":null,"required":null,"pattern":"(?=drop)","properties":{"x":{"type":"string","pattern":"[a-z]+"}}}}]}`,
		`{"tools":[{"parameters":{"required":null,"patternProperties":{"(?=drop)":{"required":null},"ok":{"pattern":"(?!drop)"}},"additionalProperties":false,"type":null}}]}`,
		`{"input":[{"tools":[{"function":{"parameters":{"oneOf":[{"type":"object","required":null}],"pattern":"(?=drop)"}}}],"type":"additional_tools"}]}`,
		`{"tools":[{"type":"namespace","tools":[{"parameters":{"type":null,"default":{"pattern":"(?=keep)","required":null}}}]}]}`,
		`{"tools":[{"parameters":{"pattern":"(?=a)","required":null,"pattern":"(?=b)","type":null,"type":null}}]}`,
		`{"input":[{"content":"keep (?=foo) \"quoted\" \\","type":"message"}],"tools":[{"function":{"parameters":{"ty\u0070e":null,"patt\u0065rn":"(?=drop)"}}}]}`,
		`{"input":[{"type":"message","content":"hi"}],"tools":[]}`,
	}
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic, PlatformGrok, PlatformKimi, PlatformDeepseek, PlatformGemini, ""} {
		for _, raw := range cases {
			body := []byte(raw)
			require.True(t, json.Valid(body), raw)
			want, wantChanged, err := separateOpenAICPUToolSchemaPasses(body, platform)
			require.NoError(t, err)
			got, changed, err := sanitizeOpenAIResponsesToolSchemasForPlatform(body, platform)
			require.NoError(t, err)
			require.Equal(t, wantChanged, changed, raw)
			require.Equal(t, string(want), string(got), raw)
			require.Equal(t, raw, string(body))
			require.True(t, json.Valid(got))
		}
	}
}

func TestOpenAICPUToolSchemaFastStringsStillRejectMalformedJSON(t *testing.T) {
	for _, value := range []string{`"bad\q"`, `"bad\u123"`, `"bad\uXXXX"`, "\"bad\nnewline\"", `"unterminated`, "\"bad" + string([]byte{0xff}) + "\""} {
		body := []byte(`{"tools":[{"parameters":{"type":null,"pattern":"(?=drop)"}}],"input":` + value + "}")
		_, changed, err := sanitizeOpenAIResponsesToolSchemasForPlatform(body, PlatformOpenAI)
		require.Error(t, err, "%q", body)
		require.False(t, changed)
	}
	for slashes := 0; slashes < 12; slashes++ {
		text := strings.Repeat("\\", slashes) + "\" followed by quotes \" and unicode 汉字"
		encoded, err := json.Marshal(text)
		require.NoError(t, err)
		body := append([]byte(`{"tools":[{"parameters":{"type":null}}],"input":`), encoded...)
		body = append(body, '}')
		got, changed, err := sanitizeOpenAIResponsesToolSchemasForPlatform(body, PlatformOpenAI)
		require.NoError(t, err)
		require.True(t, changed)
		require.Equal(t, strings.Replace(string(body), `"type":null`, `"type":"object"`, 1), string(got))
	}
}

func FuzzOpenAICPUEmptyImagePrefilter(f *testing.F) {
	f.Add([]byte(`{"input":[{"type":"input_image","image_url":"data:image/png;base64,"}]}`))
	f.Add([]byte(`{"input":[{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]}`))
	f.Add([]byte(`{"input":[{"type":"input_image","image_url":"data:image/png;ba\u0073e64\u002c\t"}]}`))
	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > 1<<20 || !json.Valid(body) {
			return
		}
		want := openAIJSONValueMayContainEmptyBase64InputImage(parseRawJSONView(body).Get("input"))
		require.Equal(t, want, openAIRequestBodyMayContainEmptyBase64InputImage(body))
	})
}

func FuzzOpenAICPUToolSchemaCombined(f *testing.F) {
	f.Add([]byte(`{"tools":[{"parameters":{"type":null,"required":null,"pattern":"(?=x)"}}]}`))
	f.Add([]byte(`{"input":[{"tools":[{"parameters":{"patternProperties":{"(?=x)":{}},"additionalProperties":false}}],"type":"additional_tools"}]}`))
	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > 1<<20 || !json.Valid(body) {
			return
		}
		want, wantChanged, wantErr := separateOpenAICPUToolSchemaPasses(body, PlatformOpenAI)
		got, changed, err := sanitizeOpenAIResponsesToolSchemasForPlatform(body, PlatformOpenAI)
		require.Equal(t, wantErr == nil, err == nil)
		if err == nil {
			require.Equal(t, wantChanged, changed)
			require.Equal(t, string(want), string(got))
		}
	})
}

func BenchmarkOpenAICPUEmptyImageCheck(b *testing.B) {
	body := append([]byte(`{"input":[{"content":[{"type":"input_image","image_url":"data:image/png;base64,`), bytes.Repeat([]byte("A"), 8<<20)...)
	body = append(body, []byte(`"}]}]}`)...)
	b.Run("Previous", func(b *testing.B) {
		b.SetBytes(int64(len(body)))
		b.ReportAllocs()
		for b.Loop() {
			_ = openAIJSONValueMayContainEmptyBase64InputImage(parseRawJSONView(body).Get("input"))
		}
	})
	b.Run("Current", func(b *testing.B) {
		b.SetBytes(int64(len(body)))
		b.ReportAllocs()
		for b.Loop() {
			_ = openAIRequestBodyMayContainEmptyBase64InputImage(body)
		}
	})
}

func BenchmarkOpenAICPUToolSchemaPasses(b *testing.B) {
	body := append([]byte(`{"input":"`), bytes.Repeat([]byte("long context (with parentheses) "), 1<<18)...)
	body = append(body, []byte(`","tools":[{"parameters":{"type":null,"required":null,"pattern":"(?=drop)"}}]}`)...)
	b.Run("Separate", func(b *testing.B) {
		b.SetBytes(int64(len(body)))
		b.ReportAllocs()
		for b.Loop() {
			_, _, _ = separateOpenAICPUToolSchemaPasses(body, PlatformOpenAI)
		}
	})
	b.Run("Combined", func(b *testing.B) {
		b.SetBytes(int64(len(body)))
		b.ReportAllocs()
		for b.Loop() {
			_, _, _ = sanitizeOpenAIResponsesToolSchemasForPlatform(body, PlatformOpenAI)
		}
	})
}
