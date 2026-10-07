package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"lotteryd/internal/lottery"
	"lotteryd/internal/store"
)

// ---- 任务模块 handlers ----

type taskInput struct {
	Name            string                 `json:"name"`
	Cover           string                 `json:"cover"`
	Description     string                 `json:"description"`
	GroupID         int64                  `json:"group_id"`
	GroupName       string                 `json:"group_name"`
	Model           string                 `json:"model"`
	StartDate       string                 `json:"start_date"`
	DurationDays    int                    `json:"duration_days"`
	SettleTime      string                 `json:"settle_time"`
	ThresholdTokens float64                `json:"threshold_tokens"`
	RewardType      string                 `json:"reward_type"`
	RewardValue     float64                `json:"reward_value"`
	RepeatPolicy    string                 `json:"repeat_policy"`
	Whitelist       []string               `json:"whitelist"`
	Blacklist       []string               `json:"blacklist"`
	Conditions      []lottery.ConditionDef `json:"conditions"`
	Status          string                 `json:"status"`
}

func (in *taskInput) toTask() *lottery.Task {
	return &lottery.Task{
		Name: in.Name, Cover: in.Cover, Description: in.Description,
		GroupID: in.GroupID, GroupName: in.GroupName, Model: in.Model,
		StartDate: in.StartDate, DurationDays: in.DurationDays, SettleTime: in.SettleTime,
		ThresholdTokens: in.ThresholdTokens,
		RewardType:      in.RewardType, RewardValue: in.RewardValue,
		RepeatPolicy: in.RepeatPolicy,
		Whitelist:    in.Whitelist, Blacklist: in.Blacklist,
		Conditions: in.Conditions,
		Status:     in.Status,
	}
}

// ---- 用户侧 ----

// handleListTasks 用户侧进行中的任务列表（全局显隐 + 白名单/黑名单过滤可见性）。
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request, claims *Claims) {
	if !s.App.IsUserAllowedTask(claims.Role, claims.Email) {
		ok(w, map[string]any{"tasks": []any{}})
		return
	}
	tasks, err := s.App.ListUserTasks(claims.Email, time.Now())
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"tasks": tasks})
}

// handleTaskPrompts 符合可参与条件的进行中任务（供全局引导弹窗，WS 断开时的轮询回退）。
func (s *Server) handleTaskPrompts(w http.ResponseWriter, r *http.Request, claims *Claims) {
	tasks, err := s.App.TaskPromptList(r.Context(), claims.UserID, claims.Email, claims.Role, claims.RegisteredAt)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"tasks": tasks})
}

// handleMyTasksPhase 用户侧角标探测：该用户有可见的进行中任务 → active。
func (s *Server) handleMyTasksPhase(w http.ResponseWriter, r *http.Request, claims *Claims) {
	if !s.App.IsUserAllowedTask(claims.Role, claims.Email) {
		ok(w, map[string]any{"phase": "none"})
		return
	}
	phase, err := s.App.TasksPhase(claims.Email, time.Now())
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"phase": phase})
}

// handleMyTaskVisibility 用户侧任务中心显隐结果（供侧边栏入口显隐与页面未开放态）。
func (s *Server) handleMyTaskVisibility(w http.ResponseWriter, r *http.Request, claims *Claims) {
	ok(w, map[string]any{"visible": s.App.IsUserAllowedTask(claims.Role, claims.Email)})
}

// handleMyTaskRewards 用户自己的任务结算/奖励记录（含兑换码）。
func (s *Server) handleMyTaskRewards(w http.ResponseWriter, r *http.Request, claims *Claims) {
	rewards, err := s.App.MyTaskRewards(claims.UserID)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"rewards": rewards})
}

// handleAdminGetTaskSettings 任务显隐设置。
func (s *Server) handleAdminGetTaskSettings(w http.ResponseWriter, r *http.Request, _ *Claims) {
	ok(w, map[string]any{"visibility": s.App.GetTaskVisibility()})
}

