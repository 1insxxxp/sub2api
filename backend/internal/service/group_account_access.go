package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

type groupAccountAccessContextKey struct{}
type groupAccountAccessScopeContextKey struct{}

type groupAccountAccessPolicy struct {
	userID     int64
	exclusions GroupAccountExclusions
}

// WithGroupAccountExclusions binds a private authorization snapshot supplied by
// authentication. Routing metadata must never be used to choose the user.
func WithGroupAccountExclusions(ctx context.Context, userID int64, exclusions GroupAccountExclusions) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, groupAccountAccessContextKey{}, groupAccountAccessPolicy{
		userID: userID, exclusions: cloneGroupAccountExclusions(exclusions),
	})
}

// IsGroupAccountAllowedForUser applies only the rule for the actual scheduling
// group, not the billing container or another group attached to the account.
func IsGroupAccountAllowedForUser(ctx context.Context, groupID *int64, accountID int64) bool {
	if ctx == nil || groupID == nil || *groupID <= 0 || accountID <= 0 {
		return true
	}
	policy, ok := ctx.Value(groupAccountAccessContextKey{}).(groupAccountAccessPolicy)
	if !ok || policy.userID <= 0 {
		return true
	}
	for _, excluded := range policy.exclusions[*groupID] {
		if excluded == accountID {
			return false
		}
	}
	return true
}

// A selector pins its resolved group before any sticky/get/hydration path.
// Storing a value also distinguishes an explicitly ungrouped selection from
// an authentication context that still contains its original billing group.
func withGroupAccountAccessScope(ctx context.Context, groupID *int64) context.Context {
	return context.WithValue(ctx, groupAccountAccessScopeContextKey{}, derefGroupID(groupID))
}

func groupAccountAllowedInSchedulingContext(ctx context.Context, accountID int64) bool {
	if ctx == nil {
		return true
	}
	if groupID, ok := ctx.Value(groupAccountAccessScopeContextKey{}).(int64); ok {
		return IsGroupAccountAllowedForUser(ctx, &groupID, accountID)
	}
	if resolution, ok := SystemCustomGroupResolutionFromContext(ctx); ok {
		return IsGroupAccountAllowedForUser(ctx, &resolution.SourceGroupID, accountID)
	}
	if group, ok := ctx.Value(ctxkey.Group).(*Group); ok && group != nil {
		return IsGroupAccountAllowedForUser(ctx, &group.ID, accountID)
	}
	return true
}

func filterGroupAccountAllowedAccounts(ctx context.Context, groupID *int64, accounts []Account) []Account {
	if ctx == nil || groupID == nil {
		return accounts
	}
	policy, ok := ctx.Value(groupAccountAccessContextKey{}).(groupAccountAccessPolicy)
	if !ok || policy.userID <= 0 || len(policy.exclusions[*groupID]) == 0 {
		return accounts
	}
	filtered := make([]Account, 0, len(accounts))
	for i := range accounts {
		if IsGroupAccountAllowedForUser(ctx, groupID, accounts[i].ID) {
			filtered = append(filtered, accounts[i])
		}
	}
	return filtered
}
