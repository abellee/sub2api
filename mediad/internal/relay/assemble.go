package relay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// AssembleInput 携带一次中转请求所需的全部输入。
// 除 Body 外均为 sub2api 通过受信请求头传入(见 README 契约)。
type AssembleInput struct {
	Schema      *Schema
	UpstreamURL string // 账号的上游 API 地址(含 scheme),来自 sub2api 账号配置
	Credential  string // 账号凭证,来自 sub2api 账号配置
	Method      string // 客户端方法(仅 POST/GET 有意义)
	ClientPath  string // 客户端请求路径(用于日志与 dry-run 展示)
	Body        []byte // 客户端原始 body(JSON 或 multipart)
	ContentType string // 客户端 Content-Type

	UserID  string
	KeyID   string
	GroupID string
	Model   string
}

// UpstreamRequest 是组装完成、即将发往上游的请求。
type UpstreamRequest struct {
	Method      string
	URL         string
	Header      http.Header
	Body        []byte
	ContentType string
}

// Assemble 按 schema 把客户端请求组装为上游请求。
// 上游请求头只包含 schema 定义的头(凭证 + HeaderParams),
// 不透传客户端任意头,避免把 sub2api 的内部头泄给上游。
func Assemble(in AssembleInput) (*UpstreamRequest, error) {
	if in.Schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if strings.TrimSpace(in.UpstreamURL) == "" {
		return nil, fmt.Errorf("upstream base url is required")
	}
	create := in.Schema.Upstream.Create
	method := strings.ToUpper(firstNonEmpty(in.Method, create.Method, http.MethodPost))

	payload, ct, err := buildPayload(in)
	if err != nil {
		return nil, err
	}

	header := make(http.Header)
	for name, tmpl := range in.Schema.Request.Credential {
		header.Set(name, renderTemplate(tmpl, in))
	}
	for name, tmpl := range in.Schema.Request.HeaderParams {
		header.Set(name, renderTemplate(tmpl, in))
	}
	if header.Get("Content-Type") == "" && payload != nil {
		header.Set("Content-Type", ct)
	}

	path := strings.ReplaceAll(create.Path, "{model}", url.PathEscape(in.Model))
	if in.Schema.ID == "klinkw-gpt-image" && strings.HasSuffix(in.ClientPath, "/images/edits") {
		path = "/v1/images/edits"
	}
	if strings.Contains(create.Path, "{model}") && (in.Model == "" || strings.ContainsAny(in.Model, "/\\?#")) {
		return nil, fmt.Errorf("valid model is required for upstream path")
	}
	fullURL := joinURL(in.UpstreamURL, path)
	return &UpstreamRequest{
		Method:      method,
		URL:         fullURL,
		Header:      header,
		Body:        payload,
		ContentType: header.Get("Content-Type"),
	}, nil
}

// buildPayload 依据 body 模式与 multipart 规则产出上游 body。
func buildPayload(in AssembleInput) ([]byte, string, error) {
	spec := in.Schema.Request

	var clientFields map[string]any
	isMultipart := strings.HasPrefix(strings.ToLower(in.ContentType), "multipart/")
	if isMultipart {
		fields, files, err := parseMultipart(in.Body, in.ContentType)
		if err != nil {
			return nil, "", fmt.Errorf("parse multipart: %w", err)
		}
		clientFields = fields
		if spec.Multipart != nil {
			dataURLs := make([]any, 0, len(files))
			for _, f := range files {
				if spec.Multipart.MaxFiles > 0 && len(dataURLs) >= spec.Multipart.MaxFiles {
					break
				}
				dataURLs = append(dataURLs, fileDataURL(f))
			}
			if len(dataURLs) > 0 {
				if clientFields == nil {
					clientFields = map[string]any{}
				}
				clientFields[spec.Multipart.FilesField] = dataURLs
			}
		}
	} else if len(in.Body) > 0 {
		if err := json.Unmarshal(in.Body, &clientFields); err != nil {
			return nil, "", fmt.Errorf("client body is not valid JSON: %w", err)
		}
	}

	var upstream map[string]any
	switch spec.BodyMode {
	case "passthrough":
		upstream = clientFields
	case "mapped":
		upstream = map[string]any{}
		for clientKey, rule := range spec.FieldMap {
			raw, ok := clientFields[clientKey]
			if !ok {
				continue
			}
			if rule.Drop {
				continue
			}
			value := raw
			if rule.Transform != "" && rule.Transform != "passthrough" {
				fn, found := builtinTransforms[rule.Transform]
				if !found {
					return nil, "", fmt.Errorf("unknown transform %q for field %q", rule.Transform, clientKey)
				}
				converted, err := fn(TransformContext{Raw: raw, Args: rule.Args})
				if err != nil {
					return nil, "", fmt.Errorf("transform %q for field %q: %w", rule.Transform, clientKey, err)
				}
				value = converted
			}
			target := firstNonEmpty(rule.To, clientKey)
			upstream[target] = value
		}
	default:
		return nil, "", fmt.Errorf("unsupported body_mode %q", spec.BodyMode)
	}

	if upstream == nil {
		upstream = map[string]any{}
	}
	for k, v := range spec.StaticFields {
		upstream[k] = v
	}
	// multipart 文件字段在 mapped 模式下若未被 field_map 引用,也要落到 to_body_field。
	if isMultipart && spec.Multipart != nil && spec.BodyMode == "mapped" {
		if raw, ok := clientFields[spec.Multipart.FilesField]; ok {
			if _, exists := upstream[spec.Multipart.ToBodyField]; !exists {
				upstream[spec.Multipart.ToBodyField] = raw
			}
		}
	}

	buf, err := json.Marshal(upstream)
	if err != nil {
		return nil, "", fmt.Errorf("encode upstream payload: %w", err)
	}
	return buf, "application/json", nil
}

