-- One attack on a monster, atomically:
--  * attacker cooldown (so a fast client cannot out-shoot its weapon)
--  * lazy respawn (a monster whose death timer passed is back at full HP)
--  * damage + threat table (damage dealt per attacker, capped by remaining
--    HP so overkill earns nothing)
--  * exactly-once death: only the killing call sees state 1, and receives
--    the whole threat table for fair XP split and personal loot.
--   KEYS[1] mon hash, KEYS[2] threat hash, KEYS[3] attacker cooldown key
--   ARGV: max_hp, damage, respawn_ms (0 = never), ttl_s, attacker, cooldown_ms
-- returns {state, hp, dead_until, [attacker, damage]...}
--   state 0 hit, 1 killed, 2 already dead, 3 on cooldown (hp field = ms left)
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local left = redis.call('PTTL', KEYS[3])
if left > 0 then return {3, left, 0} end
local maxhp, dmg = tonumber(ARGV[1]), tonumber(ARGV[2])
local s = redis.call('HMGET', KEYS[1], 'hp', 'dead_until')
local dead = tonumber(s[2] or '0')
if dead == -1 or dead > now then return {2, 0, dead} end
redis.call('SET', KEYS[3], 1, 'PX', tonumber(ARGV[6]))
local hp = tonumber(s[1] or '-1')
if hp < 0 or dead ~= 0 then
  hp = maxhp
  redis.call('DEL', KEYS[2]) -- a respawned monster starts with a clean threat table
end
local dealt = math.min(hp, dmg)
hp = hp - dmg
redis.call('HINCRBY', KEYS[2], ARGV[5], dealt)
redis.call('EXPIRE', KEYS[2], tonumber(ARGV[4]))
if hp <= 0 then
  local untilv = -1
  if tonumber(ARGV[3]) > 0 then untilv = now + tonumber(ARGV[3]) end
  redis.call('HSET', KEYS[1], 'hp', maxhp, 'dead_until', untilv)
  redis.call('EXPIRE', KEYS[1], tonumber(ARGV[4]))
  local threat = redis.call('HGETALL', KEYS[2])
  redis.call('DEL', KEYS[2])
  local out = {1, 0, untilv}
  for i = 1, #threat do table.insert(out, threat[i]) end
  return out
end
redis.call('HSET', KEYS[1], 'hp', hp, 'dead_until', 0)
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[4]))
return {0, hp, 0}
