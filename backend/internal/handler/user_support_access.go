package handler

import "context"

type userSupportLoginAccess interface {
	CanAccess(context.Context, int64) (bool, error)
}

// CanAccessUserSupport permits identity introspection for explicitly authorized
// staff in backend mode. It never grants the general manager or user API surface.
func (h *AuthHandler) CanAccessUserSupport(ctx context.Context, userID int64) (bool, error) {
	if h == nil || h.userSupport == nil || userID <= 0 {
		return false, nil
	}
	return h.userSupport.CanAccess(ctx, userID)
}
