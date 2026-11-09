package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

var libraryID = regexp.MustCompile(`^mf[0-9a-f]{32}$`)

type libraryFileState struct {
	TaskID    string     `json:"task_id"`
	FileID    string     `json:"file_id"`
	Available bool       `json:"available"`
	Deleted   bool       `json:"deleted"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (h *MediaHubController) RegisterUser(user *gin.RouterGroup) {
	g := user.Group("/media-files")
	g.GET("", h.listLibrary)
	g.POST("/delete", h.deleteLibrary)
}

func (h *MediaHubController) libraryStates(ctx context.Context, userID int64, records []mediahub.ResultLink, remove bool) ([]libraryFileState, error) {
	s := h.settings.Active()
	client := &mediahub.ManagementClient{BaseURL: s.MediadBaseURL, Token: s.ManagementToken, HTTP: &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	refs := make([]libraryFileState, 0, len(records))
	for _, record := range records {
		refs = append(refs, libraryFileState{TaskID: record.TaskID, FileID: record.FileID})
	}
	path := "/management/files/status"
	if remove {
		path = "/management/files/delete"
	}
	raw, err := client.Request(ctx, http.MethodPost, path, gin.H{"user_id": userID, "files": refs})
	if err != nil {
		return nil, err
	}
	var out struct {
		Files []libraryFileState `json:"files"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if len(out.Files) != len(records) {
		return nil, errors.New("incomplete media response")
	}
	for i, state := range out.Files {
		if state.TaskID != records[i].TaskID || state.FileID != records[i].FileID {
			return nil, errors.New("mismatched media response")
		}
	}
	return out.Files, nil
}

func (h *MediaHubController) libraryOwner(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Error(c, 401, "请先登录")
		return 0, false
	}
	if h.links == nil {
		response.Error(c, 503, "媒体记录暂不可用")
		return 0, false
	}
	c.Header("Cache-Control", "private, no-store")
	return subject.UserID, true
}

func (h *MediaHubController) listLibrary(c *gin.Context) {
	userID, ok := h.libraryOwner(c)
	if !ok {
		return
	}
	after := c.Query("after")
	if after != "" && !libraryID.MatchString(after) {
		response.Error(c, 400, "分页位置不正确")
		return
	}
	kind := c.Query("kind")
	if kind != "" && kind != "image" && kind != "video" {
		response.Error(c, 400, "作品类型不正确")
		return
	}
	records, next, err := h.links.LibraryPage(c.Request.Context(), userID, after, kind)
	if err != nil {
		response.Error(c, 503, "媒体记录加载失败")
		return
	}
	type item struct {
		mediahub.ResultLink
		ExpiresAt *time.Time `json:"expires_at"`
	}
	items := []item{}
	if len(records) > 0 {
		states, err := h.libraryStates(c.Request.Context(), userID, records, false)
		if err != nil {
			response.Error(c, 503, "媒体服务暂不可用，请稍后重试")
			return
		}
		for i, state := range states {
			if state.Available {
				items = append(items, item{records[i], state.ExpiresAt})
			}
		}
	}
	response.Success(c, gin.H{"items": items, "next_cursor": next})
}

func (h *MediaHubController) deleteLibrary(c *gin.Context) {
	userID, ok := h.libraryOwner(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	var in struct {
		IDs []string `json:"ids"`
	}
	if c.ShouldBindJSON(&in) != nil || len(in.IDs) == 0 || len(in.IDs) > 50 {
		response.Error(c, 400, "每次可删除 1 到 50 个文件")
		return
	}
	seen := map[string]bool{}
	for _, id := range in.IDs {
		if !libraryID.MatchString(id) || seen[id] {
			response.Error(c, 400, "文件编号不正确或重复")
			return
		}
		seen[id] = true
	}
	deleted, failed := []string{}, []string{}
	records := []mediahub.ResultLink{}
	for _, id := range in.IDs {
		record, err := h.links.Get(c.Request.Context(), userID, id)
		if errors.Is(err, sql.ErrNoRows) {
			deleted = append(deleted, id)
			continue
		}
		if err != nil {
			failed = append(failed, id)
			continue
		}
		records = append(records, *record)
	}
	if len(records) > 0 {
		states, err := h.libraryStates(c.Request.Context(), userID, records, true)
		if err != nil {
			response.Error(c, 503, "删除未完成，请重试")
			return
		}
		for i, state := range states {
			id := records[i].ID
			if !state.Deleted || h.links.MarkDeleted(c.Request.Context(), userID, id) != nil {
				failed = append(failed, id)
			} else {
				deleted = append(deleted, id)
			}
		}
	}
	response.Success(c, gin.H{"deleted_ids": deleted, "failed_ids": failed})
}
