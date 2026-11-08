package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

// Convert SDK file uploads to the provider's documented JSON edit format.
func mediaImageMultipart(body []byte, contentType string) ([]byte, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		return nil, fmt.Errorf("invalid multipart boundary")
	}
	r := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	out := map[string]any{}
	images := []any{}
	for {
		part, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid multipart image request")
		}
		name := strings.TrimSuffix(part.FormName(), "[]")
		data, err := io.ReadAll(io.LimitReader(part, (20<<20)+1))
		if err != nil || len(data) > 20<<20 {
			return nil, fmt.Errorf("each image or field must be at most 20 MB")
		}
		if part.FileName() != "" {
			ct := part.Header.Get("Content-Type")
			if ct == "" || ct == "application/octet-stream" {
				ct = http.DetectContentType(data)
			}
			if ct != "image/png" && ct != "image/jpeg" && ct != "image/webp" && ct != "image/gif" {
				return nil, fmt.Errorf("reference images must be PNG, JPEG, WebP or GIF")
			}
			ref := map[string]any{"image_url": "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data)}
			switch name {
			case "image":
				images = append(images, ref)
				if len(images) > 16 {
					return nil, fmt.Errorf("at most 16 reference images are supported")
				}
			case "mask":
				out["mask"] = ref
			default:
				return nil, fmt.Errorf("unsupported image file field")
			}
			continue
		}
		value := string(data)
		switch name {
		case "stream":
			v, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("stream must be a boolean")
			}
			out[name] = v
		case "n", "frame_count", "frame_duration_ms", "output_compression", "partial_images":
			v, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("%s must be an integer", name)
			}
			out[name] = v
		default:
			out[name] = value
		}
	}
	if len(images) > 0 {
		out["images"] = images
	}
	encoded, err := json.Marshal(out)
	if err != nil || len(encoded) > 128<<20 {
		return nil, fmt.Errorf("normalized image request is too large")
	}
	return encoded, nil
}
