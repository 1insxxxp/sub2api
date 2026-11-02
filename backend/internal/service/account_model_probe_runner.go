package service

import (
	"context"

	"github.com/gin-gonic/gin"
)

// RunModelProbe reuses the account-management native/default test, including its
// media-model inference. It deliberately does not change scheduled-test parsing.
func (s *AccountTestService) RunModelProbe(ctx context.Context, accountID int64, model string) (ModelProbeExecution, error) {
	if result, done := modelProbeContextResult(ctx); done {
		return result, nil
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return ModelProbeExecution{Status: "failed", ErrorCode: "probe_unavailable"}, nil
	}
	if !modelProbeSupportsDefaultMode(account.GetMappedModel(model)) {
		return ModelProbeExecution{Status: "unsupported", ErrorCode: "probe_unsupported"}, nil
	}
	return runStrictModelProbe(ctx, func(c *gin.Context) error {
		return s.TestAccountConnection(c, accountID, model, "", AccountTestModeDefault)
	})
}
