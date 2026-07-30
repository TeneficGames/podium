package redis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	tieBreakSequenceWidth = 19
	maxTieBreakSequence   = "9223372036854775807"
	maxExactRedisScore    = float64(9_007_199_254_740_991)
)

var upsertMembersWithTieBreakScript = goredis.NewScript(`
local function next_sequence()
  redis.call("DECR", KEYS[4])
  local current = redis.call("GET", KEYS[4])
  return string.rep("0", 19 - string.len(current)) .. current
end

local function ascending_internal(internal, member)
  local inverted = {}
  for i = 1, 19 do
    inverted[i] = tostring(9 - tonumber(string.sub(internal, i, i)))
  end
  return table.concat(inverted) .. member
end

local member_count = (#ARGV - 2) / 2
local current_sequence = redis.call("GET", KEYS[4])
if member_count > 0 and not current_sequence then
  redis.call("SET", KEYS[4], ARGV[1])
  current_sequence = ARGV[1]
end
if current_sequence and tonumber(current_sequence) < member_count then
  local required_sequences = 0
  for i = 3, #ARGV, 2 do
    local internal = redis.call("HGET", KEYS[3], ARGV[i])
    local old_score = nil
    if internal then
      old_score = redis.call("ZSCORE", KEYS[1], internal)
    end
    if not old_score or tonumber(old_score) ~= tonumber(ARGV[i + 1]) then
      required_sequences = required_sequences + 1
    end
  end
  if tonumber(current_sequence) < required_sequences then
    return redis.error_reply("tie-break sequence exhausted")
  end
end

local internals = {}
for i = 3, #ARGV, 2 do
  local member = ARGV[i]
  local score = ARGV[i + 1]
  local internal = redis.call("HGET", KEYS[3], member)
  local unchanged = false
  if internal then
    local old_score = redis.call("ZSCORE", KEYS[1], internal)
    if old_score and tonumber(old_score) == tonumber(score) then
      unchanged = true
    end
  end

  if not unchanged then
    local sequence = next_sequence()
    local replacement = sequence .. member
    if internal then
      redis.call("ZREM", KEYS[1], internal)
      redis.call("ZREM", KEYS[2], ascending_internal(internal, member))
    end
    redis.call("ZADD", KEYS[1], score, replacement)
    redis.call("ZADD", KEYS[2], score, ascending_internal(replacement, member))
    redis.call("HSET", KEYS[3], member, replacement)
    internal = replacement
  end
  internals[#internals + 1] = internal
end

local ranks = {}
local order = ARGV[2]
for _, internal in ipairs(internals) do
  if order == "asc" then
    local member = ARGV[2 * #ranks + 3]
    ranks[#ranks + 1] = redis.call("ZRANK", KEYS[2], ascending_internal(internal, member))
  else
    ranks[#ranks + 1] = redis.call("ZREVRANK", KEYS[1], internal)
  end
end
return ranks
`)

