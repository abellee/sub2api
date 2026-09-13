package appcatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
)

const (
	maxBodyBytes     = 2 << 20
	acceptLanguageZH = "zh-CN,zh;q=0.9,en;q=0.8"
	githubAPIRoot    = "https://api.github.com"
)

type Fetcher struct {
	HTTP      *http.Client
	GitHubAPI string
	GitHubWeb string
	GitHubRaw string
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		HTTP: &http.Client{Timeout: 20 * time.Second},
	}
}

func (f *Fetcher) githubAPI() string {
	if f != nil && strings.TrimSpace(f.GitHubAPI) != "" {
		return strings.TrimRight(strings.TrimSpace(f.GitHubAPI), "/")
	}
	return githubAPIRoot
}

func (f *Fetcher) githubWeb() string {
	if f != nil && strings.TrimSpace(f.GitHubWeb) != "" {
		return strings.TrimRight(strings.TrimSpace(f.GitHubWeb), "/")
	}
	return "https://github.com"
}

func (f *Fetcher) githubRaw() string {
	if f != nil && strings.TrimSpace(f.GitHubRaw) != "" {
		return strings.TrimRight(strings.TrimSpace(f.GitHubRaw), "/")
	}
	return "https://raw.githubusercontent.com"
}

func (f *Fetcher) FetchApp(ctx context.Context, officialURL, githubURL string) (FetchResult, error) {
	officialURL = strings.TrimSpace(officialURL)
	githubURL = strings.TrimSpace(githubURL)
	var merged FetchResult
	var lastErr error
	fetchedOfficial, fetchedGitHub := false, false

	tryOfficial := func() {
		if officialURL == "" || fetchedOfficial {
			return
		}
		fetchedOfficial = true
		result, err := f.fetchOfficial(ctx, officialURL)
		if err != nil {
			lastErr = err
			return
		}
		merged = mergeFetch(merged, result)
		if githubURL == "" && result.GitHubURL != "" {
			githubURL = result.GitHubURL
		}
	}
	tryGitHub := func() {
		if githubURL == "" || fetchedGitHub {
			return
		}
		fetchedGitHub = true
		result, err := f.fetchGitHub(ctx, githubURL)
		if err != nil {
			lastErr = err
			return
		}
		merged = mergeFetch(merged, result)
		if officialURL == "" && result.OfficialURL != "" {
			officialURL = result.OfficialURL
		}
	}

	if officialURL != "" {
		tryOfficial()
		tryGitHub()
	} else {
		tryGitHub()
		tryOfficial()
	}

	if officialURL != "" {
		merged.OfficialURL = officialURL
	}
	if githubURL != "" {
		merged.GitHubURL = githubURL
	}
	if isAvatarIcon(merged.IconURL) {
		merged.IconURL = ""
	}
	if merged.Name == "" && merged.Description == "" && merged.IconURL == "" && lastErr != nil {
		return FetchResult{}, lastErr
	}
	return merged, nil
}

func mergeFetch(base, extra FetchResult) FetchResult {
	if extra.Name != "" && (base.Name == "" || looksLikeURL(base.Name) || chineseBetter(extra.Name, base.Name)) {
		base.Name = extra.Name
	}
	base.IconURL = preferIcon(base.IconURL, extra.IconURL)
	if betterDescription(extra.Description, base.Description) {
		base.Description = extra.Description
	}
	if extra.DownloadPageURL != "" && base.DownloadPageURL == "" {
		base.DownloadPageURL = extra.DownloadPageURL
	}
	base.Screenshots = uniqueNonEmpty(append(base.Screenshots, extra.Screenshots...))
	if extra.Changelog != "" && (base.Changelog == "" || chineseBetter(extra.Changelog, base.Changelog)) {
		base.Changelog = extra.Changelog
	}
	if extra.Version != "" {
		base.Version = extra.Version
	}
	if extra.OfficialURL != "" && base.OfficialURL == "" {
		base.OfficialURL = extra.OfficialURL
	}
	if extra.GitHubURL != "" && base.GitHubURL == "" {
		base.GitHubURL = extra.GitHubURL
	}
	return base
}

func looksLikeURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func (f *Fetcher) fetchOfficial(ctx context.Context, pageURL string) (FetchResult, error) {
	body, finalURL, err := f.getText(ctx, pageURL)
	if err != nil {
		return FetchResult{}, err
	}
	meta := parseHTMLMeta(body, finalURL)
	if meta.AlternateZH != "" && !sameURL(meta.AlternateZH, finalURL) && !pageLooksChinese(meta) {
		if zhBody, zhURL, zhErr := f.getText(ctx, meta.AlternateZH); zhErr == nil {
			meta = overlayChineseMeta(meta, parseHTMLMeta(zhBody, zhURL))
		}
	}
	result := FetchResult{
		Name:            preferChinese(meta.OGTitle, meta.Title),
		IconURL:         firstNonEmpty(meta.Logo, nonAvatarURL(meta.AppleIcon), nonAvatarURL(meta.Icon)),
		Description:     preferChinese(meta.OGDescription, meta.Description),
		DownloadPageURL: firstNonEmpty(meta.DownloadURL, pageURL),
		Screenshots:     uniqueNonEmpty(append([]string{meta.OGImage, meta.TwitterImage}, meta.Screenshots...)),
		GitHubURL:       meta.GitHubURL,
	}
	if logo := extractLogoURL(body, finalURL); result.IconURL == "" || (!isLogoURL(result.IconURL) && isLogoURL(logo)) {
		result.IconURL = preferIcon(result.IconURL, logo)
	}
	return result, nil
}

