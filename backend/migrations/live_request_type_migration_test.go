package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLiveRequestTypeConstraintDefersExistingRowValidation(t *testing.T) {
	content, err := FS.ReadFile("188_allow_live_usage_request_type.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CHECK (request_type >= 0 AND request_type <= 5) NOT VALID")
}
