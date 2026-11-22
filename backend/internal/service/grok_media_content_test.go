//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type grokMediaContentUpstreamStub struct {
	request   *http.Request
	requests  []*http.Request
	response  *http.Response
	responses []*http.Response
}

func (s *grokMediaContentUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.request = req
	s.requests = append(s.requests, req)
	if len(s.responses) > 0 {
		resp := s.responses[0]
		s.responses = s.responses[1:]
		return resp, nil
	}
	return s.response, nil
}

func (s *grokMediaContentUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, accountConcurrency)
}

type memoryGrokVideoCDNStore struct {
	mu   sync.Mutex
	m    map[string]string
	fail bool
}

func (s *memoryGrokVideoCDNStore) SetGrokVideoCDN(_ context.Context, publicPath, upstreamURL string, _ time.Duration) error {
	if s.fail {
		return io.ErrClosedPipe
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]string{}
	}
	s.m[publicPath] = upstreamURL
	return nil
}

func (s *memoryGrokVideoCDNStore) GetGrokVideoCDN(_ context.Context, publicPath string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[publicPath], nil
}

func (s *memoryGrokVideoCDNStore) snapshot() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.m))
	for key, value := range s.m {
		out[key] = value
	}
	return out
}

func grokMediaContentTestAccount() *Account {
	return &Account{
		ID:       9,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "upstream-key",
			"base_url": "https://relay.example/v1",
		},
	}
}

func grokMediaContentTestContext(method, target string, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, nil)
	for name, value := range headers {
		c.Request.Header.Set(name, value)
	}
	return c, recorder
}

func grokMediaContentStatusResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestForwardGrokMediaContentUsesUpstreamCredentialAndStreamsRange(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{
		responses: []*http.Response{grokMediaContentStatusResponse(`{"status":"completed"}`), {
			StatusCode: http.StatusPartialContent,
			Header: http.Header{
				"Content-Type":   []string{"video/mp4"},
				"Content-Length": []string{"13"},
				"Content-Range":  []string{"bytes 0-12/100"},
				"Accept-Ranges":  []string{"bytes"},
				"Content-Disposition": []string{
					`attachment; filename="task-1.mp4"`,
				},
			},
			Body: io.NopCloser(strings.NewReader("video-payload")),
		}},
	}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task-1/content", map[string]string{
		"Range": "bytes=0-12",
	})

	result, err := svc.ForwardGrokMedia(
		context.Background(), c, grokMediaContentTestAccount(),
		GrokMediaEndpointVideoContent, "task-1", nil, "",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusPartialContent, recorder.Code)
	require.Equal(t, "video-payload", recorder.Body.String())
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "https://relay.example/v1/videos/task-1", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer upstream-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "https://relay.example/v1/videos/task-1/content", upstream.requests[1].URL.String())
	require.Equal(t, "Bearer upstream-key", upstream.requests[1].Header.Get("Authorization"))
	require.Equal(t, "bytes=0-12", upstream.requests[1].Header.Get("Range"))
	require.Equal(t, "*/*", upstream.requests[1].Header.Get("Accept"))
	require.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	require.Equal(t, "13", recorder.Header().Get("Content-Length"))
	require.Equal(t, "bytes 0-12/100", recorder.Header().Get("Content-Range"))
	require.Equal(t, "bytes", recorder.Header().Get("Accept-Ranges"))
	require.Equal(t, `attachment; filename="task-1.mp4"`, recorder.Header().Get("Content-Disposition"))
	require.True(t, IsResponseCommitted(c))
}

func TestForwardGrokMediaContentStreamsFullResponseWithSafeDefaults(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{
		responses: []*http.Response{grokMediaContentStatusResponse(`{"status":"completed"}`), {
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Set-Cookie": []string{"secret=upstream"}, "X-Upstream-Secret": []string{"hidden"}},
			Body:          io.NopCloser(strings.NewReader("full-video")),
			ContentLength: -1,
		}},
	}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task-1/content", nil)

	_, err := svc.ForwardGrokMedia(
		context.Background(), c, grokMediaContentTestAccount(),
		GrokMediaEndpointVideoContent, "task-1", nil, "",
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "full-video", recorder.Body.String())
	require.Len(t, upstream.requests, 2)
	require.Empty(t, upstream.requests[1].Header.Get("Range"))
	require.Equal(t, "application/octet-stream", recorder.Header().Get("Content-Type"))
	require.Empty(t, recorder.Header().Get("Content-Length"))
	require.Empty(t, recorder.Header().Get("Set-Cookie"))
	require.Empty(t, recorder.Header().Get("X-Upstream-Secret"))
	require.True(t, IsResponseCommitted(c))
}

