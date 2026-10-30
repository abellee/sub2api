package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const userMenuConfigKey = "user_menu"

// UserMenuSection groups existing user navigation entries; it never grants access.
type UserMenuSection struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Items []string `json:"items"`
}

type UserMenuConfig struct {
	Sections    []UserMenuSection `json:"sections"`
	HiddenItems []string          `json:"hidden_items,omitempty"`
}

var userMenuIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var userMenuCustomPathPattern = regexp.MustCompile(`^/custom/[a-zA-Z0-9_-]{1,128}$`)

var userMenuBuiltinPaths = map[string]bool{
	"/dashboard": true, "/keys": true, "/usage": true,
	"/available-channels": true, "/monitor": true, "/subscriptions": true,
	"/purchase": true, "/orders": true, "/lottery": true, "/tasks": true,
	"/affiliate": true, "/profile": true, "/model-plaza": true, "/redeem": true,
	"/docs/creative": true,
}

func normalizeUserMenu(input UserMenuConfig) (*UserMenuConfig, error) {
	invalid := func(message string) (*UserMenuConfig, error) {
		return nil, infraerrors.BadRequest("INVALID_USER_MENU", message)
	}
	if len(input.Sections) > 32 {
		return invalid("At most 32 menu headings are allowed")
	}
	if len(input.HiddenItems) > 128 {
		return invalid("At most 128 hidden menu items are allowed")
	}
	result := &UserMenuConfig{Sections: make([]UserMenuSection, 0, len(input.Sections))}
	hidden := map[string]bool{}
	for _, item := range input.HiddenItems {
		path := strings.TrimSpace(item)
		if !userMenuBuiltinPaths[path] && !userMenuCustomPathPattern.MatchString(path) {
			return invalid(fmt.Sprintf("Unsupported hidden user menu item: %s", path))
		}
		if hidden[path] {
			return invalid("Hidden menu items must be unique")
		}
		hidden[path] = true
		result.HiddenItems = append(result.HiddenItems, path)
	}
	ids, titles, paths := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, section := range input.Sections {
		id, title := strings.TrimSpace(section.ID), strings.TrimSpace(section.Title)
		if !userMenuIDPattern.MatchString(id) || id == "__unassigned__" || ids[id] {
			return invalid("Menu heading IDs must be unique and valid")
		}
		if title == "" || len([]rune(title)) > 80 || titles[title] {
			return invalid("Menu headings must have unique titles between 1 and 80 characters")
		}
		if len(section.Items) > 128 {
			return invalid("At most 128 menu items are allowed per heading")
		}
		ids[id], titles[title] = true, true
		items := make([]string, 0, len(section.Items))
		for _, item := range section.Items {
			path := strings.TrimSpace(item)
			if !userMenuBuiltinPaths[path] && !userMenuCustomPathPattern.MatchString(path) {
				return invalid(fmt.Sprintf("Unsupported user menu item: %s", path))
			}
			if paths[path] {
				return invalid("A menu item can belong to only one heading")
			}
			paths[path] = true
			items = append(items, path)
		}
		result.Sections = append(result.Sections, UserMenuSection{ID: id, Title: title, Items: items})
	}
	return result, nil
}

func (s *GroupCategoryStore) GetUserMenu(ctx context.Context) (*UserMenuConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var config UserMenuConfig
	if err := s.readConfig(ctx, userMenuConfigKey, &config); err != nil {
		return nil, fmt.Errorf("read user menu: %w", err)
	}
	return normalizeUserMenu(config)
}

func (s *GroupCategoryStore) SaveUserMenu(ctx context.Context, input UserMenuConfig) (*UserMenuConfig, error) {
	config, err := normalizeUserMenu(input)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writeConfig(ctx, userMenuConfigKey, config); err != nil {
		return nil, err
	}
	return config, nil
}
