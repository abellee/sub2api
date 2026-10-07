package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"lotteryd/internal/app"
)

// ---- WebSocket 实时推送 ----
//
// 客户端连接 GET /v1/ws?token=<jwt>（浏览器 WS 无法携带 Authorization 头，
// 走 query 参数，鉴权与 HTTP 接口同一套 local/introspect 逻辑）。
// 连接建立即推送一次当前符合条件且未参与的活动，以及符合可参与条件的进行中任务；
// 此后每当活动或任务发生变化（由各 handler 调 hub.Wake()）或每 30s 兜底 tick 时，
// 重新计算资格，发现新的可参与活动/任务才推送（已推过的 ID 不重发，
// 前端另有 localStorage 去重）。

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = (wsPongWait * 9) / 10
	wsTickEvery  = 30 * time.Second
)

type wsOutMessage struct {
	Type     string              `json:"type"` // lottery_prompt | task_prompt | pong
	Activity *app.ActivityView   `json:"activity,omitempty"`
	Task     *app.TaskPromptView `json:"task,omitempty"`
}

type wsClient struct {
	conn        *websocket.Conn
	userID      int64
	email       string
	role        string
	regAt       *time.Time
	send        chan wsOutMessage
	lastIDs     map[int64]bool // 已推送过的活动 ID
	lastTaskIDs map[int64]bool // 已推送过的任务 ID
}

type wsHub struct {
	sv      *Server
	mu      sync.Mutex
	clients map[*wsClient]struct{}
	wake    chan struct{}
}

func newWSHub(sv *Server) *wsHub {
	return &wsHub{sv: sv, clients: map[*wsClient]struct{}{}, wake: make(chan struct{}, 1)}
}

// Wake 唤醒 hub 立即重算一次资格（活动变化时由 handler 调用）。
func (h *wsHub) Wake() {
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

// Run 阻塞运行 hub（cmd/lotteryd 里放独立 goroutine）。
func (h *wsHub) Run(ctx context.Context) {
	ticker := time.NewTicker(wsTickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.wake:
		case <-ticker.C:
		}
		h.broadcastEligibility(ctx)
	}
}

func (h *wsHub) add(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *wsHub) remove(c *wsClient) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

func (h *wsHub) broadcastEligibility(ctx context.Context) {
	h.mu.Lock()
	targets := make([]*wsClient, 0, len(h.clients))
	for c := range h.clients {
		targets = append(targets, c)
	}
	h.mu.Unlock()
	for _, c := range targets {
		h.pushIfNew(ctx, c)
	}
}

// pushIfNew 计算该用户当前符合条件的活动和任务，发现新增（未推过的）就推送。
func (h *wsHub) pushIfNew(ctx context.Context, c *wsClient) {
	if !h.pushLottery(ctx, c) {
		return
	}
	h.pushTasks(ctx, c)
}

func (h *wsHub) trySend(c *wsClient, msg wsOutMessage) bool {
	select {
	case c.send <- msg:
		return true
	default:
		// 发送队列满视为连接僵死，丢弃连接
		h.remove(c)
		return false
	}
}

func (h *wsHub) pushLottery(ctx context.Context, c *wsClient) bool {
	views, err := h.sv.App.EligibilityList(ctx, c.userID, c.email, c.regAt)
	if err != nil {
		return true
	}
	for i := range views {
		v := &views[i]
		if v.Phase != "joining" && v.Phase != "upcoming" {
			continue
		}
		if c.lastIDs[v.ID] {
			continue
		}
		c.lastIDs[v.ID] = true
		if !h.trySend(c, wsOutMessage{Type: "lottery_prompt", Activity: v}) {
			return false
		}
	}
	return true
}

func (h *wsHub) pushTasks(ctx context.Context, c *wsClient) bool {
	tasks, err := h.sv.App.TaskPromptList(ctx, c.userID, c.email, c.role, c.regAt)
	if err != nil {
		return true
	}
	for i := range tasks {
		t := &tasks[i]
		if c.lastTaskIDs[t.ID] {
			continue
		}
		c.lastTaskIDs[t.ID] = true
		if !h.trySend(c, wsOutMessage{Type: "task_prompt", Task: t}) {
			return false
		}
	}
	return true
}

// ServeWS 处理 WS 升级与连接生命周期。挂在 GET /v1/ws。
func (s *Server) ServeWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	claims, err := s.claimsFromToken(r.Context(), token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	upgrader := websocket.Upgrader{
		// 鉴权已在上方完成；跨站由 token 把关，放行任意 Origin。
		CheckOrigin: func(*http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &wsClient{
		conn:        conn,
		userID:      claims.UserID,
		email:       claims.Email,
		role:        claims.Role,
		regAt:       claims.RegisteredAt,
		send:        make(chan wsOutMessage, 32),
		lastIDs:     map[int64]bool{},
		lastTaskIDs: map[int64]bool{},
	}
	s.wsHub.add(c)
	go s.wsHub.pushIfNew(r.Context(), c) // 连接建立立即推送一次
	go s.wsWritePump(c)
	s.wsReadPump(c) // 阻塞读（仅用于保活与退出），直到连接断开
}

func (s *Server) wsWritePump(c *wsClient) {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				s.wsHub.remove(c)
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.wsHub.remove(c)
				return
			}
		}
	}
}

// wsReadPump 只做保活：收到 pong 刷新读超时，连接断开时清理。
func (s *Server) wsReadPump(c *wsClient) {
	defer func() {
		s.wsHub.remove(c)
		_ = c.conn.Close()
	}()
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})
	for {
		mt, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		// 应用层心跳：客户端发 {"type":"ping"}，回 {"type":"pong"}
		if mt == websocket.TextMessage && len(data) > 0 && data[0] == '{' {
			var m struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(data, &m) == nil && m.Type == "ping" {
				select {
				case c.send <- wsOutMessage{Type: "pong"}:
				default:
				}
			}
		}
	}
}