func TestForwardGrokMediaContentPreservesRangeNotSatisfiable(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{
		responses: []*http.Response{grokMediaContentStatusResponse(`{"status":"completed"}`), {
			StatusCode: http.StatusRequestedRangeNotSatisfiable,
			Header: http.Header{
				"Content-Type":   []string{"text/plain"},
				"Content-Length": []string{"11"},
				"Content-Range":  []string{"bytes */100"},
				"Accept-Ranges":  []string{"bytes"},
			},
			Body: io.NopCloser(strings.NewReader("bad-range!!")),
		}},
	}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task-1/content", map[string]string{
		"Range": "bytes=500-600",
	})

	_, err := svc.ForwardGrokMedia(
		context.Background(), c, grokMediaContentTestAccount(),
		GrokMediaEndpointVideoContent, "task-1", nil, "",
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusRequestedRangeNotSatisfiable, recorder.Code)
	require.Equal(t, "bad-range!!", recorder.Body.String())
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "bytes=500-600", upstream.requests[1].Header.Get("Range"))
	require.Equal(t, "bytes */100", recorder.Header().Get("Content-Range"))
	require.Equal(t, "bytes", recorder.Header().Get("Accept-Ranges"))
	require.True(t, IsResponseCommitted(c))
}

func TestForwardGrokMediaContentFetchesValidatedSignedURLWithoutCredentials(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{
		responses: []*http.Response{
			grokMediaContentStatusResponse(`{"status":"done","video":{"url":"https://vidgen.x.ai/signed-token/xai-video-task-1.mp4"}}`),
			{
				StatusCode: http.StatusPartialContent,
				Header: http.Header{
					"Content-Type":   []string{"video/mp4"},
					"Content-Length": []string{"13"},
					"Content-Range":  []string{"bytes 0-12/100"},
				},
				Body: io.NopCloser(strings.NewReader("video-payload")),
			},
		},
	}
	account := grokMediaContentTestAccount()
	account.Credentials[credKeyHeaderOverrideEnabled] = true
	account.Credentials[credKeyHeaderOverrides] = map[string]any{"user-agent": "private-agent"}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task-1/content", map[string]string{
		"Range": "bytes=0-12",
	})

	_, err := svc.ForwardGrokMedia(
		context.Background(), c, account,
		GrokMediaEndpointVideoContent, "task-1", nil, "",
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusPartialContent, recorder.Code)
	require.Equal(t, "video-payload", recorder.Body.String())
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "https://relay.example/v1/videos/task-1", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer upstream-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "private-agent", upstream.requests[0].Header.Get("User-Agent"))
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.requests[0].Context()))
	require.Equal(t, "https://vidgen.x.ai/signed-token/xai-video-task-1.mp4", upstream.requests[1].URL.String())
	require.Empty(t, upstream.requests[1].Header.Get("Authorization"))
	require.Empty(t, upstream.requests[1].Header.Get("User-Agent"))
	require.Equal(t, "bytes=0-12", upstream.requests[1].Header.Get("Range"))
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.requests[1].Context()))
	require.True(t, HTTPUpstreamPublicHostsOnly(upstream.requests[1].Context()))
}

