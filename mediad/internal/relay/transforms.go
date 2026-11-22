package relay

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// TransformContext 提供转换时可能用到的旁路信息。
type TransformContext struct {
	// Raw 是该字段的原始客户端值。
	Raw any
	// Args 是 FieldRule.Args。
	Args map[string]any
}

// TransformFunc 把客户端字段值转换为上游字段值。
type TransformFunc func(ctx TransformContext) (any, error)

// builtinTransforms 是内置转换注册表。转换 id 在 schema 的
// field_map.*.transform 中引用;未知 id 在 schema 校验时即报错。
var builtinTransforms = map[string]TransformFunc{
	"passthrough":    transformPassthrough,
	"pixel_to_ratio": transformPixelToRatio,
	"enum_map":       transformEnumMap,
	"template":       transformTemplate,
	"number":         transformNumber,
	"string":         transformString,
	"default":        transformDefault,
	"bool_strict":    transformBoolStrict,
	"url_object":     transformURLObject,
}

// URL references may arrive as strings, image_url objects, or arrays of either.
func transformURLObject(ctx TransformContext) (any, error) {
	if values, ok := ctx.Raw.([]any); ok {
		out := make([]any, 0, len(values))
		for _, value := range values {
			item, err := transformURLObject(TransformContext{Raw: value})
			if err != nil {
				return nil, err
			}
			out = append(out, item)
		}
		return out, nil
	}
	value := ctx.Raw
	if fields, ok := value.(map[string]any); ok {
		value = fields["url"]
		if value == nil {
			value = fields["image_url"]
		}
		if nested, ok := value.(map[string]any); ok {
			value = nested["url"]
		}
	}
	url, ok := value.(string)
	if !ok || strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("url_object requires a URL")
	}
	return map[string]any{"url": url}, nil
}

// TransformIDs 返回全部内置转换 id(按字母序),供 dry-run/管理界面提示。
func TransformIDs() []string {
	ids := make([]string, 0, len(builtinTransforms))
	for id := range builtinTransforms {
		ids = append(ids, id)
	}
	// 小规模集合,排序保证输出稳定。
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

func transformPassthrough(ctx TransformContext) (any, error) {
	return ctx.Raw, nil
}

// transformPixelToRatio 把 "1024x1024" 之类像素尺寸折算到 args.ratios
// 列出的比例枚举(按宽高比就近);输入已是 "W:H" 时原样返回。
// args: {"ratios": ["1:1","16:9",...]}。
func transformPixelToRatio(ctx TransformContext) (any, error) {
	s, ok := ctx.Raw.(string)
	if !ok {
		return nil, fmt.Errorf("pixel_to_ratio expects string, got %T", ctx.Raw)
	}
	s = strings.TrimSpace(s)
	if strings.Contains(s, ":") {
		return s, nil // 已经是比例
	}
	parts := strings.SplitN(strings.ToLower(s), "x", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("pixel_to_ratio cannot parse %q", s)
	}
	w, errW := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	h, errH := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("pixel_to_ratio cannot parse %q", s)
	}
	ratios, err := stringSliceArg(ctx.Args, "ratios")
	if err != nil || len(ratios) == 0 {
		return nil, fmt.Errorf("pixel_to_ratio requires args.ratios")
	}
	target := w / h
	best, bestDiff := "", math.MaxFloat64
	for _, r := range ratios {
		rw, rh, ok := parseRatio(r)
		if !ok || rw <= 0 || rh <= 0 {
			continue
		}
		diff := math.Abs(rw/rh - target)
		if diff < bestDiff {
			best, bestDiff = r, diff
		}
	}
	if best == "" {
		return nil, fmt.Errorf("pixel_to_ratio no usable ratios in args")
	}
	return best, nil
}

func parseRatio(s string) (float64, float64, bool) {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	h, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return w, h, true
}

// transformEnumMap 按 args.map 做精确串映射;未命中时取 args.default,
// 无 default 则报错。args: {"map":{"low":"1"},"default":"3"}。
func transformEnumMap(ctx TransformContext) (any, error) {
	s, ok := ctx.Raw.(string)
	if !ok {
		return nil, fmt.Errorf("enum_map expects string, got %T", ctx.Raw)
	}
	if m, err := mapArg(ctx.Args, "map"); err == nil {
		if v, hit := m[strings.TrimSpace(s)]; hit {
			return v, nil
		}
	}
	if ctx.Args != nil {
		if d, has := ctx.Args["default"]; has {
			return d, nil
		}
	}
	return nil, fmt.Errorf("enum_map no mapping for %q and no default", s)
}

// transformTemplate 用 args.format 渲染,{value} 替换为原值的字符串形式。
// args: {"format": "--ar {value}"}。
func transformTemplate(ctx TransformContext) (any, error) {
	format, err := stringArg(ctx.Args, "format")
	if err != nil {
		return nil, err
	}
	return strings.ReplaceAll(format, "{value}", fmt.Sprintf("%v", ctx.Raw)), nil
}

// transformNumber 把数值字符串/浮点规整为 JSON number。
func transformNumber(ctx TransformContext) (any, error) {
	switch v := ctx.Raw.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return nil, fmt.Errorf("number cannot parse %q", v)
		}
		return f, nil
	default:
		return nil, fmt.Errorf("number expects number or numeric string, got %T", ctx.Raw)
	}
}

// transformString 强制转字符串。
func transformString(ctx TransformContext) (any, error) {
	return fmt.Sprintf("%v", ctx.Raw), nil
}

// transformDefault 值为空(null/空串)时取 args.default,否则原样。
func transformDefault(ctx TransformContext) (any, error) {
	if ctx.Raw == nil {
		if ctx.Args != nil {
			if d, has := ctx.Args["default"]; has {
				return d, nil
			}
		}
		return nil, nil
	}
	if s, ok := ctx.Raw.(string); ok && strings.TrimSpace(s) == "" {
		if ctx.Args != nil {
			if d, has := ctx.Args["default"]; has {
				return d, nil
			}
		}
	}
	return ctx.Raw, nil
}

// transformBoolStrict 只接受 JSON 布尔或 "true"/"false" 字符串。
func transformBoolStrict(ctx TransformContext) (any, error) {
	switch v := ctx.Raw.(type) {
	case bool:
		return v, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return nil, fmt.Errorf("bool_strict expects bool or true/false string, got %T", ctx.Raw)
}

func stringArg(args map[string]any, key string) (string, error) {
	if args == nil {
		return "", fmt.Errorf("missing args.%s", key)
	}
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing args.%s", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("args.%s must be string, got %T", key, v)
	}
	return s, nil
}

func stringSliceArg(args map[string]any, key string) ([]string, error) {
	if args == nil {
		return nil, fmt.Errorf("missing args.%s", key)
	}
	raw, ok := args[key].([]any)
	if !ok {
		return nil, fmt.Errorf("args.%s must be array", key)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("args.%s items must be strings", key)
		}
		out = append(out, s)
	}
	return out, nil
}

func mapArg(args map[string]any, key string) (map[string]any, error) {
	if args == nil {
		return nil, fmt.Errorf("missing args.%s", key)
	}
	m, ok := args[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("args.%s must be object", key)
	}
	return m, nil
}
