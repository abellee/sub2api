package service

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageInputTokenEstimateProtocols(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"responses", "/v1/responses", `{"model":"gpt-6-sol","instructions":"Be helpful","input":[{"role":"user","content":"你好，世界"}]}`},
		{"chat", "/v1/chat/completions", `{"model":"gpt-6-sol","messages":[{"role":"system","content":"Be helpful"},{"role":"user","content":"你好，世界"}]}`},
		{"messages", "/v1/messages", `{"model":"kimi-k3","system":"Be helpful","messages":[{"role":"user","content":[{"type":"text","text":"你好，世界"}]}]}`},
		{"gemini", "/v1beta/models/gemini:generateContent", `{"systemInstruction":{"parts":[{"text":"Be helpful"}]},"contents":[{"role":"user","parts":[{"text":"你好，世界"}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			estimate := EstimateUsageInputTokens([]byte(tc.body), "gemini", tc.path)
			require.NotNil(t, estimate)
			require.Greater(t, estimate.Tokens, 4)
			require.Equal(t, "o200k_base", estimate.Encoding)
			require.False(t, estimate.Partial)
		})
	}
	simple := EstimateUsageInputTokens([]byte(`{"model":"gpt-6-sol","input":"hello world"}`), "", "/responses")
	require.Equal(t, 2, simple.Tokens)
	oldModel := EstimateUsageInputTokens([]byte(`{"model":"gpt-3.5-turbo","input":"hello world"}`), "", "/responses")
	require.Equal(t, "cl100k_base", oldModel.Encoding)
	require.Nil(t, EstimateUsageInputTokens([]byte(`{broken`), "", ""))
	require.Nil(t, EstimateUsageInputTokens([]byte(`{"model":"gpt-6-sol"}`), "", ""))
}

func TestUsageInputTokenEstimateIncludesHistoryToolsAndIgnoresOutputSettings(t *testing.T) {
	base := `{"model":"gpt-6-sol","messages":[{"role":"user","content":"hello"}]%s}`
	plain := EstimateUsageInputTokens([]byte(strings.Replace(base, "%s", "", 1)), "", "/chat/completions")
	withTool := EstimateUsageInputTokens([]byte(strings.Replace(base, "%s", `,"tools":[{"type":"function","function":{"name":"weather","description":"Look up weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}]`, 1)), "", "/chat/completions")
	require.NotNil(t, withTool)
	require.Greater(t, withTool.Tokens, plain.Tokens)
	outputSettings := EstimateUsageInputTokens([]byte(strings.Replace(base, "%s", `,"max_tokens":100000,"temperature":0.5`, 1)), "", "/chat/completions")
	require.Equal(t, plain.Tokens, outputSettings.Tokens)
	history := EstimateUsageInputTokens([]byte(`{"model":"gpt-6-sol","messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"Earlier reply"},{"role":"user","content":"Continue"}]}`), "", "/chat/completions")
	require.Greater(t, history.Tokens, plain.Tokens)
}

func TestUsageInputTokenEstimateMarksOpaqueInputAndDoesNotCountBase64(t *testing.T) {
	for _, size := range []int{16, 100000} {
		body := `{"model":"gpt-6-sol","input":[{"role":"user","content":[{"type":"input_text","text":"describe"},{"type":"input_image","image_url":"data:image/png;base64,` + strings.Repeat("A", size) + `"}]}]}`
		estimate := EstimateUsageInputTokens([]byte(body), "", "/responses")
		require.NotNil(t, estimate)
		require.True(t, estimate.Partial)
		require.Less(t, estimate.Tokens, 50)
	}
	for _, body := range []string{
		`{"input":"continue","previous_response_id":"resp_1"}`,
		`{"contents":[{"parts":[{"text":"continue"}]}],"cachedContent":"cachedContents/1"}`,
	} {
		estimate := EstimateUsageInputTokens([]byte(body), "gpt-6-sol", "/responses")
		require.NotNil(t, estimate)
		require.True(t, estimate.Partial)
	}
}

func TestUsageInputTokenEstimateConcurrent(t *testing.T) {
	body := []byte(`{"model":"gpt-6-sol","input":"Hello 世界\nfunc main() {}"}`)
	want := EstimateUsageInputTokens(body, "", "/responses")
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.Equal(t, want, EstimateUsageInputTokens(body, "", "/responses"))
		}()
	}
	wg.Wait()
}

func TestUsageInputTokenEstimateNeverChangesBilling(t *testing.T) {
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	input := &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{RequestID: "tokenizer-baseline", Model: "gpt-6-sol", Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}},
		APIKey: &APIKey{ID: 2, Group: &Group{ID: 7, Platform: PlatformOpenAI, RateMultiplier: 0.5}},
		User:   &User{ID: 2}, Account: &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	baseline := *repo.lastLog
	estimate := &InputTokenEstimate{Tokens: 999999, Encoding: "o200k_base"}
	input.Result.RequestID = "tokenizer-observed"
	require.NoError(t, svc.RecordUsage(WithUsageInputTokenEstimate(context.Background(), estimate), input))
	require.Equal(t, estimate, repo.lastLog.InputTokenEstimate)
	require.Equal(t, baseline.InputTokens, repo.lastLog.InputTokens)
	require.Equal(t, baseline.OutputTokens, repo.lastLog.OutputTokens)
	require.Equal(t, baseline.TotalCost, repo.lastLog.TotalCost)
	require.Equal(t, baseline.ActualCost, repo.lastLog.ActualCost)
}

func BenchmarkUsageInputTokenEstimate(b *testing.B) {
	body := []byte(`{"model":"gpt-6-sol","input":"` + strings.Repeat("Hello 世界，评估输入 token。", 1000) + `"}`)
	EstimateUsageInputTokens(body, "", "/responses")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		EstimateUsageInputTokens(body, "", "/responses")
	}
}
