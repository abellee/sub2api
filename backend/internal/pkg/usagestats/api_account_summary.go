package usagestats

type APIAccountTotals struct {
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	UserCost     float64 `json:"user_cost"`
	AccountCost  float64 `json:"account_cost"`
	StandardCost float64 `json:"standard_cost"`
	Margin       float64 `json:"margin"`
}

type APIAccountSummaryItem struct {
	AccountID int64  `json:"account_id"`
	Name      string `json:"name"`
	APIAccountTotals
}

type APIAccountPeriod struct {
	Period string `json:"period"`
	APIAccountTotals
}

type APIAccountSummary struct {
	BaseURL     string                  `json:"base_url"`
	StartDate   string                  `json:"start_date"`
	EndDate     string                  `json:"end_date"`
	Timezone    string                  `json:"timezone"`
	Granularity string                  `json:"granularity"`
	Summary     APIAccountTotals        `json:"summary"`
	Accounts    []APIAccountSummaryItem `json:"accounts"`
	Periods     []APIAccountPeriod      `json:"periods"`
}