func TestForwardGrokMediaContentFollowsAuthenticatedSub2APIRelay(t *testing.T) {
	for _, statusURL := range []string{
		`/v1/videos/task-1/content`,
		`https://relay.example/v1/videos/task-1/content`,
	} {
		t.Run(statusURL, func(t *testing.T) {
			upstream := &grokMediaContentUpstreamStub{
				responses: []*http.Response{
					grokMediaContentStatusResponse(`{"status":"completed","video":{"url":"` + statusURL + `"}}`),
					{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"video/mp4"}},
						Body:       io.NopCloser(strings.NewReader("video-payload")),
					},
				},
			}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task-1/content", nil)

			_, err := svc.ForwardGrokMedia(
				context.Background(), c, grokMediaContentTestAccount(),
				GrokMediaEndpointVideoContent, "task-1", nil, "",
			)

			require.NoError(t, err)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "video-payload", recorder.Body.String())
			require.Len(t, upstream.requests, 2)
			require.Equal(t, "https://relay.example/v1/videos/task-1/content", upstream.requests[1].URL.String())
			require.Equal(t, "Bearer upstream-key", upstream.requests[1].Header.Get("Authorization"))
		})
	}
}

func TestForwardGrokMediaContentRejectsUntrustedSignedURL(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{
		responses: []*http.Response{
			grokMediaContentStatusResponse(`{"status":"done","video":{"url":"http://169.` + `254.169.254/latest/meta-data"}}`),
		},
	}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, _ := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task-1/content", nil)

	_, err := svc.ForwardGrokMedia(
		context.Background(), c, grokMediaContentTestAccount(),
		GrokMediaEndpointVideoContent, "task-1", nil, "",
	)

	require.ErrorContains(t, err, "unsupported video content URL")
	require.Len(t, upstream.requests, 1)
}

func TestGrokMediaSignedVideoContentURLRejectsDeceptiveOrigins(t *testing.T) {
	for _, rawURL := range []string{
		"https://vidgen.x.ai" + "@attacker.invalid/video.mp4",
		"https://vidgen.x.ai:444/video.mp4",
		"http://vidgen.x.ai/video.mp4",
		"https://127.0.0.1/video.mp4",
		"https://localhost/video.mp4",
		"https://metadata.google.internal/video.mp4",
	} {
		t.Run(rawURL, func(t *testing.T) {
			_, err := grokMediaSignedVideoContentURL([]byte(`{"video":{"url":"`+rawURL+`"}}`), "task-1")
			require.ErrorContains(t, err, "unsupported video content URL")
		})
	}
}

func TestGrokMediaSignedVideoContentURLAcceptsPublicCDN(t *testing.T) {
	got, err := grokMediaSignedVideoContentURL(
		[]byte(`{"status":"done","content":"https://cdn.example.com/clips/task-1.mp4?sig=1"}`),
		"task-1",
	)
	require.NoError(t, err)
	require.Equal(t, "https://cdn.example.com/clips/task-1.mp4?sig=1", got)
}

func TestGrokMediaSignedVideoContentURLRejectsDifferentRelayTask(t *testing.T) {
	_, err := grokMediaSignedVideoContentURL(
		[]byte(`{"video":{"url":"/v1/videos/task-2/content"}}`),
		"task-1",
	)

	require.ErrorContains(t, err, "unsupported video content URL")
}

func TestForwardGrokVideoStatusRewritesOnlyProtectedContentURL(t *testing.T) {
	statusBody := `{"id":"task-1","status":"completed","url":"https://relay.example/v1/videos/task-1/content","download_url":"/v1/videos/task-1/content","video_url":"https://vidgen.x.ai/task-1.mp4","counter":9007199254740993}`
	upstream := &grokMediaContentUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(statusBody)),
		},
	}
	store := &memoryGrokVideoCDNStore{}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, grokVideoCDN: store}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task-1", map[string]string{
		"X-Forwarded-Host":  "malicious.invalid",
		"X-Forwarded-Proto": "https",
	})

	_, err := svc.ForwardGrokMedia(
		context.Background(), c, grokMediaContentTestAccount(),
		GrokMediaEndpointVideoStatus, "task-1", nil, "",
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	proxyURL := "https://api.example/v1/videos/task-1/content"
	require.Equal(t, proxyURL, gjson.Get(recorder.Body.String(), "url").String())
	require.Equal(t, proxyURL, gjson.Get(recorder.Body.String(), "download_url").String())
	require.Equal(t, "https://d.tokenfree.dev/task-1.mp4", gjson.Get(recorder.Body.String(), "video_url").String())
	require.Equal(t, "https://vidgen.x.ai/task-1.mp4", store.snapshot()["/task-1.mp4"])
	require.Equal(t, "9007199254740993", gjson.Get(recorder.Body.String(), "counter").String())
	require.NotContains(t, recorder.Body.String(), "malicious.invalid")
	require.NotContains(t, recorder.Body.String(), "vidgen.x.ai")
}

