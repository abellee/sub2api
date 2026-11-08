package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strconv"
	"strings"
)

// Read routing and billing fields without encoding or copying reference images.
func mediaImageMultipartMetadata(body []byte, contentType string) ([]byte, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		return nil, fmt.Errorf("invalid multipart boundary")
	}
	r := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	out := map[string]any{}
	for {
		part, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid multipart image request")
		}
		name := strings.TrimSuffix(part.FormName(), "[]")
		if part.FileName() != "" {
			continue
		}
		switch name {
		case "model", "n", "duration", "size", "resolution", "stream", "response_format":
		default:
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, (20<<20)+1))
		if err != nil || len(data) > 20<<20 {
			return nil, fmt.Errorf("each image or field must be at most 20 MB")
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
	encoded, err := json.Marshal(out)
	if err != nil || len(encoded) > 128<<20 {
		return nil, fmt.Errorf("normalized image request is too large")
	}
	return encoded, nil
}
