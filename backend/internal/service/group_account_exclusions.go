package service

// GroupAccountExclusions lists upstream account IDs unavailable to a user in each group.
// It is internal authorization state, intentionally absent from public DTOs.
type GroupAccountExclusions map[int64][]int64

func cloneGroupAccountExclusions(in GroupAccountExclusions) GroupAccountExclusions {
	if in == nil {
		return nil
	}
	out := make(GroupAccountExclusions, len(in))
	for groupID, accounts := range in {
		out[groupID] = append([]int64(nil), accounts...)
	}
	return out
}