func TestRewriteGrokMediaVideoContentURLsPreservesOtherIDsAndHandlesNestedEscapedID(t *testing.T) {
	body := []byte(`{"nested":[{"url":"https://relay.example/v1/videos/task%2Fone/content"},{"url":"https://relay.example/v1/videos/task-two/content"}]}`)

	rewritten := rewriteGrokMediaVideoContentURLs(body, "task/one", grokMediaVideoRewriteOptions{
		contentProxyURL: "/v1/videos/task%2Fone/content",
	})

	require.Equal(t, "/v1/videos/task%2Fone/content", gjson.GetBytes(rewritten, "nested.0.url").String())
	require.Equal(t, "https://relay.example/v1/videos/task-two/content", gjson.GetBytes(rewritten, "nested.1.url").String())
}

func TestRewriteGrokMediaVideoContentURLsRewritesSignedVideoURL(t *testing.T) {
	body := []byte(`{"status":"done","video":{"url":"https://vidgen.x.ai/signed-token/xai-video-request-1.mp4","duration":8}}`)
	store := &memoryGrokVideoCDNStore{}

	rewritten := rewriteGrokMediaVideoContentURLs(body, "request-1", grokMediaVideoRewriteOptions{
		origin: "https://api.example",
		store: func(publicPath, upstreamURL string) bool {
			return store.SetGrokVideoCDN(context.Background(), publicPath, upstreamURL, time.Hour) == nil
		},
	})

	require.Equal(t, "https://api.example/signed-token/xai-video-request-1.mp4", gjson.GetBytes(rewritten, "video.url").String())
	require.Equal(t, "https://vidgen.x.ai/signed-token/xai-video-request-1.mp4", store.snapshot()["/signed-token/xai-video-request-1.mp4"])
	require.Equal(t, "8", gjson.GetBytes(rewritten, "video.duration").String())
	require.Equal(t, "done", gjson.GetBytes(rewritten, "status").String())
}

func TestRewriteGrokMediaVideoContentURLsRewritesContentCDN(t *testing.T) {
	body := []byte(`{"status":"done","content":"https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01","content_note":"keep","nested":{"content":"https://cdn.91topgo.com/m/other"}}`)
	store := &memoryGrokVideoCDNStore{}

	rewritten := rewriteGrokMediaVideoContentURLs(body, "request-1", grokMediaVideoRewriteOptions{
		origin: "https://api.example",
		store: func(publicPath, upstreamURL string) bool {
			return store.SetGrokVideoCDN(context.Background(), publicPath, upstreamURL, time.Hour) == nil
		},
	})

	require.Equal(t, "https://api.example/m/97dc6ef8a7fdc92c12a6bd01", gjson.GetBytes(rewritten, "content").String())
	require.Equal(t, "https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01", store.snapshot()["/m/97dc6ef8a7fdc92c12a6bd01"])
	require.Equal(t, "keep", gjson.GetBytes(rewritten, "content_note").String())
	require.Equal(t, "https://cdn.91topgo.com/m/other", gjson.GetBytes(rewritten, "nested.content").String())
}

func TestRewriteGrokMediaVideoContentURLsKeepsCDNWhenStoreFails(t *testing.T) {
	body := []byte(`{"status":"done","content":"https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01"}`)

	rewritten := rewriteGrokMediaVideoContentURLs(body, "request-1", grokMediaVideoRewriteOptions{
		origin:          "https://api.example",
		contentProxyURL: "https://api.example/v1/videos/request-1/content",
		store: func(string, string) bool {
			return false
		},
	})

	require.Equal(t, "https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01", gjson.GetBytes(rewritten, "content").String())
}

