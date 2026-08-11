package groupref

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// PostgreSQL's two-int advisory-lock keyspace is independent from its bigint
// keyspace. Keep group-reference serialization in a dedicated namespace.
const groupReferenceLockNamespace int32 = 0x53554232

// LockGroupReferenceWrites serializes transactions that create effective
// references to the same group. A transaction is required because the lock is
// deliberately released by PostgreSQL only when that transaction completes.
func LockGroupReferenceWrites(ctx context.Context, tx *dbent.Tx, groupIDs ...int64) error {
	if tx == nil {
		return errors.New("group reference advisory lock requires an explicit transaction")
	}
	client := tx.Client()
	if client.Driver().Dialect() != dialect.Postgres {
		return nil
	}
	for _, groupID := range sortedUniquePositiveGroupIDs(groupIDs) {
		rows, err := client.QueryContext(
			ctx,
			"SELECT pg_advisory_xact_lock($1, $2)",
			groupReferenceLockNamespace,
			groupReferenceLockKey(groupID),
		)
		if err != nil {
			return fmt.Errorf("lock group reference writes for group %d: %w", groupID, err)
		}
		for rows.Next() {
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("read group reference advisory lock for group %d: %w", groupID, err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close group reference advisory lock for group %d: %w", groupID, err)
		}
	}
	return nil
}

func sortedUniquePositiveGroupIDs(groupIDs []int64) []int64 {
	seen := make(map[int64]struct{}, len(groupIDs))
	normalized := make([]int64, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupID <= 0 {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		normalized = append(normalized, groupID)
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i] < normalized[j] })
	return normalized
}

func groupReferenceLockKey(groupID int64) int32 {
	unsigned := uint64(groupID)
	return int32(uint32(unsigned) ^ uint32(unsigned>>32))
}
