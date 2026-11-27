package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"lotteryd/internal/store"
)

func pointsView(points store.PointsBalance) map[string]string {
	return map[string]string{"balance": strconv.FormatInt(points.Balance, 10), "total_earned": strconv.FormatInt(points.TotalEarned, 10), "total_spent": strconv.FormatInt(points.TotalSpent, 10)}
}

func (s *Server) handleMyCheckIn(w http.ResponseWriter, r *http.Request, claims *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	status, err := s.App.Store.GetDailyCheckIn(claims.UserID, time.Now())
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, status)
}

func (s *Server) handleClaimCheckIn(w http.ResponseWriter, r *http.Request, claims *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	status, points, granted, err := s.App.Store.ClaimDailyCheckIn(claims.UserID, time.Now())
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"status": status, "points": pointsView(points), "granted": granted})
}

func (s *Server) handleGetPointsSettings(w http.ResponseWriter, r *http.Request, _ *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	settings, err := s.App.Store.GetPointsSettings()
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, settings)
}

func (s *Server) handleSavePointsSettings(w http.ResponseWriter, r *http.Request, _ *Claims) {
	var settings store.PointsSettings
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid daily check-in rewards")
		return
	}
	settings.Normalize()
	if err := settings.Validate(); err != nil {
		fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if err := s.App.Store.SavePointsSettings(settings); err != nil {
		internalError(w, err)
		return
	}
	ok(w, settings)
}

func (s *Server) pointsTarget(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil || userID < 1 {
		fail(w, http.StatusBadRequest, 400, "invalid user ID")
		return 0, false
	}
	user, err := s.App.Sub2API.GetUser(r.Context(), userID, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if err != nil {
		fail(w, http.StatusBadGateway, 502, "unable to verify target user")
		return 0, false
	}
	if user.ID != userID {
		fail(w, http.StatusNotFound, 404, "user not found")
		return 0, false
	}
	return userID, true
}

func (s *Server) handleAdminUserPoints(w http.ResponseWriter, r *http.Request, _ *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	userID, valid := s.pointsTarget(w, r)
	if !valid {
		return
	}
	points, err := s.App.Store.GetPointsBalance(userID)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, pointsView(points))
}

var rechargeRequestID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (s *Server) handleAdminRechargePoints(w http.ResponseWriter, r *http.Request, claims *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	var body struct {
		Amount    string `json:"amount"`
		Notes     string `json:"notes"`
		RequestID string `json:"request_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid points recharge")
		return
	}
	amount, err := strconv.ParseInt(body.Amount, 10, 64)
	if err != nil || amount < 1 || amount > 1000000000000 || !rechargeRequestID.MatchString(body.RequestID) || len([]rune(body.Notes)) > 1000 {
		fail(w, http.StatusBadRequest, 400, "invalid points amount, notes or request ID")
		return
	}
	userID, valid := s.pointsTarget(w, r)
	if !valid {
		return
	}
	note := "管理员积分充值"
	if strings.TrimSpace(body.Notes) != "" {
		note += " · " + strings.TrimSpace(body.Notes)
	}
	points, granted, err := s.App.Store.GrantAdminPoints(userID, claims.UserID, amount, body.RequestID, note)
	if errors.Is(err, store.ErrPointsRechargeConflict) {
		fail(w, http.StatusConflict, 409, err.Error())
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"points": pointsView(points), "granted": granted})
}
