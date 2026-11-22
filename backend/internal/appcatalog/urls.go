package appcatalog

import (
	"fmt"
	"net/url"
	"strings"
)

func NormalizeSourceURLs(officialRaw, githubRaw string) (official string, github string, err error) {
	official = strings.TrimSpace(officialRaw)
	github = strings.TrimSpace(githubRaw)
	if official == "" && github == "" {
		return "", "", fmt.Errorf("official_url or github_url is required")
	}
	if official != "" {
		official, err = normalizeHTTPURL(official)
		if err != nil {
			return "", "", fmt.Errorf("invalid official_url: %w", err)
		}
	}
	if github != "" {
		github, err = normalizeGitHubURL(github)
		if err != nil {
			return "", "", fmt.Errorf("invalid github_url: %w", err)
		}
	}
	return official, github, nil
}

func normalizeHTTPURL(raw string) (string, error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("only http/https URLs are allowed")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func normalizeGitHubURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimSuffix(trimmed, ".git")
	if strings.HasPrefix(trimmed, "git@github.com:") {
		trimmed = "https://github.com/" + strings.TrimPrefix(trimmed, "git@github.com:")
	}
	normalized, err := normalizeHTTPURL(trimmed)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(parsed.Host)
	if host != "github.com" && host != "www.github.com" {
		return "", fmt.Errorf("must be a github.com repository URL")
	}
	owner, repo, err := githubOwnerRepo(parsed)
	if err != nil {
		return "", err
	}
	return "https://github.com/" + owner + "/" + repo, nil
}

func githubOwnerRepo(parsed *url.URL) (string, string, error) {
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("must be a github.com/{owner}/{repo} URL")
	}
	repo := strings.TrimSuffix(parts[1], ".git")
	return parts[0], repo, nil
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func resolveURL(baseRaw, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	base, err := url.Parse(baseRaw)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}
