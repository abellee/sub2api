package server

import (
	"lotteryd/api/events"
	"net/http"
	"strings"
)

type eventResponse struct {
	http.ResponseWriter
	status int
}

func (w *eventResponse) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *eventResponse) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}
func (w *eventResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func observeAuthedEvent(w http.ResponseWriter, r *http.Request, claims *Claims, next authedHandler, admin bool) {
	if !events.Enabled() {
		next(w, r, claims)
		return
	}
	recorder := &eventResponse{ResponseWriter: w}
	next(recorder, r, claims)
	if recorder.status < 200 || recorder.status >= 300 {
		return
	}
	data := map[string]string{"endpoint": r.Pattern, "method": r.Method, "actor_id": events.S(claims.UserID), "action": strings.ToLower(r.Method)}
	if admin {
		if r.Method != "GET" && !strings.HasPrefix(r.URL.Path, "/v1/admin/events/") {
			events.Publish("admin.content_changed", 0, "", data)
		}
	} else if r.Method == "GET" {
		events.Publish("content.activity_view", claims.UserID, "", data)
	}
}
