package appcatalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchOfficialParsesMetaAndDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><head>
<title>Demo App</title>
<meta property="og:title" content="Demo App">
<meta property="og:description" content="A demo">
<meta property="og:image" content="/shot.png">
<link rel="apple-touch-icon" href="/icon.png">
</head><body>
<img class="screenshot" src="/ui.png" alt="screenshot">
<a href="/download">Download</a>
</body></html>`))
	}))
	t.Cleanup(srv.Close)

	fetcher := NewFetcher()
	fetcher.HTTP = srv.Client()
	result, err := fetcher.fetchOfficial(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Demo App" {
		t.Fatalf("name=%q", result.Name)
	}
	if result.Description != "A demo" {
		t.Fatalf("desc=%q", result.Description)
	}
	if result.IconURL != srv.URL+"/icon.png" {
		t.Fatalf("icon=%q", result.IconURL)
	}
	if result.DownloadPageURL != srv.URL+"/download" {
		t.Fatalf("download=%q", result.DownloadPageURL)
	}
	if len(result.Screenshots) == 0 {
		t.Fatal("expected screenshots")
	}
}

func TestParseHTMLMeta(t *testing.T) {
	meta := parseHTMLMeta(`<meta property="og:title" content="Hello"><meta name="description" content="World">`, "https://ex.com")
	if meta.OGTitle != "Hello" || meta.Description != "World" {
		t.Fatalf("%+v", meta)
	}
}

func TestMarkdownTitleAndIntroFromHTMLReadme(t *testing.T) {
	text := `<p align="center">
  <img src="assets/logo.png" alt="Grok App Logo" width="128" />
</p>
<h1 align="center">Grok App</h1>
<p align="center"><strong>专为 Grok Build CLI 打造的现代桌面工作台</strong></p>
<p align="center"><em>多项目空间 · 实时 Agent 流式会话</em></p>
<p align="center">
  <a href="./README.md">English</a> ·
  <a href="./README_ZH.md">中文</a>
</p>

---

> [!NOTE]
> **关于 Grok App：** 本项目是面向本地 Grok Build CLI（` + "`grok agent stdio`" + `）的开源桌面客户端与工作台，**非 xAI 官方产品**。
>
> 完整 Agent 能力需要本地安装并登录 Grok Build CLI。

---

## 目录

