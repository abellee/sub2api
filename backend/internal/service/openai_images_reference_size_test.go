package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func referenceSizePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, height))))
	return buffer.Bytes()
}

func TestOpenAIImagesReferenceSizeConditions(t *testing.T) {
	enabled := &Group{Platform: PlatformOpenAI, ImageAutoSizeFromReference: true}
	for _, tc := range []struct {
		name     string
		group    *Group
		size     string
		explicit bool
		image    bool
	}{
		{"no group", nil, "auto", true, true},
		{"disabled", &Group{Platform: PlatformOpenAI}, "auto", true, true},
		{"other platform", &Group{Platform: PlatformGemini, ImageAutoSizeFromReference: true}, "auto", true, true},
		{"omitted size", enabled, "auto", false, true},
		{"empty size", enabled, "", false, true},
		{"explicit dimensions", enabled, "1024x1536", true, true},
		{"different value", enabled, "AUTO", true, true},
		{"no reference", enabled, "auto", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"size":"auto","custom":"preserve"}`)
			parsed := &OpenAIImagesRequest{Size: tc.size, ExplicitSize: tc.explicit, Body: body, ContentType: "application/json"}
			if tc.image {
				// Invalid bytes ensure an inactive rule never even tries to read the image.
				parsed.Uploads = []OpenAIImagesUpload{{Data: []byte("unreadable")}}
			}
			require.NoError(t, ApplyOpenAIImagesReferenceSize(context.Background(), tc.group, parsed))
			require.Equal(t, tc.size, parsed.Size)
			require.Equal(t, body, parsed.Body)
		})
	}
}

func TestOpenAIImagesReferenceSizeJSONFormats(t *testing.T) {
	first := "data:image/png;base64," + base64.StdEncoding.EncodeToString(referenceSizePNG(t, 640, 960))
	for _, input := range []string{
		fmt.Sprintf(`"image":%q`, first),
		fmt.Sprintf(`"images":[%q]`, first),
		fmt.Sprintf(`"images":[{"image_url":%q}]`, first),
	} {
		t.Run(input[:strings.IndexByte(input, ':')], func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":"gpt-image-2","prompt":"draw","size":"auto","custom":17,%s}`, input))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, openAIImagesEditsEndpoint, bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			parsed, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			require.NoError(t, ApplyOpenAIImagesReferenceSize(context.Background(), &Group{Platform: PlatformComposite, ImageAutoSizeFromReference: true}, parsed))
			require.Equal(t, "640x960", parsed.Size)
			require.Equal(t, normalizeOpenAIImageSizeTier("640x960"), parsed.SizeTier)
			require.Equal(t, "640x960", gjson.GetBytes(parsed.Body, "size").String())
			require.Equal(t, int64(17), gjson.GetBytes(parsed.Body, "custom").Int())
			require.Equal(t, first, parsed.InputImageURLs[0])
		})
	}
}

func TestOpenAIImagesReferenceSizeMultipartPreservesFiles(t *testing.T) {
	first := referenceSizePNG(t, 640, 960)
	mask := []byte("mask bytes stay unchanged")
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for _, field := range [][2]string{{"model", "gpt-image-2"}, {"prompt", " keep spacing "}, {"size", "auto"}, {"custom", "keep me"}} {
		require.NoError(t, writer.WriteField(field[0], field[1]))
	}
	for i, data := range [][]byte{first, mask} {
		field := "image[]"
		if i == 1 {
			field = "mask"
		}
		part, err := writer.CreateFormFile(field, fmt.Sprintf("%d.png", i))
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	contentType := writer.FormDataContentType()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, openAIImagesEditsEndpoint, bytes.NewReader(buffer.Bytes()))
	c.Request.Header.Set("Content-Type", contentType)
	parsed, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, buffer.Bytes())
	require.NoError(t, err)
	parsed.Uploads[0].Width, parsed.Uploads[0].Height = 1, 1
	require.NoError(t, ApplyOpenAIImagesReferenceSize(context.Background(), &Group{Platform: PlatformOpenAI, ImageAutoSizeFromReference: true}, parsed))
	require.Equal(t, "640x960", parsed.Size)

	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(parsed.Body))
	request.Header.Set("Content-Type", parsed.ContentType)
	require.NoError(t, request.ParseMultipartForm(1<<20))
	defer request.MultipartForm.RemoveAll()
	require.Equal(t, "640x960", request.FormValue("size"))
	require.Equal(t, " keep spacing ", request.FormValue("prompt"))
	require.Equal(t, "keep me", request.FormValue("custom"))
	files := append(request.MultipartForm.File["image[]"], request.MultipartForm.File["mask"]...)
	require.Len(t, files, 2)
	for i, file := range files {
		opened, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(opened)
		require.NoError(t, err)
		require.NoError(t, opened.Close())
		require.Equal(t, [][]byte{first, mask}[i], data)
		require.Equal(t, fmt.Sprintf("%d.png", i), file.Filename)
	}
}

