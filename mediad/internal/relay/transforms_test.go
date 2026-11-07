package relay

import (
	"testing"
)

func TestTransformPixelToRatio(t *testing.T) {
	fn := builtinTransforms["pixel_to_ratio"]
	args := map[string]any{"ratios": []any{"1:1", "16:9", "9:16", "3:2"}}

	cases := []struct {
		in   string
		want string
	}{
		{"1024x1024", "1:1"},
		{"1280x720", "16:9"},
		{"720x1280", "9:16"},
		{"1536x1024", "3:2"},
		{"16:9", "16:9"}, // 已是比例,原样
	}
	for _, tc := range cases {
		got, err := fn(TransformContext{Raw: tc.in, Args: args})
		if err != nil {
			t.Fatalf("pixel_to_ratio(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("pixel_to_ratio(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := fn(TransformContext{Raw: 123, Args: args}); err == nil {
		t.Fatal("non-string input should error")
	}
	if _, err := fn(TransformContext{Raw: "100x100"}); err == nil {
		t.Fatal("missing args should error")
	}
}

func TestTransformEnumMap(t *testing.T) {
	fn := builtinTransforms["enum_map"]
	args := map[string]any{"map": map[string]any{"low": "1", "high": "5"}, "default": "3"}

	if got, _ := fn(TransformContext{Raw: "low", Args: args}); got != "1" {
		t.Fatalf("enum_map low = %v", got)
	}
	if got, _ := fn(TransformContext{Raw: "auto", Args: args}); got != "3" {
		t.Fatalf("enum_map fallback = %v", got)
	}
	noDefault := map[string]any{"map": map[string]any{"low": "1"}}
	if _, err := fn(TransformContext{Raw: "zzz", Args: noDefault}); err == nil {
		t.Fatal("unmapped without default should error")
	}
}

func TestTransformTemplateNumberDefaultBool(t *testing.T) {
	tpl := builtinTransforms["template"]
	got, err := tpl(TransformContext{Raw: "16:9", Args: map[string]any{"format": "--ar {value}"}})
	if err != nil || got != "--ar 16:9" {
		t.Fatalf("template = %v, %v", got, err)
	}

	num := builtinTransforms["number"]
	if got, _ := num(TransformContext{Raw: "0.5"}); got != 0.5 {
		t.Fatalf("number = %v", got)
	}

	def := builtinTransforms["default"]
	if got, _ := def(TransformContext{Raw: nil, Args: map[string]any{"default": "x"}}); got != "x" {
		t.Fatalf("default nil = %v", got)
	}
	if got, _ := def(TransformContext{Raw: "keep"}); got != "keep" {
		t.Fatalf("default passthrough = %v", got)
	}

	bl := builtinTransforms["bool_strict"]
	if got, _ := bl(TransformContext{Raw: "true"}); got != true {
		t.Fatalf("bool_strict = %v", got)
	}
	if _, err := bl(TransformContext{Raw: "yes"}); err == nil {
		t.Fatal("bool_strict 'yes' should error")
	}
}

func TestTransformIDsSorted(t *testing.T) {
	ids := TransformIDs()
	if len(ids) < 7 {
		t.Fatalf("expected >=7 builtin transforms, got %d", len(ids))
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] > ids[i] {
			t.Fatalf("TransformIDs not sorted: %v", ids)
		}
	}
}
