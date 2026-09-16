package appcatalog

import "time"

const (
	DefaultListen                    = "127.0.0.1:18099"
	DefaultProviderPricingPath       = "resources/model-pricing/model_prices_and_context_window.json"
	DefaultProviderPricingMultiplier = 0.08
)

type App struct {
	ID              int64      `json:"id"`
	OfficialURL     string     `json:"official_url"`
	GitHubURL       string     `json:"github_url"`
	Name            string     `json:"name"`
	IconURL         string     `json:"icon_url"`
	Description     string     `json:"description"`
	DownloadPageURL string     `json:"download_page_url"`
	Screenshots     []string   `json:"screenshots"`
	Changelog       string     `json:"changelog"`
	Version         string     `json:"version"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastFetchedAt   *time.Time `json:"last_fetched_at,omitempty"`
	FetchError      string     `json:"fetch_error,omitempty"`
}

type FetchResult struct {
	Name            string   `json:"name"`
	IconURL         string   `json:"icon_url"`
	Description     string   `json:"description"`
	DownloadPageURL string   `json:"download_page_url"`
	Screenshots     []string `json:"screenshots"`
	Changelog       string   `json:"changelog"`
	Version         string   `json:"version"`
	OfficialURL     string   `json:"official_url,omitempty"`
	GitHubURL       string   `json:"github_url,omitempty"`
}

type CreateAppInput struct {
	OfficialURL     string   `json:"official_url"`
	GitHubURL       string   `json:"github_url"`
	Name            string   `json:"name"`
	IconURL         string   `json:"icon_url"`
	Description     string   `json:"description"`
	DownloadPageURL string   `json:"download_page_url"`
	Screenshots     []string `json:"screenshots"`
	Changelog       string   `json:"changelog"`
	Version         string   `json:"version"`
}

type fetchApplyMode int

const (
	fetchFillEmpty fetchApplyMode = iota
	fetchRefreshRelease
)
