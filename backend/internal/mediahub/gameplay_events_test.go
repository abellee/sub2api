package mediahub

import (
	"encoding/json"
	"lotteryd/api/events"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGameplayCallbackRequiresSignatureAndDoesNotSettle(t *testing.T) {
	var captured []events.Event
	events.SetSink(func(e events.Event) { captured = append(captured, e) })
	t.Cleanup(func() { events.SetSink(nil) })
	s := NewSettleServer(func() string { return "event-secret" }, sinkFunc(func(SettleRequest) error { t.Fatal("gameplay callback entered billing"); return nil }), time.Minute)
	e := events.Event{ID: "mediad:test", Type: "media.succeeded", Source: "mediad", UserID: 2, Reference: "mtask_test", OccurredAt: time.Now().UTC(), Data: map[string]string{"source_id": "mtask_test", "model": "image-model", "credential": "never-forward"}}
	post := func(sign bool) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(e)
		r := httptest.NewRequest("POST", "/internal/media/settle", strings.NewReader(string(raw)))
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		r.Header.Set(HeaderTimestamp, ts)
		r.Header.Set(HeaderEvent, "gameplay.event")
		if sign {
			r.Header.Set(HeaderSignature, Sign("event-secret", ts, r.Method, r.URL.Path, raw))
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if w := post(false); w.Code != 401 || len(captured) != 0 {
		t.Fatalf("unsigned status=%d events=%d", w.Code, len(captured))
	}
	if w := post(true); w.Code != 200 || len(captured) != 1 {
		t.Fatalf("signed status=%d events=%d", w.Code, len(captured))
	}
	if _, ok := captured[0].Data["credential"]; ok {
		t.Fatal("non-whitelisted data forwarded")
	}
	e.Type = "points.granted"
	if w := post(true); w.Code != 400 || len(captured) != 1 {
		t.Fatalf("non-media status=%d events=%d", w.Code, len(captured))
	}
}
