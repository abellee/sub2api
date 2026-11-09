package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// BuildDocumentationSkill packages the saved instructions and full reference without personal keys.
func BuildDocumentationSkill(article DocumentationArticle) ([]byte, error) {
	const name = "tokenfree-creative"
	title := strings.NewReplacer("<", "〈", ">", "〉").Replace(article.Name)
	description, _ := json.Marshal("在用户需要使用「" + title + "」的接口或将其接入项目时使用，包含接入说明和完整接口参考。")
	values := map[string]string{
		"article_name": article.Name,
		"base_url":     article.Agent.BaseURL,
		"api_key":      "<从本地环境变量 TOKENFREE_API_KEY 读取>",
		"group_name":   "<API Key 所属分组>",
		"document":     "请阅读 [完整接口文档](references/api.md)。",
	}
	prompt := documentationVariable.ReplaceAllStringFunc(article.Agent.Template, func(token string) string {
		key := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(token, "{{"), "}}"))
		if value, ok := values[key]; ok {
			return value
		}
		return token
	})
	skill := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# TokenFree Creative\n\n当前文档：%s\n\n先阅读 [完整接口文档](references/api.md)，按其中的接口、参数和限制完成用户要求的任务。\n\n%s\n", name, description, article.Name, prompt)
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	for _, file := range []struct{ path, content string }{
		{name + "/SKILL.md", skill},
		{name + "/references/api.md", article.Content},
		{name + "/agents/openai.yaml", "interface:\n  display_name: TokenFree Creative\n  short_description: TokenFree 创作接口提示词与完整文档\n"},
	} {
		entry, err := archive.Create(file.path)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write([]byte(file.content)); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
