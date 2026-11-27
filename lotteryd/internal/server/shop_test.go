package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPointsShopRoutesRequireAdmin(t *testing.T) {
	handler := New(nil, "shop-test", "test").Handler()
	for _, route := range []struct{ method, path string }{
		{"POST", "/v1/admin/points/assets"}, {"GET", "/v1/admin/points/items"}, {"POST", "/v1/admin/points/items"}, {"PUT", "/v1/admin/points/items/1"},
		{"GET", "/v1/admin/points/products"}, {"POST", "/v1/admin/points/products"}, {"PUT", "/v1/admin/points/products/1"}, {"PUT", "/v1/admin/points/products/1/status"},
	} {
		for _, role := range []string{"", "user"} {
			r := httptest.NewRequest(route.method, route.path, nil)
			if role != "" {
				r.Header.Set("Authorization", "Bearer "+signJWT("shop-test", 2, role))
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if role == "" {
				require.Equal(t, 401, w.Code)
			} else {
				require.Equal(t, 403, w.Code)
			}
		}
	}
}

func TestPointsShopUploadCreateEditPublishAndHide(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	admin := signJWT(e.secret, 2, "admin")
	user := signJWT(e.secret, 2, "user")
	upload := func(name, slot, contents string) string {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", name)
		require.NoError(t, err)
		_, err = part.Write([]byte(contents))
		require.NoError(t, err)
		require.NoError(t, writer.WriteField("slot", slot))
		require.NoError(t, writer.Close())
		r, err := http.NewRequest("POST", e.srv.URL+"/v1/admin/points/assets", &body)
		require.NoError(t, err)
		r.Header.Set("Authorization", "Bearer "+admin)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		resp, err := http.DefaultClient.Do(r)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		return mustJSON(t, resp)["id"].(string)
	}
	cover := upload("cover.svg", "cover", testItemSVG)
	active := upload("active.svg", "active", strings.Replace(testItemSVG, "red", "blue", 1))
	expired := upload("expired.svg", "expired", strings.Replace(testItemSVG, "red", "gray", 1))
	animation := upload("animation.json", "animation", testItemLottie)
	item := map[string]any{"code": "test-item", "name": "Test item", "description": "Item description", "cover_asset": cover, "active_asset": active, "expired_asset": expired, "animation_asset": animation}
	data := mustJSON(t, e.req(t, "POST", "/v1/admin/points/items", admin, item))
	id := int64(data["id"].(float64))
	require.Positive(t, id)
	data = mustJSON(t, e.req(t, "GET", "/v1/admin/points/items", admin, nil))
	require.Len(t, data["items"], 1)
	missing := map[string]any{"code": "missing", "name": "Missing", "cover_asset": cover}
	resp := e.req(t, "POST", "/v1/admin/points/items", admin, missing)
	require.Equal(t, 400, resp.StatusCode)
	resp.Body.Close()
	product := map[string]any{"kind": "item", "item_id": id, "price": "100", "status": "draft", "sort_order": 0}
	data = mustJSON(t, e.req(t, "POST", "/v1/admin/points/products", admin, product))
	productID := int64(data["id"].(float64))
	require.Equal(t, "Test item", data["name"])
	data = mustJSON(t, e.req(t, "GET", "/v1/points/shop", user, nil))
	require.Empty(t, data["items"])
	path := "/v1/admin/points/products/" + strconv.FormatInt(productID, 10) + "/status"
	mustJSON(t, e.req(t, "PUT", path, admin, map[string]string{"status": "published"}))
	data = mustJSON(t, e.req(t, "GET", "/v1/points/shop", user, nil))
	require.Len(t, data["items"], 1)
	require.Equal(t, "100", data["items"].([]any)[0].(map[string]any)["price"])
	item["name"] = "Updated item"
	mustJSON(t, e.req(t, "PUT", "/v1/admin/points/items/"+strconv.FormatInt(id, 10), admin, item))
	data = mustJSON(t, e.req(t, "GET", "/v1/points/shop", user, nil))
	require.Equal(t, "Updated item", data["items"].([]any)[0].(map[string]any)["name"])
	mustJSON(t, e.req(t, "PUT", path, admin, map[string]string{"status": "draft"}))
	data = mustJSON(t, e.req(t, "GET", "/v1/points/shop", user, nil))
	require.Empty(t, data["items"])
	custom := map[string]any{"kind": "custom", "name": "Custom gift", "description": "A gift", "icon_asset": cover, "price": "900719925474", "status": "published", "sort_order": -1}
	data = mustJSON(t, e.req(t, "POST", "/v1/admin/points/products", admin, custom))
	require.Equal(t, "900719925474", data["price"])
	for _, price := range []string{"0", "-1", "1.5", "1000000000001"} {
		custom["price"] = price
		resp = e.req(t, "POST", "/v1/admin/points/products", admin, custom)
		require.Equal(t, 400, resp.StatusCode)
		resp.Body.Close()
	}
	custom["price"] = "10"
	custom["icon_asset"] = animation
	resp = e.req(t, "POST", "/v1/admin/points/products", admin, custom)
	require.Equal(t, 400, resp.StatusCode)
	resp.Body.Close()
	resp = e.req(t, "GET", "/v1/points/assets/"+cover, "", nil)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "image/svg+xml", resp.Header.Get("Content-Type"))
	require.Contains(t, resp.Header.Get("Content-Security-Policy"), "sandbox")
	resp.Body.Close()
	resp = e.req(t, "GET", "/v1/points/assets/"+animation, "", nil)
	require.Equal(t, 200, resp.StatusCode)
	var raw map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))
	require.EqualValues(t, 30, raw["fr"])
	resp.Body.Close()
}
