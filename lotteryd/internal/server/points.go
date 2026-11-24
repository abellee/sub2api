package server

import (
	"log/slog"
	"net/http"
	"strconv"
)

func (s *Server) handleMyPoints(w http.ResponseWriter, r *http.Request, claims *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	balance, err := s.App.Store.GetPointsBalance(claims.UserID)
	if err != nil {
		slog.Error("get user points", "user", claims.UserID, "err", err)
		fail(w, http.StatusInternalServerError, 500, "unable to load points")
		return
	}
	// Strings preserve bigint precision in browsers.
	ok(w, map[string]string{
		"balance":      strconv.FormatInt(balance.Balance, 10),
		"total_earned": strconv.FormatInt(balance.TotalEarned, 10),
		"total_spent":  strconv.FormatInt(balance.TotalSpent, 10),
	})
}

type pointsTransactionView struct {
	ID           string `json:"id"`
	Delta        string `json:"delta"`
	BalanceAfter string `json:"balance_after"`
	Kind         string `json:"kind"`
	Note         string `json:"note"`
	CreatedAt    string `json:"created_at"`
}

func (s *Server) handleMyPointsTransactions(w http.ResponseWriter, r *http.Request, claims *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	page, size := 1, 10
	for key, target := range map[string]*int{"page": &page, "page_size": &size} {
		if raw := r.URL.Query().Get(key); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || (key == "page_size" && n > 50) || n > 1000000 {
				fail(w, http.StatusBadRequest, 400, "invalid pagination")
				return
			}
			*target = n
		}
	}
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "all"
	}
	if kind != "all" && kind != "spend" && kind != "grant" {
		fail(w, http.StatusBadRequest, 400, "invalid points filter")
		return
	}
	records, total, err := s.App.Store.ListPointsTransactions(claims.UserID, kind, size, (page-1)*size)
	if err != nil {
		slog.Error("list user points transactions", "user", claims.UserID, "err", err)
		fail(w, http.StatusInternalServerError, 500, "unable to load points records")
		return
	}
	items := make([]pointsTransactionView, 0, len(records))
	for _, row := range records {
		items = append(items, pointsTransactionView{
			ID: strconv.FormatInt(row.ID, 10), Delta: strconv.FormatInt(row.Delta, 10),
			BalanceAfter: strconv.FormatInt(row.BalanceAfter, 10), Kind: row.Kind,
			Note: row.Note, CreatedAt: row.CreatedAt,
		})
	}
	ok(w, map[string]any{"items": items, "total": total, "page": page, "page_size": size})
}