var incrementWithTieBreakScript = goredis.NewScript(`
local function ascending_internal(internal, member)
  local inverted = {}
  for i = 1, 19 do
    inverted[i] = tostring(9 - tonumber(string.sub(internal, i, i)))
  end
  return table.concat(inverted) .. member
end

local member = ARGV[1]
local increment = tonumber(ARGV[2])
local order = ARGV[3]
local internal = redis.call("HGET", KEYS[3], member)
local old_score = nil
if internal then
  old_score = redis.call("ZSCORE", KEYS[1], internal)
end
local new_score = tonumber(old_score or "0") + increment
local max_score = tonumber(ARGV[5])
if new_score ~= new_score or new_score > max_score or new_score < -max_score then
  return redis.error_reply("score is outside Redis exact integer range")
end

if old_score and increment == 0 then
  local rank
  if order == "asc" then
    rank = redis.call("ZRANK", KEYS[2], ascending_internal(internal, member))
  else
    rank = redis.call("ZREVRANK", KEYS[1], internal)
  end
  return {old_score, rank}
end

local current = redis.call("GET", KEYS[4])
if not current then
  redis.call("SET", KEYS[4], ARGV[4])
elseif current == "0" then
  return redis.error_reply("tie-break sequence exhausted")
end
redis.call("DECR", KEYS[4])
current = redis.call("GET", KEYS[4])
local sequence = string.rep("0", 19 - string.len(current)) .. current
local replacement = sequence .. member

if internal then
  redis.call("ZREM", KEYS[1], internal)
  redis.call("ZREM", KEYS[2], ascending_internal(internal, member))
end
redis.call("ZADD", KEYS[1], new_score, replacement)
redis.call("ZADD", KEYS[2], new_score, ascending_internal(replacement, member))
redis.call("HSET", KEYS[3], member, replacement)

local rank
if order == "asc" then
  rank = redis.call("ZRANK", KEYS[2], ascending_internal(replacement, member))
else
  rank = redis.call("ZREVRANK", KEYS[1], replacement)
end
return {redis.call("ZSCORE", KEYS[1], replacement), rank}
`)

var getMembersWithTieBreakScript = goredis.NewScript(`
local function ascending_internal(internal, member)
  local inverted = {}
  for i = 1, 19 do
    inverted[i] = tostring(9 - tonumber(string.sub(internal, i, i)))
  end
  return table.concat(inverted) .. member
end

local result = {}
local order = ARGV[1]
local include_ttl = ARGV[2] == "1"
for i = 3, #ARGV do
  local member = ARGV[i]
  local internal = redis.call("HGET", KEYS[3], member)
  local score = false
  local rank = false
  local ttl = false
  if internal then
    local score_key = KEYS[1]
    local ranked_internal = internal
    if order == "asc" then
      score_key = KEYS[2]
      ranked_internal = ascending_internal(internal, member)
    end
    score = redis.call("ZSCORE", score_key, ranked_internal)
    if score then
      if order == "asc" then
        rank = redis.call("ZRANK", score_key, ranked_internal)
      else
        rank = redis.call("ZREVRANK", score_key, ranked_internal)
      end
      if include_ttl then
        ttl = redis.call("ZSCORE", KEYS[4], member)
      end
    end
  end
  result[#result + 1] = score
  result[#result + 1] = rank
  result[#result + 1] = ttl
end
return result
`)

var getRankWithTieBreakScript = goredis.NewScript(`
local function ascending_internal(internal, member)
  local inverted = {}
  for i = 1, 19 do
    inverted[i] = tostring(9 - tonumber(string.sub(internal, i, i)))
  end
  return table.concat(inverted) .. member
end

local member = ARGV[1]
local internal = redis.call("HGET", KEYS[3], member)
if not internal then
  return false
end
if ARGV[2] == "asc" then
  local ascending = ascending_internal(internal, member)
  if not redis.call("ZSCORE", KEYS[2], ascending) then
    return false
  end
  return redis.call("ZRANK", KEYS[2], ascending)
end
if not redis.call("ZSCORE", KEYS[1], internal) then
  return false
end
return redis.call("ZREVRANK", KEYS[1], internal)
`)

var removeMembersWithTieBreakScript = goredis.NewScript(`
local function ascending_internal(internal, member)
  local inverted = {}
  for i = 1, 19 do
    inverted[i] = tostring(9 - tonumber(string.sub(internal, i, i)))
  end
  return table.concat(inverted) .. member
end

for i = 1, #ARGV do
  local member = ARGV[i]
  local internal = redis.call("HGET", KEYS[3], member)
  if internal then
    redis.call("ZREM", KEYS[1], internal)
    redis.call("ZREM", KEYS[2], ascending_internal(internal, member))
    redis.call("HDEL", KEYS[3], member)
  end
  redis.call("ZREM", KEYS[4], member)
end
return 1
`)