func (f *Fetcher) fetchGitHub(ctx context.Context, repoURL string) (FetchResult, error) {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return FetchResult{}, err
	}
	owner, repo, err := githubOwnerRepo(parsed)
	if err != nil {
		return FetchResult{}, err
	}
	apiRoot := f.githubAPI()
	result := FetchResult{
		DownloadPageURL: f.githubWeb() + "/" + owner + "/" + repo + "/releases",
		GitHubURL:       "https://github.com/" + owner + "/" + repo,
	}
	if apiRepo, apiErr := f.getJSON(ctx, apiRoot+"/repos/"+owner+"/"+repo); apiErr == nil {
		result.Name = stringValue(apiRepo["name"])
		result.Description = stringValue(apiRepo["description"])
		if home := officialHomepage(stringValue(apiRepo["homepage"]), owner, repo); home != "" {
			result.OfficialURL = home
		}
		if release, relErr := f.getJSON(ctx, apiRoot+"/repos/"+owner+"/"+repo+"/releases/latest"); relErr == nil {
			result.Version = strings.TrimPrefix(stringValue(release["tag_name"]), "v")
			result.Changelog = stringValue(release["body"])
			if htmlURL := stringValue(release["html_url"]); htmlURL != "" {
				result.DownloadPageURL = htmlURL
			}
		}
	} else {
		public := f.fetchGitHubPublicPage(ctx, owner, repo)
		result = mergeFetch(result, public)
	}
	applyReadme := func(text, base string) {
		title, intro := markdownTitleAndIntro(text)
		if title != "" && (result.Name == "" || looksLikeURL(result.Name) || !hasChinese(result.Name)) {
			result.Name = preferChinese(title, result.Name)
		}
		if betterDescription(intro, result.Description) {
			result.Description = intro
		}
		if logo := extractLogoURL(text, base); logo != "" {
			result.IconURL = preferIcon(result.IconURL, logo)
		}
		if result.OfficialURL == "" {
			if home := extractHomepageURL(text, owner, repo); home != "" {
				result.OfficialURL = home
			}
		}
	}
	zhText, zhBase, zhErr := f.fetchGitHubChineseReadme(ctx, owner, repo)
	if zhErr == nil && zhText != "" {
		applyReadme(zhText, zhBase)
		result.Screenshots = append(result.Screenshots, markdownImages(zhText, zhBase)...)
	}
	if result.Description == "" || !hasChinese(result.Description) || result.IconURL == "" {
		if text, base, readErr := f.fetchGitHubDefaultReadme(ctx, owner, repo); readErr == nil && text != "" {
			applyReadme(text, base)
			result.Screenshots = append(result.Screenshots, markdownImages(text, base)...)
		}
	} else if readme, err := f.getJSON(ctx, apiRoot+"/repos/"+owner+"/"+repo+"/readme"); err == nil {
		if download := stringValue(readme["download_url"]); download != "" {
			if text, _, readErr := f.getText(ctx, download); readErr == nil {
				result.Screenshots = append(result.Screenshots, markdownImages(text, download)...)
				if result.IconURL == "" {
					result.IconURL = preferIcon(result.IconURL, extractLogoURL(text, download))
				}
			}
		}
	}
	if result.IconURL == "" {
		result.IconURL = f.fetchRepoLogo(ctx, owner, repo)
	}
	result.Screenshots = uniqueNonEmpty(result.Screenshots)
	if result.Name == "" {
		result.Name = owner + "/" + repo
	}
	if isAvatarIcon(result.IconURL) {
		result.IconURL = ""
	}
	return result, nil
}

func (f *Fetcher) getText(ctx context.Context, rawURL string) (string, string, error) {
	body, finalURL, err := f.getBytes(ctx, rawURL, "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.8")
	if err != nil {
		return "", "", err
	}
	return string(body), finalURL, nil
}

func (f *Fetcher) getJSON(ctx context.Context, rawURL string) (map[string]any, error) {
	body, _, err := f.getBytes(ctx, rawURL, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (f *Fetcher) getBytes(ctx context.Context, rawURL, accept string) ([]byte, string, error) {
	body, finalURL, status, err := f.doGet(ctx, rawURL, accept)
	if err != nil {
		return nil, "", err
	}
	if status >= 400 {
		return nil, "", fmt.Errorf("GET %s: HTTP %d", rawURL, status)
	}
	return body, finalURL, nil
}

func (f *Fetcher) getOptionalText(ctx context.Context, rawURL string) (string, string, bool, error) {
	body, finalURL, status, err := f.doGet(ctx, rawURL, "text/plain,text/markdown,text/html;q=0.9,*/*;q=0.8")
	if err != nil {
		return "", "", false, err
	}
	if status == http.StatusNotFound {
		return "", "", false, nil
	}
	if status >= 400 {
		return "", "", false, fmt.Errorf("GET %s: HTTP %d", rawURL, status)
	}
	return string(body), finalURL, true, nil
}

func (f *Fetcher) doGet(ctx context.Context, rawURL, accept string) ([]byte, string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", 0, err
	}
	req.Header.Set("User-Agent", "sub2api-appcatalogd/1.0")
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", acceptLanguageZH)
	resp, err := f.HTTP.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", resp.StatusCode, err
	}
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return body, finalURL, resp.StatusCode, nil
}

