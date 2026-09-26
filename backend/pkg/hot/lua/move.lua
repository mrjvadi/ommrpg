-- Authoritative movement with a distance token bucket and optimistic
-- concurrency. The caller has already checked the path for collisions from
-- the position it read (version ARGV[1]); if anything moved the character
-- in between, the version differs and the move is retried.
--
-- The bucket refills at speed*slack tiles per second up to `cap` tiles, so
-- network jitter (bunched packets) is tolerated while sustained speed
-- hacks are impossible: total distance over any window <= speed*t + cap.
--
--   KEYS[1] pos:<id> (hash)  KEYS[2] old area zset  KEYS[3] new area zset
--   ARGV: expected_ver, x, y, dir, anim, speed, cap, seq, ttl_s, member
-- returns {status, ver, x, y}
--   status 1 accepted, 0 rejected (too fast), -1 version conflict, -2 stale seq
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local p = redis.call('HMGET', KEYS[1], 'ver', 'x', 'y', 'at', 'budget', 'seq')
local ver = tonumber(p[1] or '0')
if ver ~= tonumber(ARGV[1]) then return {-1, ver, p[2] or '0', p[3] or '0'} end
local seq = tonumber(ARGV[8])
if seq > 0 and seq <= tonumber(p[6] or '0') then return {-2, ver, p[2] or '0', p[3] or '0'} end
local x0, y0 = tonumber(p[2]), tonumber(p[3])
local speed, cap = tonumber(ARGV[6]), tonumber(ARGV[7])
local at = tonumber(p[4] or now)
local budget = tonumber(p[5] or cap)
budget = math.min(cap, budget + speed * math.max(0, now - at) / 1000)
local x, y = tonumber(ARGV[2]), tonumber(ARGV[3])
local dist = math.sqrt((x - x0) ^ 2 + (y - y0) ^ 2)
if dist > budget + 0.05 then
  redis.call('HSET', KEYS[1], 'at', now, 'budget', budget)
  return {0, ver, tostring(x0), tostring(y0)}
end
ver = ver + 1
redis.call('HSET', KEYS[1], 'x', ARGV[2], 'y', ARGV[3], 'dir', ARGV[4], 'anim', ARGV[5],
  'at', now, 'budget', budget - dist, 'ver', ver, 'seq', math.max(seq, tonumber(p[6] or '0')))
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[9]))
if KEYS[2] ~= KEYS[3] then redis.call('ZREM', KEYS[2], ARGV[10]) end
redis.call('ZADD', KEYS[3], now, ARGV[10])
redis.call('EXPIRE', KEYS[3], tonumber(ARGV[9]))
return {1, ver, ARGV[2], ARGV[3]}