var expireTieBreakKeysAtScript = goredis.NewScript(`
local found = redis.call("EXISTS", KEYS[1])
if found == 0 then
  return 0
end
for i = 1, #KEYS do
  if redis.call("EXISTS", KEYS[i]) == 1 then
    redis.call("EXPIREAT", KEYS[i], ARGV[1])
  end
end
return 1
`)

var setMembersTTLWithTieBreakScript = goredis.NewScript(`
for i = 1, #ARGV, 2 do
  redis.call("ZADD", KEYS[2], ARGV[i + 1], ARGV[i])
end
local ttl = redis.call("PTTL", KEYS[1])
if ttl > 0 then
  redis.call("PEXPIRE", KEYS[2], ttl)
end
return 1
`)

func upsertMembersWithTieBreak(
	ctx context.Context,
	client goredis.Scripter,
	keys TieBreakKeys,
	order string,
	members ...*Member,
) ([]int64, error) {
	if err := validateTieBreakMembers(order, members); err != nil {
		return nil, err
	}
	args := make([]any, 0, 2+2*len(members))
	args = append(args, maxTieBreakSequence, order)
	for _, member := range members {
		args = append(args, member.Member, strconv.FormatFloat(member.Score, 'f', -1, 64))
	}
	ranks, err := upsertMembersWithTieBreakScript.Run(
		ctx,
		client,
		[]string{keys.Scores, keys.ScoresAsc, keys.Members, keys.Sequence},
		args...,
	).Int64Slice()
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}
	return ranks, nil
}

func incrementWithTieBreak(
	ctx context.Context,
	client goredis.Scripter,
	keys TieBreakKeys,
	member, order string,
	increment float64,
) (*Member, error) {
	if err := validateTieBreakOrder(order); err != nil {
		return nil, err
	}
	if math.IsNaN(increment) || math.IsInf(increment, 0) || math.Abs(increment) > maxExactRedisScore {
		return nil, NewGeneralError("increment is outside Redis exact integer range")
	}
	values, err := incrementWithTieBreakScript.Run(
		ctx,
		client,
		[]string{keys.Scores, keys.ScoresAsc, keys.Members, keys.Sequence},
		member,
		strconv.FormatFloat(increment, 'f', -1, 64),
		order,
		maxTieBreakSequence,
		strconv.FormatFloat(maxExactRedisScore, 'f', -1, 64),
	).Slice()
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}
	if len(values) != 2 {
		return nil, NewGeneralError("invalid increment result")
	}
	score, err := parseRedisFloat(values[0])
	if err != nil {
		return nil, err
	}
	rank, err := parseRedisInt64(values[1])
	if err != nil {
		return nil, err
	}
	return &Member{Member: member, Score: score, Rank: rank}, nil
}

func getMembersWithTieBreak(
	ctx context.Context,
	client goredis.Scripter,
	keys TieBreakKeys,
	order string,
	includeTTL bool,
	members ...string,
) ([]*Member, error) {
	if err := validateTieBreakOrder(order); err != nil {
		return nil, err
	}
	args := make([]any, 0, 2+len(members))
	args = append(args, order)
	if includeTTL {
		args = append(args, "1")
	} else {
		args = append(args, "0")
	}
	for _, member := range members {
		args = append(args, member)
	}
	values, err := getMembersWithTieBreakScript.Run(
		ctx,
		client,
		[]string{keys.Scores, keys.ScoresAsc, keys.Members, keys.TTL},
		args...,
	).Slice()
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}
	if len(values) != 3*len(members) {
		return nil, NewGeneralError("invalid members result")
	}

	result := make([]*Member, len(members))
	for i, member := range members {
		scoreValue := values[3*i]
		if scoreValue == nil {
			continue
		}
		score, err := parseRedisFloat(scoreValue)
		if err != nil {
			return nil, err
		}
		rank, err := parseRedisInt64(values[3*i+1])
		if err != nil {
			return nil, err
		}
		result[i] = &Member{Member: member, Score: score, Rank: rank}
		if values[3*i+2] != nil {
			ttl, err := parseRedisFloat(values[3*i+2])
			if err != nil {
				return nil, err
			}
			result[i].TTL = time.Unix(int64(ttl), 0)
		}
	}
	return result, nil
}