func (f *Fetcher) fetchGitHubPublicPage(ctx context.Context, owner, repo string) FetchResult {
	result := FetchResult{
		Name:            repo,
		DownloadPageURL: f.githubWeb() + "/" + owner + "/" + repo + "/releases",
		GitHubURL:       "https://github.com/" + owner + "/" + repo,
	}
	if body, finalURL, err := f.getText(ctx, f.githubWeb()+"/"+owner+"/"+repo); err == nil {
		meta := parseHTMLMeta(body, finalURL)
		result.Name = preferChinese(meta.OGTitle, meta.Title, repo)
		if strings.Contains(result.Name, "/") {
			if _, after, ok := strings.Cut(result.Name, ": "); ok {
				result.Name = strings.TrimSpace(after)
			} else {
				result.Name = repo
			}
		}
		result.Description = preferChinese(meta.OGDescription, meta.Description)
	}
	if body, finalURL, err := f.getText(ctx, f.githubWeb()+"/"+owner+"/"+repo+"/releases/latest"); err == nil {
		if finalURL != "" {
			result.DownloadPageURL = finalURL
			if idx := strings.LastIndex(finalURL, "/"); idx >= 0 {
				result.Version = strings.TrimPrefix(finalURL[idx+1:], "v")
			}
		}
		if changelog := githubReleaseBody(body); changelog != "" {
			result.Changelog = changelog
		}
	}
	return result
}

func githubReleaseBody(page string) string {
	lower := strings.ToLower(page)
	start := strings.Index(lower, `class="markdown-body`)
	if start < 0 {
		start = strings.Index(lower, "class=\"markdown-body")
	}
	if start < 0 {
		return ""
	}
	open := strings.LastIndex(page[:start], "<")
	if open < 0 {
		return ""
	}
	rest := page[open:]
	end := strings.Index(strings.ToLower(rest), "</div>")
	if end < 0 {
		return ""
	}
	inner := rest[:end]
	if gt := strings.Index(inner, ">"); gt >= 0 {
		inner = inner[gt+1:]
	}
	text := strings.TrimSpace(stripHTMLTags(htmlBlocksToMarkdown(inner)))
	runes := []rune(text)
	if len(runes) > 4000 {
		text = string(runes[:4000])
	}
	return text
}

func (f *Fetcher) fetchGitHubChineseReadme(ctx context.Context, owner, repo string) (string, string, error) {
	if text, base, err := f.fetchGitHubChineseReadmeFromContents(ctx, owner, repo); err == nil && text != "" {
		return text, base, nil
	}
	return f.fetchGitHubChineseReadmeRaw(ctx, owner, repo)
}

func (f *Fetcher) fetchGitHubChineseReadmeFromContents(ctx context.Context, owner, repo string) (string, string, error) {
	body, _, err := f.getBytes(ctx, f.githubAPI()+"/repos/"+owner+"/"+repo+"/contents", "application/vnd.github+json")
	if err != nil {
		return "", "", err
	}
	var files []githubContent
	if err := json.Unmarshal(body, &files); err != nil {
		return "", "", err
	}
	var best *githubContent
	bestScore := 0
	for i := range files {
		file := &files[i]
		if !strings.EqualFold(file.Type, "file") {
			continue
		}
		score := chineseReadmeScore(file.Name)
		if score > bestScore {
			bestScore = score
			best = file
		}
	}
	if best == nil || best.DownloadURL == "" {
		return "", "", fmt.Errorf("no chinese readme")
	}
	text, finalURL, err := f.getText(ctx, best.DownloadURL)
	if err != nil {
		return "", "", err
	}
	return text, finalURL, nil
}

var chineseReadmeCandidates = []string{
	"README.zh-CN.md",
	"README.zh-cn.md",
	"README.zh_CN.md",
	"README.zh-Hans.md",
	"README.zh-TW.md",
	"README.zh-Hant.md",
	"README.zh.md",
	"README_ZH.md",
	"README.ZH.md",
	"README.cn.md",
	"README.CN.md",
	"README_CN.md",
}

func (f *Fetcher) fetchGitHubChineseReadmeRaw(ctx context.Context, owner, repo string) (string, string, error) {
	for _, name := range chineseReadmeCandidates {
		raw := f.githubRaw() + "/" + owner + "/" + repo + "/HEAD/" + name
		text, finalURL, ok, err := f.getOptionalText(ctx, raw)
		if err != nil || !ok || strings.TrimSpace(text) == "" {
			continue
		}
		return text, finalURL, nil
	}
	return "", "", fmt.Errorf("no chinese readme")
}

func (f *Fetcher) fetchGitHubDefaultReadme(ctx context.Context, owner, repo string) (string, string, error) {
	if readme, err := f.getJSON(ctx, f.githubAPI()+"/repos/"+owner+"/"+repo+"/readme"); err == nil {
		if download := stringValue(readme["download_url"]); download != "" {
			if text, finalURL, readErr := f.getText(ctx, download); readErr == nil {
				return text, finalURL, nil
			}
		}
	}
	for _, name := range []string{"README.md", "Readme.md", "readme.md"} {
		raw := f.githubRaw() + "/" + owner + "/" + repo + "/HEAD/" + name
		text, finalURL, ok, err := f.getOptionalText(ctx, raw)
		if err != nil || !ok || strings.TrimSpace(text) == "" {
			continue
		}
		return text, finalURL, nil
	}
	return "", "", fmt.Errorf("no readme")
}