- [核心亮点](#核心亮点)

## 核心亮点

- 原生 Build 会话
- 多项目工作台

## 安装

- 安装步骤不应该出现在简介里
`
	if chineseReadmeScore("README_ZH.md") == 0 {
		t.Fatal("README_ZH.md should count as a Chinese readme")
	}
	title, intro := markdownTitleAndIntro(text)
	if title != "Grok App" {
		t.Fatalf("title=%q", title)
	}
	if !strings.Contains(intro, "专为 Grok Build CLI") {
		t.Fatalf("intro=%q", intro)
	}
	if !strings.Contains(intro, "多项目空间") {
		t.Fatalf("intro should include subtitle: %q", intro)
	}
	if !strings.Contains(intro, "核心亮点") || !strings.Contains(intro, "原生 Build 会话") {
		t.Fatalf("intro should include features: %q", intro)
	}
	if strings.Contains(intro, "开源桌面客户端") {
		t.Fatalf("intro should prefer the tagline over the NOTE: %q", intro)
	}
	if strings.Contains(intro, "安装步骤") {
		t.Fatalf("intro should not include install section: %q", intro)
	}
}

func TestMarkdownTitleAndIntroFallsBackToNote(t *testing.T) {
	text := `# Demo App

> [!NOTE]
> **关于本项目：** 这是一段足够长的中文说明，用作应用简介。
`
	title, intro := markdownTitleAndIntro(text)
	if title != "Demo App" {
		t.Fatalf("title=%q", title)
	}
	if !strings.Contains(intro, "这是一段足够长的中文说明") {
		t.Fatalf("intro=%q", intro)
	}
}

func TestMarkdownTitleAndIntroSimpleMarkdown(t *testing.T) {
	title, intro := markdownTitleAndIntro("# 演示应用\n\n这是一段中文介绍，用于填充应用简介。\n")
	if title != "演示应用" {
		t.Fatalf("title=%q", title)
	}
	if !strings.Contains(intro, "这是一段中文介绍") {
		t.Fatalf("intro=%q", intro)
	}
}

func TestPreferChinese(t *testing.T) {
	if got := preferChinese("Demo App", "演示应用"); got != "演示应用" {
		t.Fatalf("got %q", got)
	}
	if got := preferChinese("Demo App", "Another"); got != "Demo App" {
		t.Fatalf("fallback got %q", got)
	}
	merged := mergeFetch(
		FetchResult{Name: "Demo", Description: "English intro", Changelog: "Release notes"},
		FetchResult{Name: "演示应用", Description: "中文简介", Changelog: "更新说明"},
	)
	if merged.Name != "演示应用" || merged.Description != "中文简介" || merged.Changelog != "更新说明" {
		t.Fatalf("%+v", merged)
	}
}

func TestFetchOfficialFollowsChineseAlternate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/zh", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Language") != acceptLanguageZH {
			t.Errorf("zh Accept-Language=%q", r.Header.Get("Accept-Language"))
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html lang="zh-CN"><head>
<title>演示应用</title>
<meta property="og:title" content="演示应用">
<meta property="og:description" content="这是中文简介">
</head><body></body></html>`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Language") != acceptLanguageZH {
			t.Errorf("Accept-Language=%q", r.Header.Get("Accept-Language"))
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head>
<title>Demo App</title>
<meta property="og:title" content="Demo App">
<meta property="og:description" content="English intro">
<link rel="alternate" hreflang="zh-CN" href="/zh">
</head><body></body></html>`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	fetcher := NewFetcher()
	fetcher.HTTP = srv.Client()
	result, err := fetcher.fetchOfficial(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "演示应用" {
		t.Fatalf("name=%q", result.Name)
	}
	if result.Description != "这是中文简介" {
		t.Fatalf("desc=%q", result.Description)
	}
}

func TestFetchGitHubPrefersChineseReadme(t *testing.T) {
	files := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/zh"):
			_, _ = w.Write([]byte("# 演示应用\n\n这是一段中文介绍，用于填充应用简介。\n"))
		default:
			_, _ = w.Write([]byte("# Demo\n\nEnglish intro.\n"))
		}
	}))
	t.Cleanup(files.Close)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/demo/contents":
			writeJSONPayload(w, []map[string]any{
				{"name": "README.md", "type": "file", "download_url": files.URL + "/en"},
				{"name": "README.zh-CN.md", "type": "file", "download_url": files.URL + "/zh"},
			})
		case "/repos/acme/demo/releases/latest":
			writeJSONPayload(w, map[string]any{
				"tag_name": "v1.2.0",
				"body":     "Bug fixes",
				"html_url": "https://github.com/acme/demo/releases/tag/v1.2.0",
			})
		case "/repos/acme/demo/readme":
			writeJSONPayload(w, map[string]any{"download_url": files.URL + "/en"})
		case "/repos/acme/demo":
			writeJSONPayload(w, map[string]any{
				"name":        "demo",
				"description": "English repo",
				"owner":       map[string]any{"avatar_url": "https://example.com/icon.png"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	raw404 := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(raw404.Close)

	fetcher := NewFetcher()
	fetcher.HTTP = http.DefaultClient
	fetcher.GitHubAPI = api.URL
	fetcher.GitHubRaw = raw404.URL
	fetcher.GitHubWeb = raw404.URL
	result, err := fetcher.fetchGitHub(context.Background(), "https://github.com/acme/demo")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "演示应用" {
		t.Fatalf("name=%q", result.Name)
	}
	if !hasChinese(result.Description) {
		t.Fatalf("desc=%q", result.Description)
	}
}

func TestFetchGitHubFallsBackToRawChineseReadme(t *testing.T) {
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/HEAD/README_ZH.md") {
			_, _ = w.Write([]byte(`<h1 align="center">Grok App</h1>
<p align="center"><strong>专为 Grok Build CLI 打造的现代化桌面工作台</strong></p>
<p align="center"><em>多项目管理 · 实时智能体流式会话</em></p>
`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(raw.Close)

	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><head>
<title>RongleCat/grok-app</title>
<meta property="og:title" content="RongleCat/grok-app">
<meta property="og:description" content="English repo description">
</head><body></body></html>`))
	}))
	t.Cleanup(web.Close)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
	}))
	t.Cleanup(api.Close)

	fetcher := NewFetcher()
	fetcher.HTTP = http.DefaultClient
	fetcher.GitHubAPI = api.URL
	fetcher.GitHubRaw = raw.URL
	fetcher.GitHubWeb = web.URL
	result, err := fetcher.fetchGitHub(context.Background(), "https://github.com/RongleCat/grok-app")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Grok App" {
		t.Fatalf("name=%q", result.Name)
	}
	if !strings.Contains(result.Description, "专为 Grok Build CLI") {
		t.Fatalf("desc=%q", result.Description)
	}
	if strings.Contains(result.Description, "English repo") {
		t.Fatalf("should prefer Chinese README tagline: %q", result.Description)
	}
}

