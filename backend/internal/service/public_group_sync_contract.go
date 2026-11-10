package service

// PublicGroupSyncSnapshotVersion is the wire version for public group snapshots.
// Version 2 prevents older receivers from applying inherited source prices to
// their global billing catalog. Deploy a v1/v2 receiver before this producer.
const PublicGroupSyncSnapshotVersion = 2

// PublicGroupSyncRequest carries a public group's configuration between services.
type PublicGroupSyncRequest struct {
	EventID       string                          `json:"event_id"`
	Version       int                             `json:"version"`
	Revision      int64                           `json:"revision"`
	GroupID       int64                           `json:"group_id"`
	SortOrder     int                             `json:"sort_order"`
	GroupName     string                          `json:"group_name"`
	PublicEnabled bool                            `json:"public_enabled"`
	GroupRatio    float64                         `json:"group_ratio"`
	BillingMode   string                          `json:"billing_mode"`
	Models        []string                        `json:"models"`
	ModelMapping  map[string]string               `json:"model_mapping"`
	ModelPricing  map[string]PublicGroupSyncModel `json:"model_pricing"`
	// DisplayPricing is an optional v2 response-only quote catalog. Receivers
	// must not use it to change relay billing, routing or the legacy catalog.
	DisplayPricing map[string]PublicGroupSyncModel `json:"display_pricing,omitempty"`
}

// PublicGroupSyncModel contains the public display name, upstream mapping and
// prices needed to reproduce a group's billing behavior.
type PublicGroupSyncModel struct {
	// Default entries inherit each published model's source price and rate.
	// Receivers must keep these prices scoped to the entry, not global models.
	SourceGroupID    int64    `json:"source_group_id,omitempty"`
	SourceModel      string   `json:"source_model,omitempty"`
	SourceGroupRatio *float64 `json:"source_group_ratio,omitempty"`
	Platform         string   `json:"platform"`
	DisplayName      string   `json:"display_name"`
	UpstreamModel    string   `json:"upstream_model"`
	BillingMode      string   `json:"billing_mode"`
	InputPrice       *float64 `json:"input_price,omitempty"`
	OutputPrice      *float64 `json:"output_price,omitempty"`
	ImageInputPrice  *float64 `json:"image_input_price,omitempty"`
	ImageOutputPrice *float64 `json:"image_output_price,omitempty"`
	PerRequestPrice  *float64 `json:"per_request_price,omitempty"`
	CacheWritePrice  *float64 `json:"cache_write_price,omitempty"`
	CacheReadPrice   *float64 `json:"cache_read_price,omitempty"`
}