var logoPathCandidates = []string{
	"assets/logo.png",
	"assets/logo.svg",
	"assets/logo.webp",
	"logo.png",
	"logo.svg",
	"logo.webp",
	"public/logo.png",
	"public/logo.svg",
	"images/logo.png",
	"static/logo.png",
}

func (f *Fetcher) fetchRepoLogo(ctx context.Context, owner, repo string) string {
	for _, path := range logoPathCandidates {
		raw := f.githubRaw() + "/" + owner + "/" + repo + "/HEAD/" + path
		_, finalURL, ok, err := f.getOptionalText(ctx, raw)
		if err != nil || !ok {
			continue
		}
		return firstNonEmpty(finalURL, raw)
	}
	return ""
}

type githubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

type htmlMeta struct {
	Title          string
	Description    string
	OGTitle        string
	OGDescription  string
	OGImage        string
	TwitterImage   string
	Icon           string
	AppleIcon      string
	DownloadURL    string
	Screenshots    []string
	Lang           string
	Locale         string
	AlternateZH    string
	Logo           string
	GitHubURL      string
}

func parseHTMLMeta(body, pageURL string) htmlMeta {
	var meta htmlMeta
	tokenizer := html.NewTokenizer(strings.NewReader(body))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return meta
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			switch token.Data {
			case "html":
				for _, attr := range token.Attr {
					if strings.EqualFold(attr.Key, "lang") {
						meta.Lang = strings.TrimSpace(attr.Val)
					}
				}
			case "title":
				if tokenizer.Next() == html.TextToken {
					assignPreferChinese(&meta.Title, tokenizer.Token().Data)
				}
			case "meta":
				applyMeta(&meta, token.Attr, pageURL)
			case "link":
				applyLink(&meta, token.Attr, pageURL)
			case "img":
				if shot := screenshotFromImg(token.Attr, pageURL); shot != "" {
					meta.Screenshots = append(meta.Screenshots, shot)
				}
				if logo := logoFromImg(token.Attr, pageURL); logo != "" {
					meta.Logo = preferIcon(meta.Logo, logo)
				}
			case "a":
				if download := downloadFromAnchor(token.Attr, pageURL); download != "" && meta.DownloadURL == "" {
					meta.DownloadURL = download
				}
				if repoURL := githubRepoFromHref(attrValue(token.Attr, "href")); repoURL != "" && meta.GitHubURL == "" {
					meta.GitHubURL = repoURL
				}
			}
		}
	}
}

func applyMeta(meta *htmlMeta, attrs []html.Attribute, pageURL string) {
	prop, name, content, lang := "", "", "", ""
	for _, attr := range attrs {
		switch strings.ToLower(attr.Key) {
		case "property":
			prop = strings.ToLower(attr.Val)
		case "name":
			name = strings.ToLower(attr.Val)
		case "content":
			content = strings.TrimSpace(attr.Val)
		case "lang":
			lang = strings.TrimSpace(attr.Val)
		}
	}
	if content == "" {
		return
	}
	chineseAttr := isChineseLang(lang)
	switch {
	case prop == "og:title":
		assignPreferChinese(&meta.OGTitle, content)
		if chineseAttr {
			meta.OGTitle = content
		}
	case prop == "og:description" || name == "description":
		if prop == "og:description" {
			assignPreferChinese(&meta.OGDescription, content)
			if chineseAttr {
				meta.OGDescription = content
			}
		} else {
			assignPreferChinese(&meta.Description, content)
			if chineseAttr {
				meta.Description = content
			}
		}
	case prop == "og:locale" || name == "og:locale":
		if meta.Locale == "" || isChineseLang(content) {
			meta.Locale = content
		}
	case prop == "og:image" || name == "og:image":
		if meta.OGImage == "" {
			meta.OGImage = resolveURL(pageURL, content)
		}
	case name == "twitter:image" || prop == "twitter:image":
		if meta.TwitterImage == "" {
			meta.TwitterImage = resolveURL(pageURL, content)
		}
	}
}

func applyLink(meta *htmlMeta, attrs []html.Attribute, pageURL string) {
	rel, href, hreflang := "", "", ""
	for _, attr := range attrs {
		switch strings.ToLower(attr.Key) {
		case "rel":
			rel = strings.ToLower(attr.Val)
		case "href":
			href = attr.Val
		case "hreflang":
			hreflang = strings.TrimSpace(attr.Val)
		}
	}
	if href == "" {
		return
	}
	resolved := resolveURL(pageURL, href)
	switch {
	case strings.Contains(rel, "alternate") && isChineseLang(hreflang):
		if meta.AlternateZH == "" || chineseLangScore(hreflang) > chineseLangScoreFromURL(meta.AlternateZH) {
			meta.AlternateZH = resolved
		}
	case strings.Contains(rel, "apple-touch-icon"):
		meta.AppleIcon = resolved
	case strings.Contains(rel, "icon"):
		if meta.Icon == "" {
			meta.Icon = resolved
		}
	}
}

