package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"lotteryd/internal/app"
	"lotteryd/internal/lottery"
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
	Type     string              `json:"type"` // lottery_prompt | task_prompt | lottery_win | pong
	Activity *app.ActivityView   `json:"activity,omitempty"`
	Task     *app.TaskPromptView `json:"task,omitempty"`
	Winner   *lottery.Winner     `json:"winner,omitempty"`
}

type wsClient struct {
	mu          sync.Mutex
	closed      bool
	conn        *websocket.Conn
	userID      int64
	bearer      string
	email       string
	role        string
	regAt       *time.Time
	send        chan wsOutMessage
	lastIDs     map[int64]bool // 已推送过的活动 ID
	lastTaskIDs map[int64]bool // 已推送过的任务 ID
	lastWinIDs  map[int64]bool // 已见过的中奖 ID。连接后先记下存量，只推之后的新中奖。
	winsReady   bool
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
		c.closed = true
		close(c.send)
	}
	h.mu.Unlock()
}

func (h *wsHub) broadcastEligibility(ctx context.Context) {
	h.mu.Lock()
	groups := map[wsIdentity][]*wsClient{}
	for c := range h.clients {
		key := wsIdentity{c.userID, c.email, c.role, c.bearer}
		groups[key] = append(groups[key], c)
	}
	h.mu.Unlock()
	for _, clients := range groups {
		h.pushGroup(ctx, clients)
	}
}

type wsIdentity struct {
	userID              int64
	email, role, bearer string
}

// The same signed-in user may have several tabs. Calculate once, deliver to each.
func (h *wsHub) pushIfNew(ctx context.Context, c *wsClient) { h.pushGroup(ctx, []*wsClient{c}) }

func commonSeen(clients []*wsClient, tasks bool) map[int64]bool {
	out := map[int64]bool{}
	for i, c := range clients {
		ids := c.lastIDs
		if tasks {
			ids = c.lastTaskIDs
		}
		if i == 0 {
			for id, seen := range ids {
				if seen {
					out[id] = true
				}
			}
			continue
		}
		for id := range out {
			if !ids[id] {
				delete(out, id)
			}
		}
	}
	return out
}

func (h *wsHub) pushGroup(ctx context.Context, clients []*wsClient) {
	if len(clients) == 0 {
		return
	}
	for _, c := range clients {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	first := clients[0]
	usageCtx := app.WithUserUsageBearer(ctx, first.userID, first.bearer)
	activities, _ := h.sv.App.EligibilityListExcluding(usageCtx, first.userID, first.email, first.regAt, commonSeen(clients, false))
	tasks, _ := h.sv.App.TaskPromptListExcluding(ctx, first.userID, first.email, first.role, first.regAt, commonSeen(clients, true))
	wins, winErr := h.sv.App.MyWinnings(first.userID)
	for _, c := range clients {
		if !h.sendLottery(c, activities) || !h.sendTasks(c, tasks) {
			continue
		}
		if winErr == nil {
			h.sendWins(c, wins)
		}
	}
}

// pendingWinMessages 第一次只登记已有中奖，不发送。之后按从早到晚返回新中奖。
func pendingWinMessages(known map[int64]bool, ready bool, wins []lottery.Winner) ([]wsOutMessage, bool) {
	if !ready {
		for _, w := range wins {
			known[w.ID] = true
		}
		return nil, true
	}
	msgs := make([]wsOutMessage, 0)
	for i := len(wins) - 1; i >= 0; i-- {
		if known[wins[i].ID] {
			continue
		}
		cp := wins[i]
		known[cp.ID] = true
		msgs = append(msgs, wsOutMessage{Type: "lottery_win", Winner: &cp})
	}
	return msgs, true
}

func (h *wsHub) pushWins(c *wsClient) bool {
	wins, err := h.sv.App.MyWinnings(c.userID)
	if err != nil {
		return true
	}
	return h.sendWins(c, wins)
}

func (h *wsHub) sendWins(c *wsClient, wins []lottery.Winner) bool {
	if c.lastWinIDs == nil {
		c.lastWinIDs = map[int64]bool{}
	}
	msgs, ready := pendingWinMessages(c.lastWinIDs, c.winsReady, wins)
	c.winsReady = ready
	for _, msg := range msgs {
		if !h.trySend(c, msg) {
			return false
		}
	}
	return true
}

func (h *wsHub) trySend(c *wsClient, msg wsOutMessage) bool {
	h.mu.Lock()
	if c.closed {
		h.mu.Unlock()
		return false
	}
	select {
	case c.send <- msg:
		h.mu.Unlock()
		return true
	default:
		h.mu.Unlock()
		h.remove(c)
		return false
	}
}

func (h *wsHub) pushLottery(ctx context.Context, c *wsClient) bool {
	views, err := h.sv.App.EligibilityListExcluding(app.WithUserUsageBearer(ctx, c.userID, c.bearer), c.userID, c.email, c.regAt, c.lastIDs)
	if err != nil {
		return true
	}
	return h.sendLottery(c, views)
}

func (h *wsHub) sendLottery(c *wsClient, views []app.ActivityView) bool {
	if c.lastIDs == nil {
		c.lastIDs = map[int64]bool{}
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
	tasks, err := h.sv.App.TaskPromptListExcluding(ctx, c.userID, c.email, c.role, c.regAt, c.lastTaskIDs)
	if err != nil {
		return true
	}
	return h.sendTasks(c, tasks)
}

func (h *wsHub) sendTasks(c *wsClient, tasks []app.TaskPromptView) bool {
	if c.lastTaskIDs == nil {
		c.lastTaskIDs = map[int64]bool{}
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
		bearer:      token,
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
				s.wsHub.trySend(c, wsOutMessage{Type: "pong"})
			}
		}
	}
}
