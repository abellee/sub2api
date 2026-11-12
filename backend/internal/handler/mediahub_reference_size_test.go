package handler

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestMediaImageReferenceSize(t *testing.T) {
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 640, 960))))
	imageURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
	for _, format := range []string{"json", "multipart"} {
		for _, scenario := range []string{"single", "multiple", "unreadable", "disabled", "omitted", "explicit"} {
			for _, async := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/async=%v", format, scenario, async), func(t *testing.T) {
					count := 1
					if scenario == "multiple" {
						count = 2
					}
					size := "auto"
					switch scenario {
					case "omitted":
						size = ""
					case "explicit":
						size = "1024x1024"
					}
					expected := size
					if scenario == "single" {
						expected = "640x960"
					}
					var body []byte
					contentType := "application/json"
					if format == "json" {
						url := imageURL
						if scenario == "unreadable" {
							url = "data:image/png;base64,aW52YWxpZA=="
						}
						body = []byte(fmt.Sprintf(`{"model":"gpt-image-2.6-sunburst","prompt":"fixture","images":[%q],"custom":"preserve"}`, url))
						if count == 2 {
							body = []byte(fmt.Sprintf(`{"model":"gpt-image-2.6-sunburst","prompt":"fixture","images":[%q,%q],"custom":"preserve"}`, url, url))
						}
						if size != "" {
							body = append(body[:len(body)-1], []byte(fmt.Sprintf(`,"size":%q}`, size))...)
						}
					} else {
						var buffer bytes.Buffer
						writer := multipart.NewWriter(&buffer)
						require.NoError(t, writer.WriteField("model", "gpt-image-2.6-sunburst"))
						require.NoError(t, writer.WriteField("prompt", "fixture"))
						require.NoError(t, writer.WriteField("custom", "preserve"))
						if size != "" {
							require.NoError(t, writer.WriteField("size", size))
						}
						for i := 0; i < count; i++ {
							part, err := writer.CreateFormFile("image[]", fmt.Sprintf("%d.png", i))
							require.NoError(t, err)
							data := encoded.Bytes()
							if scenario == "unreadable" {
								data = []byte("unreadable")
							}
							_, err = part.Write(data)
							require.NoError(t, err)
						}
						require.NoError(t, writer.Close())
						body, contentType = buffer.Bytes(), writer.FormDataContentType()
					}

					forwarded := 0
					media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						forwarded++
						raw, err := io.ReadAll(r.Body)
						require.NoError(t, err)
						if scenario != "single" {
							require.Equal(t, body, raw, "fallback must preserve every request byte")
						}
						if format == "json" {
							require.Equal(t, expected, gjson.GetBytes(raw, "size").String())
							require.Equal(t, "preserve", gjson.GetBytes(raw, "custom").String())
						} else {
							parsed := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
							parsed.Header.Set("Content-Type", r.Header.Get("Content-Type"))
							require.NoError(t, parsed.ParseMultipartForm(1<<20))
							defer func() { require.NoError(t, parsed.MultipartForm.RemoveAll()) }()
							require.Equal(t, expected, parsed.FormValue("size"))
							require.Equal(t, "preserve", parsed.FormValue("custom"))
							require.Len(t, parsed.MultipartForm.File["image[]"], count)
						}
						if async {
							w.WriteHeader(http.StatusAccepted)
							_, _ = io.WriteString(w, `{"task_id":"mtask_reference_size"}`)
						} else {
							_, _ = io.WriteString(w, `{"data":[{"url":"https://image.example/result.png"}]}`)
						}
					}))
					defer media.Close()
					bindings, err := mediahub.NewBindingStore(filepath.Join(t.TempDir(), "bindings.json"))
					require.NoError(t, err)
					require.NoError(t, bindings.Put(mediahub.Binding{GroupID: 3, SchemaID: "fixture", Enabled: true}))
					relay := &MediaHubRelayHandler{bindings: bindings, cfg: MediaHubConfig{MediadBaseURL: media.URL, RelaySecret: "fixture", ForwardTimeout: 2 * time.Second}, client: media.Client(), upstreamResolver: func(int64, string) (string, string) { return "https://upstream.example", "fixture" }}
					controller := &MediaHubController{bindings: bindings, relay: relay}
					router := gin.New()
					router.POST("/v1/images/edits/*suffix", func(c *gin.Context) {
						gid := int64(3)
						c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, GroupID: &gid, Group: &service.Group{Platform: service.PlatformOpenAI, ImageAutoSizeFromReference: scenario != "disabled"}})
						c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
						require.True(t, controller.TryStandardImage(c, async))
					})
					path := "/v1/images/edits/"
					if async {
						path += "async"
					}
					request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
					request.Header.Set("Content-Type", contentType)
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, request)
					expectedStatus := http.StatusOK
					if async {
						expectedStatus = http.StatusAccepted
					}
					require.Equal(t, expectedStatus, recorder.Code, recorder.Body.String())
					require.Equal(t, 1, forwarded)
					require.False(t, strings.Contains(recorder.Body.String(), "specify size"))
				})
			}
		}
	}
}
