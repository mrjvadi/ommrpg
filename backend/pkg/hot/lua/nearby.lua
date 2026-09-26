--!df flags=allow-undeclared-keys
-- Area-of-interest query in one round trip: prunes idle members from each
-- area index and returns the live positions of everyone else in the zone.
--   KEYS  area zsets (3x3 around the caller)
--   ARGV[1] idle window ms, ARGV[2] caller id, ARGV[3] zone, ARGV[4] pos key prefix
-- returns flat {id, x, y, dir, anim, at, ...}
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local min = now - tonumber(ARGV[1])
local out = {}
for _, k in ipairs(KEYS) do
  redis.call('ZREMRANGEBYSCORE', k, '-inf', '(' .. min)
  local ids = redis.call('ZRANGEBYSCORE', k, min, '+inf')
  for _, id in ipairs(ids) do
    if id ~= ARGV[2] then
      local p = redis.call('HMGET', ARGV[4] .. id, 'zone', 'x', 'y', 'dir', 'anim', 'at')
      if p[1] == ARGV[3] then
        table.insert(out, id)
        table.insert(out, p[2]); table.insert(out, p[3])
        table.insert(out, p[4] or 'down'); table.insert(out, p[5] or 'idle'); table.insert(out, p[6] or '0')
      end
    end
  end
end
return out
