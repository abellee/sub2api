package costguard

import "math"

// TokenPrices contains per-token prices supplied by the host.
type TokenPrices struct{ Input, CacheRead, CacheCreation float64 }

func validPrice(v float64) bool { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }
