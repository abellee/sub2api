package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestListPublicModelPlazaGroupsFiltersAndSanitizes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	adminSvc.groups = []service.Group{
		{ID: 1, Name: "Public", Platform: service.PlatformOpenAI, RateMultiplier: 0.2, Status: service.StatusActive, ModelsListConfig: service.GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5"}}},
		{ID: 2, Name: "Exclusive", Platform: service.PlatformOpenAI, IsExclusive: true, Status: service.StatusActive, ModelsListConfig: service.GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5"}}},
		{ID: 3, Name: "Disabled", Platform: service.PlatformAnthropic, Status: service.StatusDisabled, ModelsListConfig: service.GroupModelsListConfig{Enabled: true, Models: []string{"claude-sonnet-4-6"}}},
		{ID: 4, Name: "No list", Platform: service.PlatformAnthropic, Status: service.StatusActive},
		{ID: 5, Name: "Configured list", Platform: service.PlatformOpenAI, RateMultiplier: 0.3, Status: service.StatusActive, ModelsListConfig: service.GroupModelsListConfig{Models: []string{"gpt-5.2"}}},
	}

	router := gin.New()
	handler := NewGroupHandler(adminSvc, nil, nil)
	router.GET("/api/v1/model-plaza/groups", handler.ListPublicModelPlazaGroups)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza/groups", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var body response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	encoded, err := json.Marshal(body.Data)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"name":"Public"`)
	require.Contains(t, string(encoded), `"name":"Configured list"`)
	require.NotContains(t, string(encoded), "Exclusive")
	require.NotContains(t, string(encoded), "Disabled")
	require.NotContains(t, string(encoded), "description")
	require.NotContains(t, string(encoded), "account")
}
