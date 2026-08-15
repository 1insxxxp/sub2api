//go:build unit

package admin

func newSystemCustomGroupHandlerForService(service systemCustomGroupAdminService) *SystemCustomGroupHandler {
	return &SystemCustomGroupHandler{service: service}
}