func TestOpenAIImagesReferenceSizeLeavesUnreadableOrMultipleImagesUnchanged(t *testing.T) {
	upload := OpenAIImagesUpload{Data: referenceSizePNG(t, 640, 960)}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(upload.Data)
	for _, tc := range []struct {
		name    string
		urls    []string
		uploads []OpenAIImagesUpload
	}{
		{"unreadable upload", nil, []OpenAIImagesUpload{{Data: []byte("invalid")}}},
		{"unreadable data URL", []string{"data:image/png;base64,aW52YWxpZA=="}, nil},
		{"unavailable URL", []string{"http://127.0.0.1/image"}, nil},
		{"multiple uploads", nil, []OpenAIImagesUpload{upload, upload}},
		{"multiple URLs", []string{dataURL, dataURL}, nil},
		{"mixed inputs", []string{dataURL}, []OpenAIImagesUpload{upload}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed := &OpenAIImagesRequest{ExplicitSize: true, Size: "auto", Body: []byte(`{"size":"auto"}`), ContentType: "application/json", InputImageURLs: tc.urls, Uploads: tc.uploads}
			before := *parsed
			require.NoError(t, ApplyOpenAIImagesReferenceSize(context.Background(), &Group{Platform: PlatformOpenAI, ImageAutoSizeFromReference: true}, parsed))
			require.Equal(t, before, *parsed)
		})
	}
}

func TestOpenAIImagesReferenceSizePublicURL(t *testing.T) {
	imageBytes := referenceSizePNG(t, 640, 960)
	transport := newImageDownloadHTTPClient(nil).Transport.(*imageDownloadTransport)
	transport.direct = referenceSizeRoundTripper(func(request *http.Request) (*http.Response, error) {
		require.Empty(t, request.Header.Get("Authorization"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(imageBytes)), Header: make(http.Header)}, nil
	})
	original := defaultImageDownloadHTTPClient
	defaultImageDownloadHTTPClient = func() *http.Client { return &http.Client{Transport: transport} }
	t.Cleanup(func() { defaultImageDownloadHTTPClient = original })
	require.Equal(t, "640x960", openAIReferenceImageURLSize(context.Background(), "https://reference.example/image"))
	for _, rawURL := range []string{"http://127.0.0.1/image", "http://169.254.169.254/image", "http://192.168.1.1/image"} {
		require.Empty(t, openAIReferenceImageURLSize(context.Background(), rawURL))
	}
}

type referenceSizeRoundTripper func(*http.Request) (*http.Response, error)

func (f referenceSizeRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestOpenAIReferenceImageSizeJPEGAndWebP(t *testing.T) {
	var buffer bytes.Buffer
	require.NoError(t, jpeg.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 929, 2043)), nil))
	require.Equal(t, "929x2043", detectOpenAIImageSize(&buffer))
	// Lossless WebP header for a 640x960 image.
	header := []byte{'R', 'I', 'F', 'F', 17, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8', 'L', 5, 0, 0, 0, 0x2f, 0x7f, 0xc2, 0xef, 0}
	require.Equal(t, "640x960", detectOpenAIImageSize(bytes.NewReader(header)))
}