func attrValue(attrs []html.Attribute, key string) string {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Key, key) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func screenshotFromImg(attrs []html.Attribute, pageURL string) string {
	src, alt, className := "", "", ""
	for _, attr := range attrs {
		switch strings.ToLower(attr.Key) {
		case "src":
			src = attr.Val
		case "alt":
			alt = strings.ToLower(attr.Val)
		case "class":
			className = strings.ToLower(attr.Val)
		}
	}
	if src == "" {
		return ""
	}
	haystack := alt + " " + className + " " + strings.ToLower(src)
	if strings.Contains(haystack, "screenshot") || strings.Contains(haystack, "screen-shot") {
		return resolveURL(pageURL, src)
	}
	return ""
}

func logoFromImg(attrs []html.Attribute, pageURL string) string {
	src, alt, className := "", "", ""
	for _, attr := range attrs {
		switch strings.ToLower(attr.Key) {
		case "src":
			src = attr.Val
		case "alt":
			alt = attr.Val
		case "class":
			className = attr.Val
		}
	}
	if src == "" || logoScore(src, alt, className) < 10 {
		return ""
	}
	return resolveURL(pageURL, src)
}

func downloadFromAnchor(attrs []html.Attribute, pageURL string) string {
	href := ""
	for _, attr := range attrs {
		if strings.ToLower(attr.Key) == "href" {
			href = attr.Val
		}
	}
	if href == "" {
		return ""
	}
	lower := strings.ToLower(href)
	if strings.Contains(lower, "/download") || strings.Contains(lower, "/releases") ||
		strings.HasSuffix(lower, ".dmg") || strings.HasSuffix(lower, ".exe") ||
		strings.HasSuffix(lower, ".apk") || strings.HasSuffix(lower, ".msi") ||
		strings.HasSuffix(lower, ".zip") {
		return resolveURL(pageURL, href)
	}
	return ""
}

var markdownImage = regexp.MustCompile(`!\[([^\]]*)]\(([^)]+)\)`)
var reHTMLImgTag = regexp.MustCompile(`(?is)<img\b[^>]*>`)
var reHTMLAttr = regexp.MustCompile(`(?i)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))`)
var reAbsURL = regexp.MustCompile(`https?://[^\s)<>"'\\]+`)

func markdownImages(markdown, baseURL string) []string {
	matches := markdownImage.FindAllStringSubmatch(markdown, 12)
	var out []string
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		src := strings.TrimSpace(match[2])
		src = strings.Split(src, " ")[0]
		lower := strings.ToLower(src)
		if strings.Contains(lower, "badge") || strings.Contains(lower, "shields.io") || logoScore(src, match[1], "") >= 10 {
			continue
		}
		out = append(out, resolveURL(baseURL, src))
	}
	return out
}

func extractLogoURL(text, baseURL string) string {
	best := ""
	bestScore := 0
	for _, raw := range reHTMLImgTag.FindAllString(text, 20) {
		attrs := parseHTMLTagAttrs(raw)
		src := firstNonEmpty(attrs["src"], attrs["data-src"])
		if src == "" {
			continue
		}
		score := logoScore(src, attrs["alt"], attrs["class"])
		if score > bestScore {
			bestScore = score
			best = resolveURL(baseURL, src)
		}
	}
	for _, match := range markdownImage.FindAllStringSubmatch(text, 20) {
		if len(match) < 3 {
			continue
		}
		src := strings.TrimSpace(strings.Split(match[2], " ")[0])
		score := logoScore(src, match[1], "")
		if score > bestScore {
			bestScore = score
			best = resolveURL(baseURL, src)
		}
	}
	if bestScore < 10 {
		return ""
	}
	return best
}

func parseHTMLTagAttrs(tag string) map[string]string {
	out := map[string]string{}
	for _, match := range reHTMLAttr.FindAllStringSubmatch(tag, -1) {
		key := strings.ToLower(match[1])
		val := firstNonEmpty(match[3], match[4], match[5])
		out[key] = unescapeBasic(val)
	}
	return out
}

func extractHomepageURL(text, owner, repo string) string {
	for _, raw := range reAbsURL.FindAllString(text, 40) {
		raw = strings.TrimRight(raw, ".,;)]}")
		if home := officialHomepage(raw, owner, repo); home != "" {
			return home
		}
	}
	return ""
}

func githubRepoFromHref(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Host)
	if host != "github.com" && host != "www.github.com" {
		return ""
	}
	owner, repo, err := githubOwnerRepo(parsed)
	if err != nil {
		return ""
	}
	if _, reserved := githubReservedOwners[strings.ToLower(owner)]; reserved {
		return ""
	}
	return "https://github.com/" + owner + "/" + repo
}

var githubReservedOwners = map[string]struct{}{
	"about": {}, "blog": {}, "collections": {}, "customer-stories": {},
	"enterprise": {}, "events": {}, "features": {}, "github": {},
	"login": {}, "marketplace": {}, "notifications": {}, "orgs": {},
	"pricing": {}, "readme": {}, "resources": {}, "security": {},
	"settings": {}, "solutions": {}, "sponsors": {}, "team": {},
	"topics": {}, "site": {},
}

func officialHomepage(raw, owner, repo string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	normalized, err := normalizeHTTPURL(raw)
	if err != nil {
		return ""
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Host)
	if host == "github.com" || host == "www.github.com" || host == "raw.githubusercontent.com" ||
		host == "user-images.githubusercontent.com" || host == "avatars.githubusercontent.com" {
		return ""
	}
	if isSocialOrBadgeHost(host) {
		return ""
	}
	_ = owner
	_ = repo
	return strings.TrimRight(normalized, "/")
}

