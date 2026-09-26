-- Places a character unconditionally (enter world, teleport, respawn) and
-- keeps the area index consistent. Resets the movement bucket.
--   KEYS[1] pos hash, KEYS[2] old area zset ("" when none), KEYS[3] new area zset
--   ARGV: zone, x, y, dir, anim, cap, ttl_s, member
-- returns new version
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local ver = tonumber(redis.call('HGET', KEYS[1], 'ver') or '0') + 1
redis.call('HSET', KEYS[1], 'zone', ARGV[1], 'x', ARGV[2], 'y', ARGV[3], 'dir', ARGV[4], 'anim', ARGV[5],
  'at', now, 'budget', ARGV[6], 'ver', ver, 'seq', 0)
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[7]))
if KEYS[2] ~= KEYS[3] then redis.call('ZREM', KEYS[2], ARGV[8]) end
redis.call('ZADD', KEYS[3], now, ARGV[8])
redis.call('EXPIRE', KEYS[3], tonumber(ARGV[7]))
return ver
