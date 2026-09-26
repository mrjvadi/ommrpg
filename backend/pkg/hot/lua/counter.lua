-- Sliding-window event counter with 1-second buckets in one hash.
-- Increments the current bucket and returns the sum over the window,
-- deleting buckets that fell out of it (bounded memory).
--   KEYS[1] counter hash, ARGV[1] increment, ARGV[2] window seconds
-- returns {sum_over_window, current_second_count}
local t = redis.call('TIME')
local now = tonumber(t[1])
local win = tonumber(ARGV[2])
local inc = tonumber(ARGV[1])
local cur = 0
if inc ~= 0 then cur = redis.call('HINCRBY', KEYS[1], now, inc) end
local all = redis.call('HGETALL', KEYS[1])
local sum = 0
for i = 1, #all, 2 do
  local sec = tonumber(all[i])
  if sec <= now - win then
    redis.call('HDEL', KEYS[1], all[i])
  else
    sum = sum + tonumber(all[i + 1])
  end
end
redis.call('EXPIRE', KEYS[1], win * 2)
return {sum, cur}