func isSocialOrBadgeHost(host string) bool {
	host = strings.ToLower(host)
	names := []string{
		"shields.io", "twitter.com", "x.com", "discord.com", "discord.gg",
		"youtube.com", "youtu.be", "linkedin.com", "facebook.com",
		"instagram.com", "reddit.com", "npmjs.com",
	}
	for _, name := range names {
		if host == name || strings.HasSuffix(host, "."+name) {
			return true
		}
	}
	return false
}

func preferIcon(base, extra string) string {
	extra = strings.TrimSpace(extra)
	base = strings.TrimSpace(base)
	if extra == "" || isAvatarIcon(extra) {
		return nonAvatarURL(base)
	}
	if base == "" || isAvatarIcon(base) {
		return extra
	}
	if isLogoURL(extra) && !isLogoURL(base) {
		return extra
	}
	return base
}

func nonAvatarURL(raw string) string {
	if isAvatarIcon(raw) {
		return ""
	}
	return strings.TrimSpace(raw)
}

func isAvatarIcon(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "avatars.githubusercontent.com") ||
		strings.Contains(lower, "gravatar.com") ||
		strings.Contains(lower, "opengraph.githubassets.com") ||
		strings.Contains(lower, "/identicon") {
		return true
	}
	parsed, err := url.Parse(lower)
	if err != nil {
		return false
	}
	host := parsed.Host
	if host != "github.com" && host != "www.github.com" {
		return false
	}
	path := strings.Trim(parsed.Path, "/")
	return strings.HasSuffix(path, ".png") && !strings.Contains(path, "/")
}

func isLogoURL(raw string) bool {
	return logoScore(raw, "", "") >= 10
}

func logoScore(src, alt, className string) int {
	if strings.TrimSpace(src) == "" || isAvatarIcon(src) {
		return 0
	}
	hay := strings.ToLower(src + " " + alt + " " + className)
	if strings.Contains(hay, "shields.io") || strings.Contains(hay, "badge") ||
		strings.Contains(hay, "screenshot") || strings.Contains(hay, "screen-shot") ||
		strings.Contains(hay, "qrcode") || strings.Contains(hay, "qr-code") ||
		strings.Contains(hay, "wechat") || strings.Contains(hay, "微信") {
		return 0
	}
	score := 0
	if strings.Contains(hay, "logo") {
		score += 20
	}
	base := fileBase(src)
	if strings.HasPrefix(base, "logo.") || base == "logo" {
		score += 15
	}
	if strings.Contains(hay, "brand") {
		score += 8
	}
	return score
}

func fileBase(raw string) string {
	value := strings.ToLower(strings.Split(raw, "?")[0])
	value = strings.ReplaceAll(value, "\\", "/")
	if idx := strings.LastIndex(value, "/"); idx >= 0 {
		value = value[idx+1:]
	}
	return value
}

func betterDescription(candidate, current string) bool {
	candidate = strings.TrimSpace(candidate)
	current = strings.TrimSpace(current)
	if candidate == "" {
		return false
	}
	if current == "" {
		return true
	}
	if chineseBetter(candidate, current) {
		return true
	}
	if chineseBetter(current, candidate) {
		return false
	}
	return len([]rune(candidate)) > len([]rune(current))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func preferChinese(values ...string) string {
	fallback := ""
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if fallback == "" {
			fallback = value
		}
		if hasChinese(value) {
			return value
		}
	}
	return fallback
}

func assignPreferChinese(dst *string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if *dst == "" || chineseBetter(value, *dst) {
		*dst = value
	}
}

func chineseBetter(candidate, current string) bool {
	return hasChinese(candidate) && !hasChinese(current)
}

func hasChinese(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func isChineseLang(value string) bool {
	return chineseLangScore(value) > 0
}

func chineseLangScore(value string) int {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
	switch {
	case normalized == "zh-cn" || strings.HasPrefix(normalized, "zh-hans") || normalized == "zh-sg":
		return 3
	case normalized == "zh-tw" || normalized == "zh-hk" || normalized == "zh-mo" || strings.HasPrefix(normalized, "zh-hant"):
		return 2
	case normalized == "zh" || strings.HasPrefix(normalized, "zh-") || normalized == "cmn-hans" || normalized == "cmn-hant":
		return 1
	default:
		return 0
	}
}

func chineseLangScoreFromURL(raw string) int {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "zh-cn") || strings.Contains(lower, "zh_cn") || strings.Contains(lower, "/zh-hans"):
		return 3
	case strings.Contains(lower, "zh-tw") || strings.Contains(lower, "zh-hk"):
		return 2
	case strings.Contains(lower, "/zh"):
		return 1
	default:
		return 0
	}
}

func pageLooksChinese(meta htmlMeta) bool {
	if isChineseLang(meta.Lang) || isChineseLang(meta.Locale) {
		return true
	}
	return hasChinese(meta.OGTitle) || hasChinese(meta.Title) || hasChinese(meta.OGDescription) || hasChinese(meta.Description)
}

