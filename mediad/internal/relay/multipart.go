package relay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
)

func readImageMultipart(body []byte, contentType string) (map[string]any, []multipartFile, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		return nil, nil, fmt.Errorf("invalid multipart boundary")
	}
	r := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	out := map[string]any{}
	var files []multipartFile
	imageCount := 0
	for {
		part, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("invalid multipart image request")
		}
		name := strings.TrimSuffix(part.FormName(), "[]")
		data, err := io.ReadAll(io.LimitReader(part, (20<<20)+1))
		if err != nil || len(data) > 20<<20 {
			return nil, nil, fmt.Errorf("each image or field must be at most 20 MB")
		}
		if part.FileName() != "" {
			ct := part.Header.Get("Content-Type")
			if ct == "" || ct == "application/octet-stream" {
				ct = http.DetectContentType(data)
			}
			if ct != "image/png" && ct != "image/jpeg" && ct != "image/webp" && ct != "image/gif" {
				return nil, nil, fmt.Errorf("reference images must be PNG, JPEG, WebP or GIF")
			}
			switch name {
			case "image":
				imageCount++
				if imageCount > 16 {
					return nil, nil, fmt.Errorf("at most 16 reference images are supported")
				}
			case "mask":
			default:
				return nil, nil, fmt.Errorf("unsupported image file field")
			}
			files = append(files, multipartFile{Fieldname: part.FormName(), Filename: part.FileName(), CT: ct, Data: data})
			continue
		}
		value := string(data)
		switch name {
		case "stream":
			v, err := strconv.ParseBool(value)
			if err != nil {
				return nil, nil, fmt.Errorf("stream must be a boolean")
			}
			out[name] = v
		case "n", "frame_count", "frame_duration_ms", "output_compression", "partial_images":
			v, err := strconv.Atoi(value)
			if err != nil {
				return nil, nil, fmt.Errorf("%s must be an integer", name)
			}
			out[name] = v
		default:
			out[name] = value
		}
	}
	return out, files, nil
}

// Other provider schemas retain their JSON reference-image conversion.
func mediaImageMultipart(body []byte, contentType string) ([]byte, error) {
	out, files, err := readImageMultipart(body, contentType)
	if err != nil {
		return nil, err
	}
	images := []any{}
	for _, file := range files {
		ref := map[string]any{"image_url": "data:" + file.CT + ";base64," + base64.StdEncoding.EncodeToString(file.Data)}
		if strings.TrimSuffix(file.Fieldname, "[]") == "mask" {
			out["mask"] = ref
		} else {
			images = append(images, ref)
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

func passthroughImageEdit(in AssembleInput) bool {
	if in.Schema == nil || in.Schema.Request.BodyMode != "passthrough" || in.Schema.Request.Multipart != nil {
		return false
	}
	path := in.Schema.Upstream.Create.Path
	return strings.HasSuffix(in.ClientPath, "/images/edits") && (path == "/v1/images/generations" || path == "/v1/images/edits")
}

// Keep file uploads as multipart while applying the selected model and schema overrides.
func buildImageEditMultipart(in AssembleInput) ([]byte, string, error) {
	fields, files, err := readImageMultipart(in.Body, in.ContentType)
	if err != nil {
		return nil, "", err
	}
	if in.Model != "" {
		fields["model"] = in.Model
	}
	for name, value := range in.Schema.Request.StaticFields {
		fields[name] = value
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := w.WriteField(name, fmt.Sprint(value)); err != nil {
			return nil, "", err
		}
	}
	for _, file := range files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": file.Fieldname, "filename": file.Filename}))
		header.Set("Content-Type", file.CT)
		part, err := w.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(file.Data); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	if body.Len() > MaxRelayBody {
		return nil, "", fmt.Errorf("normalized image request is too large")
	}
	return body.Bytes(), w.FormDataContentType(), nil
}

func normalizeStandardRequest(in AssembleInput, protocol string) ([]byte, string, error) {
	body, ct := in.Body, in.ContentType
	if strings.HasPrefix(strings.ToLower(ct), "multipart/") {
		if protocol == "openai" && passthroughImageEdit(in) {
			return body, ct, nil
		}
		var err error
		body, err = mediaImageMultipart(body, ct)
		if err != nil {
			return nil, "", err
		}
		ct = "application/json"
	}
	if protocol != "gemini" {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
			return nil, "", fmt.Errorf("invalid image request")
		}
		fields["model"], _ = json.Marshal(in.Model)
		body, _ = json.Marshal(fields)
	}
	return body, ct, nil
}