func TestForwardGrokVideoStatusRewritesContentFieldToPublicProxy(t *testing.T) {
	statusBody := `{"request_id":"task-1","status":"done","content":"https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01","video":{"url":"https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01","duration":8},"prompt":"生成一个美女"}`
	upstream := &grokMediaContentUpstreamStub{
		response: grokMediaContentStatusResponse(statusBody),
	}
	store := &memoryGrokVideoCDNStore{}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, grokVideoCDN: store}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task-1", nil)

	_, err := svc.ForwardGrokMedia(
		context.Background(), c, grokMediaContentTestAccount(),
		GrokMediaEndpointVideoStatus, "task-1", nil, "",
	)

	require.NoError(t, err)
	publicURL := "https://d.tokenfree.dev/m/97dc6ef8a7fdc92c12a6bd01"
	require.Equal(t, publicURL, gjson.Get(recorder.Body.String(), "content").String())
	require.Equal(t, publicURL, gjson.Get(recorder.Body.String(), "video.url").String())
	require.Equal(t, "https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01", store.snapshot()["/m/97dc6ef8a7fdc92c12a6bd01"])
	require.Equal(t, "8", gjson.Get(recorder.Body.String(), "video.duration").String())
	require.Equal(t, "生成一个美女", gjson.Get(recorder.Body.String(), "prompt").String())
	require.NotContains(t, recorder.Body.String(), "cdn.91topgo.com")
}

func TestForwardGrokMediaContentFetchesContentFieldCDN(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{
		responses: []*http.Response{
			grokMediaContentStatusResponse(`{"status":"done","content":"https://cdn.example.com/clips/task-1.mp4?sig=1"}`),
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"video/mp4"}, "Content-Length": []string{"5"}},
				Body:       io.NopCloser(strings.NewReader("video")),
			},
		},
	}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task-1/content", nil)

	_, err := svc.ForwardGrokMedia(
		context.Background(), c, grokMediaContentTestAccount(),
		GrokMediaEndpointVideoContent, "task-1", nil, "",
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "video", recorder.Body.String())
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "https://cdn.example.com/clips/task-1.mp4?sig=1", upstream.requests[1].URL.String())
	require.Empty(t, upstream.requests[1].Header.Get("Authorization"))
	require.True(t, HTTPUpstreamPublicHostsOnly(upstream.requests[1].Context()))
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.requests[1].Context()))
}

func TestForwardGrokVideoGenerationRewritesContentCDN(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{
		response: grokMediaContentStatusResponse(`{"request_id":"task-9","status":"done","content":"https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01?sig=1"}`),
	}
	store := &memoryGrokVideoCDNStore{}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, grokVideoCDN: store}
	c, recorder := grokMediaContentTestContext(http.MethodPost, "https://api.example/v1/videos/generations", nil)

	_, err := svc.ForwardGrokMedia(
		context.Background(), c, grokMediaContentTestAccount(),
		GrokMediaEndpointVideosGenerations, "",
		[]byte(`{"model":"grok-imagine-video","prompt":"waves"}`),
		"application/json",
	)

	require.NoError(t, err)
	require.Equal(t, "https://d.tokenfree.dev/m/97dc6ef8a7fdc92c12a6bd01?sig=1", gjson.Get(recorder.Body.String(), "content").String())
	require.Equal(t, "https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01?sig=1", store.snapshot()["/m/97dc6ef8a7fdc92c12a6bd01?sig=1"])
	require.NotContains(t, recorder.Body.String(), "cdn.91topgo.com")
}

func TestTryProxyStoredGrokVideoStreamsRecordedCDN(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{
		response: &http.Response{
			StatusCode:    http.StatusPartialContent,
			ContentLength: 5,
			Header: http.Header{
				"Content-Type":   []string{"video/mp4"},
				"Content-Length": []string{"5"},
				"Content-Range":  []string{"bytes 0-4/9"},
				"Accept-Ranges":  []string{"bytes"},
			},
			Body: io.NopCloser(strings.NewReader("video")),
		},
	}
	store := &memoryGrokVideoCDNStore{}
	require.NoError(t, store.SetGrokVideoCDN(context.Background(), "/m/97dc6ef8a7fdc92c12a6bd01", "https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01", time.Hour))
	svc := &OpenAIGatewayService{httpUpstream: upstream, grokVideoCDN: store}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/m/97dc6ef8a7fdc92c12a6bd01", map[string]string{
		"Range": "bytes=0-4",
	})

	require.True(t, svc.TryProxyStoredGrokVideo(c))
	require.Equal(t, http.StatusPartialContent, recorder.Code)
	require.Equal(t, "video", recorder.Body.String())
	require.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	require.Equal(t, "bytes 0-4/9", recorder.Header().Get("Content-Range"))
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01", upstream.requests[0].URL.String())
	require.Empty(t, upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "bytes=0-4", upstream.requests[0].Header.Get("Range"))
	require.True(t, HTTPUpstreamPublicHostsOnly(upstream.requests[0].Context()))
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.requests[0].Context()))
}

