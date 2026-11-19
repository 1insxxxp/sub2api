package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	modelStatusSnapshotKey = "model-status:{report-v1}:snapshot"
	modelStatusRefreshKey  = "model-status:{report-v1}:refresh"
	modelStatusSnapshotTTL = 24 * time.Hour
)

var finishModelStatusRefreshScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
	return 0
end
if ARGV[2] ~= "" then
	redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
end
redis.call("SET", KEYS[1], "cooldown", "PX", ARGV[4])
return 1
`)

type modelStatusSnapshotCache struct {
	client *redis.Client
}

func NewModelStatusSnapshotCache(client *redis.Client) service.ModelStatusSnapshotCache {
	if client == nil {
		return nil
	}
	return &modelStatusSnapshotCache{client: client}
}

func (c *modelStatusSnapshotCache) Load(ctx context.Context) (*service.ModelStatusReport, error) {
	data, err := c.client.Get(ctx, modelStatusSnapshotKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var report service.ModelStatusReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func (c *modelStatusSnapshotCache) TryRefresh(ctx context.Context, owner string, lease time.Duration) (bool, error) {
	return c.client.SetNX(ctx, modelStatusRefreshKey, owner, lease).Result()
}

func (c *modelStatusSnapshotCache) FinishRefresh(ctx context.Context, owner string, report *service.ModelStatusReport, cooldown time.Duration) error {
	var data []byte
	if report != nil {
		var err error
		data, err = json.Marshal(report)
		if err != nil {
			return err
		}
	}
	return finishModelStatusRefreshScript.Run(ctx, c.client, []string{modelStatusRefreshKey, modelStatusSnapshotKey},
		owner, string(data), modelStatusSnapshotTTL.Milliseconds(), cooldown.Milliseconds()).Err()
}