func overlayChineseMeta(base, extra htmlMeta) htmlMeta {
	base.Title = preferChinese(extra.Title, base.Title)
	base.OGTitle = preferChinese(extra.OGTitle, base.OGTitle)
	base.Description = preferChinese(extra.Description, base.Description)
	base.OGDescription = preferChinese(extra.OGDescription, base.OGDescription)
	if extra.AppleIcon != "" && base.AppleIcon == "" {
		base.AppleIcon = extra.AppleIcon
	}
	if extra.Icon != "" && base.Icon == "" {
		base.Icon = extra.Icon
	}
	if extra.OGImage != "" && base.OGImage == "" {
		base.OGImage = extra.OGImage
	}
	if extra.DownloadURL != "" && base.DownloadURL == "" {
		base.DownloadURL = extra.DownloadURL
	}
	base.Logo = preferIcon(base.Logo, extra.Logo)
	if extra.GitHubURL != "" && base.GitHubURL == "" {
		base.GitHubURL = extra.GitHubURL
	}
	base.Screenshots = uniqueNonEmpty(append(extra.Screenshots, base.Screenshots...))
	return base
}

func sameURL(a, b string) bool {
	left := strings.TrimRight(strings.TrimSpace(a), "/")
	right := strings.TrimRight(strings.TrimSpace(b), "/")
	return strings.EqualFold(left, right)
}

func chineseReadmeScore(name string) int {
	n := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	if !strings.HasPrefix(n, "readme") {
		return 0
	}
	switch {
	case strings.Contains(n, "zh-cn") || strings.Contains(n, "zh-hans"):
		return 3
	case strings.Contains(n, "zh-tw") || strings.Contains(n, "zh-hant") || strings.Contains(n, "zh-hk"):
		return 2
	case strings.Contains(n, "zh") || strings.Contains(n, ".cn.") || strings.HasSuffix(n, ".cn.md"):
		return 1
	default:
		return 0
	}
}

var (
	reHTMLHeading = regexp.MustCompile(`(?is)<h([1-6])\b[^>]*>(.*?)</h[1-6]>`)
	reHTMLBreak   = regexp.MustCompile(`(?i)<br\s*/?\s*>`)
	reHTMLLiOpen  = regexp.MustCompile(`(?i)<li\b[^>]*>`)
	reHTMLBlock   = regexp.MustCompile(`(?i)</(p|div|li|tr)>`)
	reHTMLImage   = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	reHTMLTag      = regexp.MustCompile(`(?is)<[^>]+>`)
	reMDLink       = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	reMDWrap       = regexp.MustCompile(`[*_~` + "`" + `]{1,2}`)
	reLangNavToken = regexp.MustCompile(`(?i)english|chinese|中文|简体中文|繁體中文|繁体中文|日本語|русский|deutsch|français|francais|español|espanol|português|portugues|한국어|italiano|readme(\.[a-z0-9._-]+)?`)
	reLangNavSep   = regexp.MustCompile(`[·•|/,\s\-—_]+`)
)

func htmlBlocksToMarkdown(text string) string {
	text = reHTMLHeading.ReplaceAllStringFunc(text, func(raw string) string {
		match := reHTMLHeading.FindStringSubmatch(raw)
		if len(match) < 3 {
			return "\n"
		}
		level := strings.Repeat("#", 1)
		if match[1] >= "1" && match[1] <= "6" {
			n := int(match[1][0] - '0')
			if n < 1 {
				n = 1
			}
			level = strings.Repeat("#", n)
		}
		inner := strings.TrimSpace(stripHTMLTags(match[2]))
		return "\n" + level + " " + inner + "\n"
	})
	text = reHTMLBreak.ReplaceAllString(text, "\n")
	text = reHTMLLiOpen.ReplaceAllString(text, "\n- ")
	text = reHTMLBlock.ReplaceAllString(text, "\n")
	text = reHTMLImage.ReplaceAllString(text, "")
	text = stripHTMLTags(text)
	return unescapeBasic(text)
}

func stripHTMLTags(text string) string {
	return reHTMLTag.ReplaceAllString(text, "")
}

func unescapeBasic(text string) string {
	return strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
	).Replace(text)
}

func stripMarkdownInline(line string) string {
	line = reMDLink.ReplaceAllString(line, "$1")
	line = reMDWrap.ReplaceAllString(line, "")
	return strings.Join(strings.Fields(line), " ")
}

func isBadgeOrImageLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, "![") || strings.HasPrefix(trimmed, "[![") {
		return true
	}
	if strings.Contains(strings.ToLower(trimmed), "shields.io") || strings.Contains(strings.ToLower(trimmed), "badge") {
		return true
	}
	return false
}

func chineseCount(value string) int {
	n := 0
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			n++
		}
	}
	return n
}

func quoteLine(raw string) (body string, quoted bool) {
	line := strings.TrimSpace(raw)
	if !strings.HasPrefix(line, ">") {
		return line, false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, ">")), true
}

func isAdmonitionMarker(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "[!") && strings.HasSuffix(line, "]")
}

func isHorizontalRule(line string) bool {
	switch strings.TrimSpace(line) {
	case "---", "***", "___", "- - -":
		return true
	default:
		return false
	}
}

func isBareURL(line string) bool {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false
	}
	return !strings.ContainsAny(s, " \t")
}

func isLanguageNav(line string) bool {
	stripped := reLangNavToken.ReplaceAllString(line, " ")
	stripped = reLangNavSep.ReplaceAllString(stripped, "")
	return stripped == ""
}

func skipIntroLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || isBadgeOrImageLine(line) || isBareURL(line) || isLanguageNav(line) {
		return true
	}
	return false
}

