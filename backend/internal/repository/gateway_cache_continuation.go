package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"time"
)

func continuationAliasCacheKey(groupID, userID int64, public string) string {
	digest := sha256.Sum256([]byte(public))
	return fmt.Sprintf("openai:continuation-alias:%d:%d:%s", groupID, userID, hex.EncodeToString(digest[:]))
}
func (c *gatewayCache) SetOpenAIContinuationAlias(ctx context.Context, groupID, userID int64, public string, b service.OpenAIContinuationAlias, ttl time.Duration) error {
	if userID <= 0 || public == "" || b.ResponseID == "" || b.AccountID <= 0 || ttl <= 0 {
		return errors.New("invalid continuation alias")
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, continuationAliasCacheKey(groupID, userID, public), raw, ttl).Err()
}
func (c *gatewayCache) GetOpenAIContinuationAlias(ctx context.Context, groupID, userID int64, public string) (service.OpenAIContinuationAlias, bool, error) {
	var b service.OpenAIContinuationAlias
	raw, err := c.rdb.Get(ctx, continuationAliasCacheKey(groupID, userID, public)).Bytes()
	if errors.Is(err, redis.Nil) {
		return b, false, nil
	}
	if err != nil {
		return b, false, err
	}
	if err = json.Unmarshal(raw, &b); err != nil {
		return b, false, err
	}
	if b.ResponseID == "" || b.AccountID <= 0 {
		return b, false, errors.New("invalid continuation alias")
	}
	return b, true, nil
}
