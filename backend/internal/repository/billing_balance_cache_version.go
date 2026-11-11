package repository

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const billingBalanceSequenceKey = "billing:balance:sequence"

func billingBalanceVersionKey(userID int64) string {
	return fmt.Sprintf("%s%d", billingBalanceVersionKeyPrefix, userID)
}

func billingBalanceCacheKeys(userID int64) []string {
	inflightKey, _ := billingInflightKeys(userID)
	return []string{billingBalanceKey(userID), billingBalanceVersionKey(userID), inflightKey, billingBalanceSequenceKey}
}

// Keys are wallet, generation, inflight metadata, and a persistent sequence.
// Generate versions inside Redis so a retried mutation cannot restore an old
// generation. Cache snapshots must expire before the holds protecting them.
const balanceCacheLua = `
	local function boundedTTL(ttl)
		local holdTTL = redis.call('PTTL', KEYS[3])
		if holdTTL >= 0 then
			return math.min(ttl, holdTTL)
		end
		return ttl
	end
	local function setVersion(ttl)
		redis.call('INCR', KEYS[4])
		local version = redis.call('GET', KEYS[4])
		ttl = boundedTTL(ttl)
		if ttl <= 0 then
			redis.call('DEL', KEYS[1], KEYS[2])
			return version
		end
		redis.call('SET', KEYS[2], version, 'PX', ttl)
		return version
	end
	local function setBalance(balance, ttl)
		ttl = boundedTTL(ttl)
		if ttl <= 0 then
			redis.call('DEL', KEYS[1], KEYS[2])
			return 0
		end
		redis.call('SET', KEYS[1], balance, 'PX', ttl)
		return 1
	end
`

var invalidateBalanceScript = redis.NewScript(balanceCacheLua + `
	setVersion(tonumber(ARGV[1]) * 1000)
	return redis.call('DEL', KEYS[1])
`)

var getBalanceVersionScript = redis.NewScript(balanceCacheLua + `
	local version = redis.call('GET', KEYS[2])
	if version == false then
		redis.call('DEL', KEYS[1])
		version = setVersion(tonumber(ARGV[1]))
	else
		local ttl = boundedTTL(tonumber(ARGV[1]))
		if ttl <= 0 then
			redis.call('DEL', KEYS[1], KEYS[2])
		else
			redis.call('PEXPIRE', KEYS[2], ttl)
		end
	end
	return version
`)

var setBalanceIfVersionScript = redis.NewScript(balanceCacheLua + `
	local version = redis.call('GET', KEYS[2])
	if version ~= ARGV[1] then
		return 0
	end
	return setBalance(ARGV[2], tonumber(ARGV[3]))
`)

var setBalanceScript = redis.NewScript(balanceCacheLua + `
	setVersion(tonumber(ARGV[3]))
	return setBalance(ARGV[1], tonumber(ARGV[2]))
`)

var constrainBalanceScript = redis.NewScript(balanceCacheLua + `
	setVersion(tonumber(ARGV[3]))
	local current = redis.call('GET', KEYS[1])
	if current == false then
		return 0
	end
	local balance = math.min(tonumber(current), tonumber(ARGV[1]))
	return setBalance(string.format('%.8f', balance), tonumber(ARGV[2]))
`)

func (c *billingCache) GetUserBalanceVersion(ctx context.Context, userID int64) (string, error) {
	return getBalanceVersionScript.Run(ctx, c.rdb, billingBalanceCacheKeys(userID),
		(2 * billingCacheTTL).Milliseconds()).Text()
}

func (c *billingCache) SetUserBalanceIfVersion(ctx context.Context, userID int64, balance float64, version string) (bool, error) {
	stored, err := setBalanceIfVersionScript.Run(ctx, c.rdb, billingBalanceCacheKeys(userID),
		version, strconv.FormatFloat(balance, 'f', -1, 64), jitteredTTL().Milliseconds()).Int64()
	return stored == 1, err
}

func (c *billingCache) ConstrainUserBalance(ctx context.Context, userID int64, balance float64) error {
	return constrainBalanceScript.Run(ctx, c.rdb, billingBalanceCacheKeys(userID),
		strconv.FormatFloat(balance, 'f', -1, 64), jitteredTTL().Milliseconds(), (2 * billingCacheTTL).Milliseconds()).Err()
}