// handleAdminSaveTaskSettings 保存任务显隐设置。
func (s *Server) handleAdminSaveTaskSettings(w http.ResponseWriter, r *http.Request, _ *Claims) {
	var body struct {
		Visibility *lottery.Visibility `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid body: "+err.Error())
		return
	}
	if body.Visibility != nil {
		if err := s.App.SetTaskVisibility(*body.Visibility); err != nil {
			internalError(w, err)
			return
		}
		s.wsHub.Wake()
	}
	ok(w, map[string]any{"saved": true})
}

func (s *Server) handleAdminListTasks(w http.ResponseWriter, r *http.Request, _ *Claims) {
	tasks, err := s.App.AdminListTasks()
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"tasks": tasks})
}

func (s *Server) handleAdminCreateTask(w http.ResponseWriter, r *http.Request, _ *Claims) {
	var in taskInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid body: "+err.Error())
		return
	}
	t := in.toTask()
	if err := t.Validate(); err != nil {
		fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	id, err := s.App.CreateTask(t)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	s.wsHub.Wake()
	ok(w, map[string]any{"id": id})
}

func (s *Server) handleAdminGetTask(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid task id")
		return
	}
	t, err := s.App.Store.GetTask(id)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	codes, _ := s.App.ListTaskCodes(id)
	rewards, _ := s.App.ListTaskRewards(id)
	ok(w, map[string]any{"task": t, "codes": codes, "rewards": rewards})
}

func (s *Server) handleAdminUpdateTask(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid task id")
		return
	}
	var in taskInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid body: "+err.Error())
		return
	}
	t := in.toTask()
	t.ID = id
	if err := s.App.UpdateTask(t); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			mapStoreError(w, err)
			return
		}
		fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	s.wsHub.Wake()
	ok(w, map[string]any{"id": id})
}

func (s *Server) handleAdminDeleteTask(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid task id")
		return
	}
	if err := s.App.DeleteTask(id); err != nil {
		mapStoreError(w, err)
		return
	}
	s.wsHub.Wake()
	ok(w, map[string]any{"deleted": true})
}

func (s *Server) handleAdminTaskRewards(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid task id")
		return
	}
	rewards, err := s.App.ListTaskRewards(id)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"rewards": rewards})
}

func (s *Server) handleAdminTaskFulfill(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid task id")
		return
	}
	if err := s.App.RetryTaskFulfillment(r.Context(), id); err != nil {
		mapStoreError(w, err)
		return
	}
	ok(w, map[string]any{"retried": true})
}

// handleAdminTaskCodesGet 码池明细。
func (s *Server) handleAdminTaskCodesGet(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid task id")
		return
	}
	codes, err := s.App.ListTaskCodes(id)
	if err != nil {
		internalError(w, err)
		return
	}
	available, granted, _ := s.App.Store.TaskCodeCounts(id)
	ok(w, map[string]any{"codes": codes, "available": available, "granted": granted})
}

// handleAdminTaskCodesPut 整体重置可用码池（granted 记录保留）。
func (s *Server) handleAdminTaskCodesPut(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid task id")
		return
	}
	var body struct {
		Codes []string `json:"codes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid body: "+err.Error())
		return
	}
	added, err := s.App.SetTaskCodes(id, body.Codes)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	available, granted, _ := s.App.Store.TaskCodeCounts(id)
	ok(w, map[string]any{"added": added, "available": available, "granted": granted})
}

// handleAdminTaskGroups 分组下拉数据（代理 sub2api 集成接口）。
func (s *Server) handleAdminTaskGroups(w http.ResponseWriter, r *http.Request, _ *Claims) {
	groups, err := s.App.TaskGroups(r.Context())
	if err != nil {
		fail(w, http.StatusBadGateway, 502, "上游主服务错误: "+err.Error())
		return
	}
	ok(w, map[string]any{"groups": groups})
}

// handleAdminTaskGroupModels 某分组的模型下拉数据。
func (s *Server) handleAdminTaskGroupModels(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid group id")
		return
	}
	models, err := s.App.TaskGroupModels(r.Context(), id)
	if err != nil {
		fail(w, http.StatusBadGateway, 502, "上游主服务错误: "+err.Error())
		return
	}
	ok(w, map[string]any{"models": models})
}
