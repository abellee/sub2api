package relay

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ResolvePath 按点路径取值,支持数组下标:"data.result.data.image_urls"、"items.0.url"。
func ResolvePath(data any, path string) (any, bool) {
	cur := data
	for _, seg := range strings.Split(strings.TrimSpace(path), ".") {
		if seg == "" {
			continue
		}
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// ExtractTaskID 从异步创建响应中取上游任务号。
func ExtractTaskID(schema *Schema, respBody []byte) (string, error) {
	if schema.Response.TaskID == "" {
		return "", fmt.Errorf("schema has no response.task_id")
	}
	var data any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return "", fmt.Errorf("upstream create response is not JSON: %w", err)
	}
	v, ok := ResolvePath(data, schema.Response.TaskID)
	if !ok {
		return "", fmt.Errorf("task_id path %q not found in create response", schema.Response.TaskID)
	}
	switch s := v.(type) {
	case string:
		if strings.TrimSpace(s) == "" {
			return "", fmt.Errorf("task_id at %q is empty", schema.Response.TaskID)
		}
		return s, nil
	case float64:
		return strconv.FormatInt(int64(s), 10), nil
	default:
		return "", fmt.Errorf("task_id at %q has unsupported type %T", schema.Response.TaskID, v)
	}
}

// ExtractStatus 从查询响应中取状态字符串。
func ExtractStatus(schema *Schema, respBody []byte) (string, bool, error) {
	var data any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return "", false, fmt.Errorf("upstream query response is not JSON: %w", err)
	}
	v, ok := ResolvePath(data, schema.Response.Status)
	if !ok {
		return "", false, fmt.Errorf("status path %q not found in query response", schema.Response.Status)
	}
	s, ok := v.(string)
	if !ok {
		return "", false, fmt.Errorf("status at %q is not a string", schema.Response.Status)
	}
	return s, true, nil
}

// ClassifyStatus 把上游状态串归一为 sub2api 三态。
func ClassifyStatus(schema *Schema, status string) string {
	switch {
	case schema.IsTerminalSuccess(status):
		return "succeeded"
	case schema.IsTerminalFailure(status):
		return "failed"
	default:
		return "processing"
	}
}

// ExtractUsage 按schema 提取计费用量。
// Quantity 为数字时用固定量;为字符串时按点路径提取:
// 数值直接用,数组取长度(如 image_urls 的张数)。
func ExtractUsage(schema *Schema, respBody []byte) (float64, string, error) {
	spec := schema.Response.Usage
	unit := spec.Unit
	if unit == "" {
		unit = UnitImage
	}
	switch q := spec.Quantity.(type) {
	case float64:
		return q, unit, nil
	case int:
		return float64(q), unit, nil
	case string:
		var data any
		if err := json.Unmarshal(respBody, &data); err != nil {
			return 0, unit, fmt.Errorf("usage source response is not JSON: %w", err)
		}
		v, ok := ResolvePath(data, q)
		if !ok {
			return 0, unit, fmt.Errorf("usage path %q not found", q)
		}
		switch n := v.(type) {
		case float64:
			return n, unit, nil
		case []any:
			return float64(len(n)), unit, nil
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
			if err != nil {
				return 0, unit, fmt.Errorf("usage path %q value %q is not numeric", q, n)
			}
			return f, unit, nil
		default:
			return 0, unit, fmt.Errorf("usage path %q has unsupported type %T", q, v)
		}
	default:
		return 0, unit, fmt.Errorf("usage.quantity must be number or path string, got %T", spec.Quantity)
	}
}

// ExtractResult 按 schema.Response.Result 提取命名结果。
func ExtractResult(schema *Schema, respBody []byte) map[string]any {
	out := map[string]any{}
	if len(schema.Response.Result) == 0 {
		return out
	}
	var data any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return out
	}
	for name, path := range schema.Response.Result {
		if v, ok := ResolvePath(data, path); ok {
			out[name] = v
		}
	}
	return out
}

// ExtractErrorMessage 提取失败响应中的错误信息(可选能力)。
func ExtractErrorMessage(schema *Schema, respBody []byte) string {
	if schema.Response.ErrorMessage == "" {
		return ""
	}
	var data any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return ""
	}
	if v, ok := ResolvePath(data, schema.Response.ErrorMessage); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