func TestExtractLogoURLPrefersReadmeLogo(t *testing.T) {
	text := `<p align="center"><img src="assets/logo.png" alt="Grok App Logo" width="128" /></p>
# Grok App
`
	got := extractLogoURL(text, "https://raw.githubusercontent.com/acme/demo/main/README.md")
	if got != "https://raw.githubusercontent.com/acme/demo/main/assets/logo.png" {
		t.Fatalf("logo=%q", got)
	}
	if isAvatarIcon("https://avatars.githubusercontent.com/u/1?v=4") != true {
		t.Fatal("expected github avatar to be rejected")
	}
	if isAvatarIcon("https://github.com/acme.png") != true {
		t.Fatal("expected github user png to be rejected")
	}
}

func TestFetchGitHubUsesReadmeLogoNotAvatar(t *testing.T) {
	files := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<p align="center"><img src="assets/logo.png" alt="Demo Logo" /></p>
# 演示应用

这是一段中文介绍，用于填充应用简介。

## 特性

- 多项目管理
- 实时会话
`))
	}))
	t.Cleanup(files.Close)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/demo/contents":
			writeJSONPayload(w, []map[string]any{
				{"name": "README_ZH.md", "type": "file", "download_url": files.URL + "/zh"},
			})
		case "/repos/acme/demo/releases/latest":
			writeJSONPayload(w, map[string]any{"tag_name": "v1.0.0", "body": "notes", "html_url": "https://github.com/acme/demo/releases/tag/v1.0.0"})
		case "/repos/acme/demo/readme":
			writeJSONPayload(w, map[string]any{"download_url": files.URL + "/zh"})
		case "/repos/acme/demo":
			writeJSONPayload(w, map[string]any{
				"name":        "demo",
				"description": "English repo",
				"homepage":    "https://demo.example",
				"owner":       map[string]any{"avatar_url": "https://avatars.githubusercontent.com/u/1?v=4"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	fetcher := NewFetcher()
	fetcher.HTTP = http.DefaultClient
	fetcher.GitHubAPI = api.URL
	result, err := fetcher.fetchGitHub(context.Background(), "https://github.com/acme/demo")
	if err != nil {
		t.Fatal(err)
	}
	if result.OfficialURL != "https://demo.example" {
		t.Fatalf("official=%q", result.OfficialURL)
	}
	if result.IconURL != files.URL+"/zh/assets/logo.png" && !strings.HasSuffix(result.IconURL, "/assets/logo.png") {
		t.Fatalf("icon should be logo, got %q", result.IconURL)
	}
	if strings.Contains(result.IconURL, "avatars.githubusercontent.com") {
		t.Fatalf("icon should not be avatar: %q", result.IconURL)
	}
	if !strings.Contains(result.Description, "特性") || !strings.Contains(result.Description, "多项目管理") {
		t.Fatalf("desc should include features: %q", result.Description)
	}
}

func TestFetchAppFillsGitHubFromOfficialPage(t *testing.T) {
	var githubCalled bool
	files := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/en") || strings.Contains(strings.ToLower(r.URL.Path), "readme") {
			_, _ = w.Write([]byte("# Demo\n\nA demo app.\n"))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(files.Close)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubCalled = true
		switch r.URL.Path {
		case "/repos/acme/demo/contents":
			writeJSONPayload(w, []map[string]any{})
		case "/repos/acme/demo/releases/latest":
			writeJSONPayload(w, map[string]any{"tag_name": "v1.0.0", "body": "notes", "html_url": "https://github.com/acme/demo/releases/tag/v1.0.0"})
		case "/repos/acme/demo/readme":
			writeJSONPayload(w, map[string]any{"download_url": files.URL + "/en"})
		case "/repos/acme/demo":
			writeJSONPayload(w, map[string]any{"name": "demo", "description": "from github"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	official := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><head>
<title>Demo App</title>
<meta name="description" content="Official intro">
<link rel="apple-touch-icon" href="/icon.png">
</head><body>
<a href="https://github.com/acme/demo">GitHub</a>
</body></html>`))
	}))
	t.Cleanup(official.Close)

	fetcher := NewFetcher()
	fetcher.HTTP = http.DefaultClient
	fetcher.GitHubAPI = api.URL
	fetcher.GitHubRaw = files.URL
	fetcher.GitHubWeb = files.URL
	result, err := fetcher.FetchApp(context.Background(), official.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.GitHubURL != "https://github.com/acme/demo" {
		t.Fatalf("github=%q", result.GitHubURL)
	}
	if !githubCalled {
		t.Fatal("expected GitHub fetch after discovering repo URL")
	}
	if result.OfficialURL != official.URL {
		t.Fatalf("official=%q", result.OfficialURL)
	}
}

func writeJSONPayload(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
