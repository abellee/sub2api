package service

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

//go:embed documentation_defaults.json
var documentationDefaults []byte

var ErrDocumentationConflict = errors.New("文档已被其他管理员更新，请重新加载后再保存")
var documentationID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$`)
var documentationArticleID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,79}$`)
var documentationVariable = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

type DocumentationCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DocumentationAgent struct {
	Enabled     bool    `json:"enabled"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Template    string  `json:"template"`
	BaseURL     string  `json:"base_url"`
	KeyMode     string  `json:"key_mode"`
	GroupIDs    []int64 `json:"group_ids"`
}

type DocumentationArticle struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Logo       string             `json:"logo"`
	CategoryID string             `json:"category_id"`
	Content    string             `json:"content"`
	Published  bool               `json:"published"`
	Agent      DocumentationAgent `json:"agent"`
}

type DocumentationSnapshot struct {
	Revision   int64                   `json:"revision"`
	Categories []DocumentationCategory `json:"categories"`
	Articles   []DocumentationArticle  `json:"articles"`
}

func (s *GroupCategoryStore) Documentation(ctx context.Context) (*DocumentationSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var snap *DocumentationSnapshot
	if err := s.readConfig(ctx, "documentation", &snap); err != nil {
		return nil, err
	}
	if snap == nil {
		if err := json.Unmarshal(documentationDefaults, &snap); err != nil {
			return nil, err
		}
	}
	return snap, nil
}

func ValidateDocumentation(snap *DocumentationSnapshot) error {
	if snap == nil || snap.Revision < 0 || snap.Categories == nil || snap.Articles == nil || len(snap.Categories) > 100 || len(snap.Articles) > 300 {
		return fmt.Errorf("文档格式无效，最多支持 100 个分类和 300 篇文章")
	}
	categories := map[string]bool{}
	names := map[string]bool{}
	for i := range snap.Categories {
		cat := &snap.Categories[i]
		cat.Name = strings.TrimSpace(cat.Name)
		if !documentationID.MatchString(cat.ID) || categories[cat.ID] || names[cat.Name] || cat.Name == "" || len([]rune(cat.Name)) > 80 {
			return fmt.Errorf("分类标识重复或名称无效")
		}
		categories[cat.ID] = true
		names[cat.Name] = true
	}
	ids := map[string]bool{}
	logos := map[string]bool{"": true, "openai": true, "gemini": true, "azure": true, "grok": true, "adobe": true, "bytedance": true, "midjourney": true, "alibaba": true}
	variables := map[string]bool{"article_name": true, "base_url": true, "api_key": true, "document": true, "group_name": true}
	for i := range snap.Articles {
		article := &snap.Articles[i]
		article.Name = strings.TrimSpace(article.Name)
		if !documentationArticleID.MatchString(article.ID) || ids[article.ID] {
			return fmt.Errorf("文章标识无效或重复")
		}
		ids[article.ID] = true
		if article.Name == "" || len([]rune(article.Name)) > 160 || !categories[article.CategoryID] || !logos[article.Logo] {
			return fmt.Errorf("文章名称、分类或 Logo 无效")
		}
		if strings.TrimSpace(article.Content) == "" || len(article.Content) > 1024*1024 {
			return fmt.Errorf("文章内容不能为空，且不能超过 1 MB")
		}
		agent := &article.Agent
		if len(agent.Template) > 64000 || len([]rune(agent.Title)) > 160 || len(agent.Description) > 4000 {
			return fmt.Errorf("提示词或说明过长")
		}
		switch agent.KeyMode {
		case "all", "groups", "manual", "none":
		default:
			return fmt.Errorf("密钥读取方式无效")
		}
		if agent.Enabled && (strings.TrimSpace(agent.Title) == "" || strings.TrimSpace(agent.Template) == "") {
			return fmt.Errorf("请填写 Agent 标题和提示词")
		}
		if len(agent.GroupIDs) > 300 || (agent.KeyMode == "groups" && len(agent.GroupIDs) == 0) {
			return fmt.Errorf("请选择允许读取 Key 的分组")
		}
		seen := map[int64]bool{}
		for _, id := range agent.GroupIDs {
			if id <= 0 || seen[id] {
				return fmt.Errorf("密钥分组无效或重复")
			}
			seen[id] = true
		}
		if agent.BaseURL != "" {
			u, err := url.Parse(agent.BaseURL)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
				return fmt.Errorf("接口地址必须是有效的 HTTPS 地址，不能包含账号、参数或锚点")
			}
		}
		for _, match := range documentationVariable.FindAllStringSubmatch(agent.Template, -1) {
			if !variables[strings.TrimSpace(match[1])] {
				return fmt.Errorf("未知提示词占位符：%s", match[1])
			}
		}
	}
	return nil
}

func (s *GroupCategoryStore) SaveDocumentation(ctx context.Context, snap *DocumentationSnapshot) error {
	if err := ValidateDocumentation(snap); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := snap.Revision
	next := *snap
	next.Revision++
	raw, err := json.Marshal(&next)
	if err != nil {
		return err
	}
	db, err := s.openConfigDB(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	var result sql.Result
	if previous == 0 {
		result, err = db.ExecContext(ctx, `INSERT OR IGNORE INTO ui_configuration(key, value) VALUES ('documentation', ?)`, string(raw))
	} else {
		result, err = db.ExecContext(ctx, `UPDATE ui_configuration SET value = ? WHERE key = 'documentation' AND json_extract(value, '$.revision') = ?`, string(raw), previous)
	}
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrDocumentationConflict
	}
	snap.Revision = next.Revision
	return nil
}