func joinIntroLines(parts []string) string {
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if hasChinese(strings.Join(out, " ")) && !hasChinese(part) {
			continue
		}
		out = append(out, part)
		joined := strings.Join(out, " ")
		if len(out) >= 3 || len([]rune(joined)) >= 240 {
			break
		}
	}
	return strings.Join(out, " ")
}

type readmeSection struct {
	title string
	lines []string
}

func markdownTitleAndIntro(text string) (string, string) {
	text = htmlBlocksToMarkdown(text)
	var title string
	var hero []string
	var fallback []string
	var sections []readmeSection
	var currentHeading string
	var current []string
	inAdmonition := false
	afterTitle := false

	flushHero := func() {
		if len(current) == 0 || currentHeading != "" {
			return
		}
		joined := strings.TrimSpace(strings.Join(current, " "))
		current = nil
		if joined != "" {
			hero = append(hero, joined)
		}
	}
	flushSection := func() {
		if currentHeading == "" {
			current = nil
			return
		}
		sec := readmeSection{title: currentHeading, lines: append([]string{}, current...)}
		current = nil
		if len(sec.lines) > 0 || featureHeadingScore(sec.title) > 0 {
			sections = append(sections, sec)
		}
	}

	for _, raw := range strings.Split(text, "\n") {
		body, quoted := quoteLine(raw)
		if quoted && isAdmonitionMarker(body) {
			flushHero()
			inAdmonition = true
			continue
		}
		if inAdmonition {
			if quoted {
				cleaned := stripMarkdownInline(body)
				if cleaned != "" {
					fallback = append(fallback, cleaned)
				}
				continue
			}
			inAdmonition = false
			if strings.TrimSpace(raw) == "" {
				continue
			}
			body = strings.TrimSpace(raw)
		}
		if strings.HasPrefix(body, "#") {
			heading := stripMarkdownInline(strings.TrimSpace(strings.TrimLeft(body, "#")))
			if title == "" && heading != "" {
				title = heading
				afterTitle = true
				continue
			}
			if !afterTitle {
				continue
			}
			flushHero()
			flushSection()
			currentHeading = heading
			continue
		}
		if !afterTitle {
			continue
		}
		if currentHeading == "" && isHorizontalRule(body) {
			flushHero()
			continue
		}
		if skipIntroLine(body) {
			if currentHeading == "" {
				flushHero()
			}
			continue
		}
		cleaned := stripMarkdownInline(body)
		if skipIntroLine(cleaned) || (title != "" && strings.EqualFold(cleaned, title)) {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(body), "- ") || strings.HasPrefix(strings.TrimSpace(body), "* ") {
			cleaned = "- " + strings.TrimSpace(strings.TrimLeft(cleaned, "-* "))
		}
		current = append(current, cleaned)
	}
	flushHero()
	flushSection()

	intro := joinIntroLines(hero)
	if chineseCount(intro) < 4 {
		if fb := joinIntroLines(fallback); chineseCount(fb) >= 4 || (intro == "" && fb != "") {
			intro = fb
		}
	}
	if features := formatFeatureSections(sections); features != "" {
		if intro != "" {
			intro += "\n\n" + features
		} else {
			intro = features
		}
	}
	runes := []rune(intro)
	if len(runes) > 2500 {
		intro = string(runes[:2500])
	}
	return title, intro
}

func featureHeadingScore(heading string) int {
	n := normalizeHeading(heading)
	if n == "" {
		return 0
	}
	skip := []string{"preview", "screenshot", "install", "gettingstarted", "quickstart", "toc", "contents",
		"contributing", "license", "changelog", "faq", "sponsor", "目录", "预览", "安装", "快速开始",
		"贡献", "许可", "更新日志", "常见问题", "赞助"}
	for _, key := range skip {
		if strings.Contains(n, key) {
			return 0
		}
	}
	switch {
	case strings.Contains(n, "features"), strings.Contains(n, "highlights"),
		strings.Contains(n, "核心亮点"), strings.Contains(n, "特性"), strings.Contains(n, "亮点"):
		return 3
	case strings.Contains(n, "keyfeatures"), strings.Contains(n, "主要功能"),
		strings.Contains(n, "核心功能"), strings.Contains(n, "功能特点"), strings.Contains(n, "功能特色"):
		return 2
	case strings.Contains(n, "功能"):
		return 1
	default:
		return 0
	}
}

func normalizeHeading(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.Is(unicode.Han, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func formatFeatureSections(sections []readmeSection) string {
	var strong, weak []readmeSection
	for _, sec := range sections {
		switch featureHeadingScore(sec.title) {
		case 0:
			continue
		case 1:
			weak = append(weak, sec)
		default:
			strong = append(strong, sec)
		}
	}
	picked := strong
	if len(picked) == 0 {
		picked = weak
	}
	if len(picked) == 0 {
		return ""
	}
	var parts []string
	for i, sec := range picked {
		if i >= 2 {
			break
		}
		lines := make([]string, 0, len(sec.lines)+1)
		if sec.title != "" {
			lines = append(lines, sec.title)
		}
		for _, line := range sec.lines {
			line = strings.TrimSpace(line)
			if line == "" || isLanguageNav(line) || isBareURL(line) {
				continue
			}
			lines = append(lines, line)
			if len(lines) >= 16 {
				break
			}
		}
		if len(lines) > 1 {
			parts = append(parts, strings.Join(lines, "\n"))
		}
	}
	return strings.Join(parts, "\n\n")
}

func stringValue(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
