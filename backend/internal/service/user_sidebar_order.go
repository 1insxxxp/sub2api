package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	maxUserSidebarOrderItems = 100
	maxUserSidebarOrderIDLen = 128
)

var userSidebarOrderIDPattern = regexp.MustCompile(`^/[A-Za-z0-9][A-Za-z0-9/_-]*$`)

// NormalizeUserSidebarOrder trims and de-duplicates the administrator's
// global sidebar identifiers while rejecting values that cannot be stable
// route-like identifiers. Unknown but well-formed identifiers are retained so
// newer frontend entries can be configured before all nodes are upgraded.
func NormalizeUserSidebarOrder(items []string) ([]string, error) {
	if len(items) > maxUserSidebarOrderItems {
		return nil, fmt.Errorf("too many user sidebar order items (max %d)", maxUserSidebarOrderItems)
	}

	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, raw := range items {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if len(id) > maxUserSidebarOrderIDLen || !userSidebarOrderIDPattern.MatchString(id) || strings.Contains(id, "..") {
			return nil, fmt.Errorf("invalid user sidebar order identifier")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

// ParseUserSidebarOrder is deliberately fail-closed for legacy or malformed
// settings: an invalid value means the frontend uses its built-in order.
func ParseUserSidebarOrder(raw string) []string {
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []string{}
	}
	normalized, err := NormalizeUserSidebarOrder(items)
	if err != nil {
		return []string{}
	}
	return normalized
}
