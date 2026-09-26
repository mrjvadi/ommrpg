-- Player HP with lazy regeneration: stored as {hp, at}; the current value is
-- hp + regen * elapsed, computed on access, so no ticking process is needed.
--   KEYS[1] hp hash
--   ARGV[1] max_hp, ARGV[2] delta (negative = damage), ARGV[3] regen fraction/s, ARGV[4] ttl_s
-- returns {hp, died(0/1)}; on death the stored hp is reset to max (respawn)
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local maxhp = tonumber(ARGV[1])
local v = redis.call('HMGET', KEYS[1], 'hp', 'at')
local hp = tonumber(v[1] or maxhp)
local at = tonumber(v[2] or now)
hp = math.min(maxhp, hp + maxhp * tonumber(ARGV[3]) * math.max(0, now - at) / 1000)
hp = math.min(maxhp, hp + tonumber(ARGV[2]))
local died = 0
if hp <= 0 then died = 1; hp = maxhp end
redis.call('HSET', KEYS[1], 'hp', hp, 'at', now)
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[4]))
return {math.floor(hp), died}