func getRankWithTieBreak(
	ctx context.Context,
	client goredis.Scripter,
	keys TieBreakKeys,
	member, order string,
) (int64, error) {
	if err := validateTieBreakOrder(order); err != nil {
		return -1, err
	}
	rank, err := getRankWithTieBreakScript.Run(
		ctx,
		client,
		[]string{keys.Scores, keys.ScoresAsc, keys.Members},
		member,
		order,
	).Int64()
	if errors.Is(err, goredis.Nil) {
		return -1, NewMemberNotFoundError(keys.Scores, member)
	}
	if err != nil {
		return -1, NewGeneralError(err.Error())
	}
	return rank, nil
}

func removeMembersWithTieBreak(
	ctx context.Context,
	client goredis.Scripter,
	keys TieBreakKeys,
	members ...string,
) error {
	args := make([]any, len(members))
	for i, member := range members {
		args[i] = member
	}
	if err := removeMembersWithTieBreakScript.Run(
		ctx,
		client,
		[]string{keys.Scores, keys.ScoresAsc, keys.Members, keys.TTL},
		args...,
	).Err(); err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

func expireTieBreakKeysAt(
	ctx context.Context,
	client goredis.Scripter,
	keys TieBreakKeys,
	expireAt time.Time,
) error {
	found, err := expireTieBreakKeysAtScript.Run(
		ctx,
		client,
		[]string{keys.Scores, keys.ScoresAsc, keys.Members, keys.Sequence, keys.TTL},
		expireAt.Unix(),
	).Int64()
	if err != nil {
		return NewGeneralError(err.Error())
	}
	if found == 0 {
		return NewKeyNotFoundError(keys.Scores)
	}
	return nil
}

func setMembersTTLWithTieBreak(
	ctx context.Context,
	client goredis.Scripter,
	keys TieBreakKeys,
	members ...*Member,
) error {
	args := make([]any, 0, 2*len(members))
	for _, member := range members {
		args = append(args, member.Member, member.TTL.Unix())
	}
	if err := setMembersTTLWithTieBreakScript.Run(
		ctx,
		client,
		[]string{keys.Scores, keys.TTL},
		args...,
	).Err(); err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

func parseRedisFloat(value any) (float64, error) {
	switch value := value.(type) {
	case string:
		result, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, NewGeneralError(err.Error())
		}
		return result, nil
	case float64:
		return value, nil
	case int64:
		return float64(value), nil
	default:
		return 0, NewGeneralError(fmt.Sprintf("unexpected Redis number type %T", value))
	}
}

func parseRedisInt64(value any) (int64, error) {
	switch value := value.(type) {
	case int64:
		return value, nil
	case string:
		result, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, NewGeneralError(err.Error())
		}
		return result, nil
	default:
		return 0, NewGeneralError(fmt.Sprintf("unexpected Redis integer type %T", value))
	}
}

func validateTieBreakMembers(order string, members []*Member) error {
	if err := validateTieBreakOrder(order); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		if member == nil {
			return NewGeneralError("member is nil")
		}
		if math.IsNaN(member.Score) || math.IsInf(member.Score, 0) || math.Abs(member.Score) > maxExactRedisScore {
			return NewGeneralError(fmt.Sprintf("score for member %s is outside Redis exact integer range", member.Member))
		}
		if _, exists := seen[member.Member]; exists {
			return NewGeneralError(fmt.Sprintf("duplicate member %s", member.Member))
		}
		seen[member.Member] = struct{}{}
	}
	return nil
}

func validateTieBreakOrder(order string) error {
	if order != "asc" && order != "desc" {
		return NewGeneralError(fmt.Sprintf("invalid order %s", order))
	}
	return nil
}
