package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"lotteryd/internal/store"
)

var itemCodePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$`)

func shopError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrInvalidShopData):
		fail(w, 400, 400, err.Error())
	case errors.Is(err, sql.ErrNoRows):
		fail(w, 404, 404, "item or product not found")
	default:
		internalError(w, err)
	}
}

func decodeShopBody(w http.ResponseWriter, r *http.Request, value any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		fail(w, 400, 400, "invalid request body")
		return false
	}
	return true
}

func shopPathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		fail(w, 400, 400, "invalid ID")
		return 0, false
	}
	return id, true
}

func (s *Server) handleManagedItems(w http.ResponseWriter, r *http.Request, _ *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	items, err := s.App.Store.ListManagedItems()
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"items": items})
}

func (s *Server) handleSaveManagedItem(w http.ResponseWriter, r *http.Request, _ *Claims) {
	var item store.ManagedItem
	if !decodeShopBody(w, r, &item) {
		return
	}
	item.ID = 0
	if r.Method == http.MethodPut {
		var valid bool
		item.ID, valid = shopPathID(w, r)
		if !valid {
			return
		}
	}
	if !itemCodePattern.MatchString(item.Code) {
		fail(w, 400, 400, "item code must contain letters, digits, underscores or hyphens")
		return
	}
	item, err := s.App.Store.SaveManagedItem(item)
	if err != nil {
		shopError(w, err)
		return
	}
	ok(w, item)
}

func (s *Server) handleAdminShopProducts(w http.ResponseWriter, r *http.Request, _ *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	products, err := s.App.Store.ListShopProducts(false)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"items": products})
}

func (s *Server) handleShopProducts(w http.ResponseWriter, r *http.Request, _ *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	products, err := s.App.Store.ListShopProducts(true)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"items": products})
}

func (s *Server) handleSaveShopProduct(w http.ResponseWriter, r *http.Request, _ *Claims) {
	var product store.ShopProduct
	if !decodeShopBody(w, r, &product) {
		return
	}
	product.ID = 0
	if r.Method == http.MethodPut {
		var valid bool
		product.ID, valid = shopPathID(w, r)
		if !valid {
			return
		}
	}
	product, err := s.App.Store.SaveShopProduct(product)
	if err != nil {
		shopError(w, err)
		return
	}
	ok(w, product)
}

func (s *Server) handleShopProductStatus(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, valid := shopPathID(w, r)
	if !valid {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if !decodeShopBody(w, r, &body) {
		return
	}
	if err := s.App.Store.SetShopProductStatus(id, body.Status); err != nil {
		shopError(w, err)
		return
	}
	ok(w, map[string]bool{"saved": true})
}
