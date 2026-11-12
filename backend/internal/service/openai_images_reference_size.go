package service

import (
	"bytes"
	"context"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/sjson"
)

// ResolveOpenAIImagesReferenceSize preserves media passthrough on any parsing or probe failure.
func ResolveOpenAIImagesReferenceSize(ctx context.Context, group *Group, body []byte, contentType string) ([]byte, string) {
	if group == nil || !group.ImageAutoSizeFromReference ||
		(group.Platform != PlatformOpenAI && group.Platform != PlatformComposite) {
		return body, contentType
	}
	parsed := &OpenAIImagesRequest{Endpoint: openAIImagesEditsEndpoint, Body: body, ContentType: contentType}
	mediaType, _, _ := mime.ParseMediaType(contentType)
	var err error
	if strings.EqualFold(mediaType, "multipart/form-data") {
		parsed.Multipart = true
		err = parseOpenAIImagesMultipartRequest(body, contentType, parsed)
	} else {
		err = parseOpenAIImagesJSONRequest(body, parsed)
	}
	if err != nil || ApplyOpenAIImagesReferenceSize(ctx, group, parsed) != nil {
		return body, contentType
	}
	return parsed.Body, parsed.ContentType
}

// ApplyOpenAIImagesReferenceSize requires an explicit auto size and one readable reference.
func ApplyOpenAIImagesReferenceSize(ctx context.Context, group *Group, parsed *OpenAIImagesRequest) error {
	if group == nil || !group.ImageAutoSizeFromReference ||
		(group.Platform != PlatformOpenAI && group.Platform != PlatformComposite) ||
		parsed == nil || !parsed.ExplicitSize || parsed.Size != "auto" ||
		len(parsed.InputImageURLs)+len(parsed.Uploads) != 1 {
		return nil
	}
	var size string
	if len(parsed.InputImageURLs) > 0 {
		size = openAIReferenceImageURLSize(ctx, parsed.InputImageURLs[0])
	} else {
		// Use the image bytes, not client-supplied width/height headers.
		size = detectOpenAIImageSize(bytes.NewReader(parsed.Uploads[0].Data))
	}
	if size == "" {
		return nil
	}

	body, contentType, err := rewriteOpenAIImagesSize(parsed.Body, parsed.ContentType, size)
	if err != nil {
		return err
	}
	parsed.Body, parsed.ContentType = body, contentType
	parsed.Size = size
	parsed.SizeTier = normalizeOpenAIImageSizeTier(size)
	return nil
}

func openAIReferenceImageURLSize(ctx context.Context, rawURL string) string {
	if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		return detectOpenAIImageResultSize(rawURL)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	// This shared client checks public destinations on every redirect and dial.
	resp, err := defaultImageDownloadHTTPClient().Do(request)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ""
	}
	return detectOpenAIImageSize(resp.Body)
}

func rewriteOpenAIImagesSize(body []byte, contentType, size string) ([]byte, string, error) {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		body, err = sjson.SetBytes(body, "size", size)
		return body, contentType, err
	}
	return rewriteOpenAIImagesMultipartField(body, contentType, "size", size)
}
