//go:build unit

package service

import (
	"archive/zip"
	"bytes"
	"io"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDocumentationSkillKeepsCompleteReferenceAndReusableInstructions(t *testing.T) {
	for _, article := range documentationFixture(t).Articles {
		t.Run(article.ID, func(t *testing.T) {
			article.Name = "Quoted \"name\" <version>:\n" + article.Name
			article.Agent.Template = "Use {{ article_name }} {{base_url}} {{api_key}} {{group_name}}\n{{document}}"
			article.Content += "\nLiteral reference placeholder: {{api_key}}"
			data, err := BuildDocumentationSkill(article)
			require.NoError(t, err)
			archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			require.NoError(t, err)
			require.Len(t, archive.File, 3)
			files := map[string]string{}
			for _, file := range archive.File {
				require.Equal(t, path.Clean(file.Name), file.Name)
				require.False(t, strings.HasPrefix(file.Name, "/"))
				reader, err := file.Open()
				require.NoError(t, err)
				content, err := io.ReadAll(reader)
				require.NoError(t, err)
				require.NoError(t, reader.Close())
				files[file.Name] = string(content)
			}
			root := strings.Split(archive.File[0].Name, "/")[0]
			require.Equal(t, "tokenfree-creative", root)
			require.Contains(t, files[root+"/agents/openai.yaml"], "display_name: TokenFree Creative")
			require.Less(t, len(root), 64)
			require.Regexp(t, `^[a-z0-9-]+$`, root)
			skill := files[root+"/SKILL.md"]
			parts := strings.SplitN(skill, "---\n", 3)
			require.Len(t, parts, 3)
			var metadata map[string]string
			require.NoError(t, yaml.Unmarshal([]byte(parts[1]), &metadata))
			require.Equal(t, root, metadata["name"])
			require.Contains(t, metadata["description"], strings.ReplaceAll(strings.ReplaceAll(article.Name, "<", "〈"), ">", "〉"))
			require.NotContains(t, metadata["description"], "<")
			require.Contains(t, skill, "# TokenFree Creative")
			require.Contains(t, skill, "TOKENFREE_API_KEY")
			require.Contains(t, skill, "references/api.md")
			require.NotContains(t, skill, "{{")
			require.NotContains(t, skill, article.Content)
			require.Equal(t, article.Content, files[root+"/references/api.md"])
		})
	}
}