type multipartFile struct {
	Fieldname string
	Filename  string
	CT        string
	Data      []byte
}

// parseMultipart 拆出表单字段(字符串值)与文件。
// 同名多值字段(如 image[])折叠为数组。
func parseMultipart(body []byte, contentType string) (map[string]any, []multipartFile, error) {
	reader := multipart.NewReader(bytes.NewReader(body), extractBoundary(contentType))
	fields := map[string]any{}
	var files []multipartFile
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		data, err := io.ReadAll(io.LimitReader(part, 64<<20))
		if err != nil {
			return nil, nil, err
		}
		if part.FileName() != "" {
			files = append(files, multipartFile{
				Fieldname: formKey(part.FormName()),
				Filename:  part.FileName(),
				CT:        part.Header.Get("Content-Type"),
				Data:      data,
			})
			continue
		}
		key := formKey(part.FormName())
		value := string(data)
		if existing, ok := fields[key]; ok {
			switch prev := existing.(type) {
			case []any:
				fields[key] = append(prev, value)
			default:
				fields[key] = []any{prev, value}
			}
		} else {
			fields[key] = value
		}
	}
	return fields, files, nil
}

// formKey 去掉 PHP 风格的 "[]" 后缀,统一字段名。
func formKey(name string) string {
	return strings.TrimSuffix(name, "[]")
}

func fileDataURL(f multipartFile) string {
	ct := f.CT
	if ct == "" {
		ct = "application/octet-stream"
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(f.Data)
}

// extractBoundary 从 Content-Type 头中取 boundary 参数。
func extractBoundary(contentType string) string {
	for _, part := range strings.Split(contentType, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "boundary=") {
			b := strings.TrimPrefix(part, part[:len("boundary=")])
			return strings.Trim(b, `"`)
		}
	}
	return ""
}

// renderTemplate 渲染凭证/请求头模板中的占位符。
func renderTemplate(tmpl string, in AssembleInput) string {
	r := strings.NewReplacer(
		"{credential}", in.Credential,
		"{user_id}", in.UserID,
		"{key_id}", in.KeyID,
		"{group_id}", in.GroupID,
		"{model}", in.Model,
	)
	return r.Replace(tmpl)
}

// joinURL 拼接 base 与路径模板,规范双斜杠。
func joinURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if !strings.Contains(path, "{task_id}") {
		return base + "/" + strings.TrimLeft(path, "/")
	}
	// create 路径不应含 {task_id};出现时保留原样交由调用方替换。
	return base + "/" + strings.TrimLeft(path, "/")
}

// BuildQueryURL 用内部任务记录替换 query 模板中的 {task_id}。
func BuildQueryURL(base string, template string, upstreamTaskID string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	p := strings.ReplaceAll(template, "{task_id}", url.PathEscape(upstreamTaskID))
	return base + "/" + strings.TrimLeft(p, "/")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