func TestTryProxyStoredGrokVideoIgnoresUnknownAndUnsafeTargets(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{}`)}
	store := &memoryGrokVideoCDNStore{}
	require.NoError(t, store.SetGrokVideoCDN(context.Background(), "/m/bad", "https://127.0.0.1/m/bad", time.Hour))
	require.NoError(t, store.SetGrokVideoCDN(context.Background(), "/m/mismatch", "https://cdn.example.com/other.mp4", time.Hour))
	require.NoError(t, store.SetGrokVideoCDN(context.Background(), "/v1/videos/task-1/content", "https://cdn.example.com/v1/videos/task-1/content", time.Hour))
	svc := &OpenAIGatewayService{httpUpstream: upstream, grokVideoCDN: store}

	unknown, _ := grokMediaContentTestContext(http.MethodGet, "https://api.example/m/missing", nil)
	require.False(t, svc.TryProxyStoredGrokVideo(unknown))

	reserved, reservedRecorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task-1/content", nil)
	require.False(t, svc.TryProxyStoredGrokVideo(reserved))
	require.Equal(t, http.StatusOK, reservedRecorder.Code)

	bad, badRecorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/m/bad", nil)
	require.True(t, svc.TryProxyStoredGrokVideo(bad))
	require.Equal(t, http.StatusNotFound, badRecorder.Code)

	mismatch, mismatchRecorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/m/mismatch", nil)
	require.True(t, svc.TryProxyStoredGrokVideo(mismatch))
	require.Equal(t, http.StatusNotFound, mismatchRecorder.Code)
	require.Empty(t, upstream.requests)
}

func TestGrokMediaContentProxyURLUsesTrustedForwardedHost(t *testing.T) {
	c, _ := grokMediaContentTestContext(http.MethodGet, "https://internal.invalid/v1/videos/task-1", map[string]string{
		"X-Forwarded-Host":  "api.customer.test",
		"X-Forwarded-Proto": "https",
	})
	cfg := &config.Config{}
	cfg.Server.TrustedProxies = []string{"192.0.2.1/32"}
	cfg.Server.TrustedProxiesConfigured = true

	require.Equal(t, "https://api.customer.test/v1/videos/task-1/content", grokMediaContentProxyURL(c, cfg, "task-1"))

	cfg.Server.TrustedProxiesConfigured = false
	require.Equal(t, "https://internal.invalid/v1/videos/task-1/content", grokMediaContentProxyURL(c, cfg, "task-1"))
}

func TestForwardGrokVideoStatusSwapsCDNHostFromTrustedProxy(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{
		response: grokMediaContentStatusResponse(`{"status":"done","video":{"url":"https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01"}}`),
	}
	store := &memoryGrokVideoCDNStore{}
	cfg := &config.Config{}
	cfg.Server.TrustedProxies = []string{"192.0.2.1/32"}
	cfg.Server.TrustedProxiesConfigured = true
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream, grokVideoCDN: store}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://internal.invalid/v1/videos/task-1", map[string]string{
		"X-Forwarded-Host":  "api.customer.test",
		"X-Forwarded-Proto": "https",
	})

	_, err := svc.ForwardGrokMedia(
		context.Background(), c, grokMediaContentTestAccount(),
		GrokMediaEndpointVideoStatus, "task-1", nil, "",
	)

	require.NoError(t, err)
	require.Equal(t, "https://d.tokenfree.dev/m/97dc6ef8a7fdc92c12a6bd01", gjson.Get(recorder.Body.String(), "video.url").String())
	require.Equal(t, "https://cdn.91topgo.com/m/97dc6ef8a7fdc92c12a6bd01", store.snapshot()["/m/97dc6ef8a7fdc92c12a6bd01"])
}

func TestForwardGrokVideoStatusRewritesWhenRedisStoreFails(t *testing.T) {
	const fileID = "fcbef1fbe6801fe067a80a51"
	upstreamURL := "https://cdn.example.com/m/" + fileID
	upstream := &grokMediaContentUpstreamStub{
		responses: []*http.Response{
			grokMediaContentStatusResponse(`{"status":"done","content":"` + upstreamURL + `","url":"` + upstreamURL + `"}`),
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"video/mp4"}, "Content-Length": []string{"5"}},
				Body:       io.NopCloser(strings.NewReader("video")),
			},
		},
	}
	store := &memoryGrokVideoCDNStore{fail: true}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, grokVideoCDN: store}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/task_yNxVK3pcvA9oU1mNHSR2QEh21OMq70b3", nil)

	_, err := svc.ForwardGrokMedia(
		context.Background(), c, grokMediaContentTestAccount(),
		GrokMediaEndpointVideoStatus, "task_yNxVK3pcvA9oU1mNHSR2QEh21OMq70b3", nil, "",
	)

	require.NoError(t, err)
	publicURL := "https://d.tokenfree.dev/m/" + fileID
	require.Equal(t, publicURL, gjson.Get(recorder.Body.String(), "content").String())
	require.Equal(t, publicURL, gjson.Get(recorder.Body.String(), "url").String())
	require.Empty(t, store.snapshot())

	playback, playbackRecorder := grokMediaContentTestContext(http.MethodGet, publicURL, nil)
	require.True(t, svc.TryProxyStoredGrokVideo(playback))
	require.Equal(t, http.StatusOK, playbackRecorder.Code)
	require.Equal(t, "video", playbackRecorder.Body.String())
	require.Equal(t, upstreamURL, upstream.requests[len(upstream.requests)-1].URL.String())
}

func TestTryProxyStoredGrokVideoFallsBackToRelayCDN(t *testing.T) {
	const fileID = "fcbef1fbe6801fe067a80a51"
	upstream := &grokMediaContentUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"video/mp4"}, "Content-Length": []string{"5"}},
			Body:       io.NopCloser(strings.NewReader("video")),
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "https://api.example/m/"+fileID, nil)

	require.True(t, svc.TryProxyStoredGrokVideo(c))
	require.Equal(t, "video", recorder.Body.String())
	require.Equal(t, "https://cdn.91topgo.com/m/"+fileID, upstream.requests[0].URL.String())

	withQuery, _ := grokMediaContentTestContext(http.MethodGet, "https://api.example/m/"+fileID+"?sig=1", nil)
	require.False(t, svc.TryProxyStoredGrokVideo(withQuery))
	require.Len(t, upstream.requests, 1)
}

func TestRewriteGrokMediaVideoContentURLsRewritesDataArray(t *testing.T) {
	body := []byte(`{"code":0,"data":[{"url":"https://cdn.91topgo.com/m/fcbef1fbe6801fe067a80a51","content":"https://cdn.91topgo.com/m/fcbef1fbe6801fe067a80a51"}],"nested":{"content":"https://cdn.91topgo.com/m/other"}}`)
	store := &memoryGrokVideoCDNStore{}

	rewritten := rewriteGrokMediaVideoContentURLs(body, "task-1", grokMediaVideoRewriteOptions{
		origin: "https://api.example",
		store: func(publicPath, upstreamURL string) bool {
			return store.SetGrokVideoCDN(context.Background(), publicPath, upstreamURL, time.Hour) == nil
		},
	})

	publicURL := "https://api.example/m/fcbef1fbe6801fe067a80a51"
	require.Equal(t, publicURL, gjson.GetBytes(rewritten, "data.0.url").String())
	require.Equal(t, publicURL, gjson.GetBytes(rewritten, "data.0.content").String())
	require.Equal(t, "https://cdn.91topgo.com/m/other", gjson.GetBytes(rewritten, "nested.content").String())
	require.Equal(t, int64(0), gjson.GetBytes(rewritten, "code").Int())
	require.Equal(t, "https://cdn.91topgo.com/m/fcbef1fbe6801fe067a80a51", store.snapshot()["/m/fcbef1fbe6801fe067a80a51"])
}
