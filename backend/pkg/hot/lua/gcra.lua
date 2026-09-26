-- GCRA (Generic Cell Rate Algorithm) rate limiter.
-- One key per subject stores the "theoretical arrival time" (TAT) in ms.
-- Smooth (no fixed-window bursts at boundaries), O(1), one key.
--   KEYS[1] limiter key
--   ARGV[1] period ms, ARGV[2] limit per period, ARGV[3] burst, ARGV[4] cost
-- returns {allowed(0/1), retry_after_ms, remaining}
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local interval = tonumber(ARGV[1]) / tonumber(ARGV[2])
local burst = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])
local tat = tonumber(redis.call('GET', KEYS[1]) or now)
if tat < now then tat = now end
local new_tat = tat + interval * cost
local allow_at = new_tat - interval * burst
if allow_at > now then
  return {0, math.ceil(allow_at - now), 0}
end
redis.call('SET', KEYS[1], tostring(new_tat), 'PX', math.max(1, math.ceil(new_tat - now)))
return {1, 0, math.floor((now - allow_at) / interval)}
