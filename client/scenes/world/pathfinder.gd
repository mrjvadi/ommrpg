class_name Pathfinder
extends RefCounted
## A* on the tile grid with a binary-heap open set (O(n log n)), octile
## heuristic with 8-way moves that never cut corners, and "string pulling"
## smoothing so the character walks straight lines instead of staircases.

class Heap:
	var keys: Array[float] = []
	var vals: Array[Vector2i] = []

	func push(k: float, v: Vector2i) -> void:
		keys.append(k)
		vals.append(v)
		var i := keys.size() - 1
		while i > 0:
			var p := (i - 1) >> 1
			if keys[p] <= keys[i]:
				break
			_swap(i, p)
			i = p

	func pop() -> Vector2i:
		var top := vals[0]
		var last := keys.size() - 1
		_swap(0, last)
		keys.pop_back()
		vals.pop_back()
		var i := 0
		var n := keys.size()
		while true:
			var l := i * 2 + 1
			var r := l + 1
			var m := i
			if l < n and keys[l] < keys[m]:
				m = l
			if r < n and keys[r] < keys[m]:
				m = r
			if m == i:
				break
			_swap(i, m)
			i = m
		return top

	func _swap(a: int, b: int) -> void:
		var k := keys[a]
		keys[a] = keys[b]
		keys[b] = k
		var v := vals[a]
		vals[a] = vals[b]
		vals[b] = v

	func empty() -> bool:
		return keys.is_empty()

const DIRS := [Vector2i(1, 0), Vector2i(-1, 0), Vector2i(0, 1), Vector2i(0, -1), Vector2i(1, 1), Vector2i(1, -1), Vector2i(-1, 1), Vector2i(-1, -1)]

static func _h(a: Vector2i, b: Vector2i) -> float:
	var dx := absi(a.x - b.x)
	var dy := absi(a.y - b.y)
	return float(maxi(dx, dy)) + 0.41421356 * float(mini(dx, dy))

## Returns tile centres from start (exclusive) to the goal, or [] if none.
## `adjacent` stops next to the goal (for monsters / objects).
static func find(start: Vector2i, goal: Vector2i, adjacent: bool, walkable: Callable, limit := 6000) -> Array:
	var open := Heap.new()
	var came := {start: start}
	var g := {start: 0.0}
	open.push(_h(start, goal), start)
	var found := Vector2i(-99999, -99999)
	while not open.empty() and limit > 0:
		limit -= 1
		var cur: Vector2i = open.pop()
		var d := maxi(absi(cur.x - goal.x), absi(cur.y - goal.y))
		if (adjacent and d <= 1 and cur != goal) or (not adjacent and cur == goal) or (adjacent and cur == goal and walkable.call(goal)):
			found = cur
			break
		for dir in DIRS:
			var nx: Vector2i = cur + dir
			if not walkable.call(nx):
				continue
			if dir.x != 0 and dir.y != 0 and (not walkable.call(Vector2i(cur.x + dir.x, cur.y)) or not walkable.call(Vector2i(cur.x, cur.y + dir.y))):
				continue # no corner cutting
			var ng: float = g[cur] + (1.41421356 if dir.x != 0 and dir.y != 0 else 1.0)
			if g.has(nx) and ng >= g[nx]:
				continue
			g[nx] = ng
			came[nx] = cur
			open.push(ng + _h(nx, goal), nx)
	if found.x == -99999:
		return []
	var tiles: Array = []
	var n := found
	while n != start:
		tiles.push_front(n)
		n = came[n]
	return smooth(start, tiles, walkable)

## String pulling: skip waypoints while the straight segment stays walkable.
static func smooth(start: Vector2i, tiles: Array, walkable: Callable) -> Array:
	var out: Array = []
	var anchor := Vector2(start) + Vector2(0.5, 0.5)
	var i := 0
	while i < tiles.size():
		var j := tiles.size() - 1
		while j > i and not line_clear(anchor, Vector2(tiles[j]) + Vector2(0.5, 0.5), walkable):
			j -= 1
		var p := Vector2(tiles[j]) + Vector2(0.5, 0.5)
		out.append(p)
		anchor = p
		i = j + 1
	return out

static func line_clear(a: Vector2, b: Vector2, walkable: Callable) -> bool:
	var steps := int(a.distance_to(b) / 0.25) + 1
	for s in range(1, steps + 1):
		var p := a.lerp(b, float(s) / steps)
		# check a small footprint so smoothed lines don't graze obstacles
		for o in [Vector2(0.2, 0.2), Vector2(-0.2, 0.2), Vector2(0.2, -0.2), Vector2(-0.2, -0.2)]:
			var q: Vector2 = p + o
			if not walkable.call(Vector2i(floori(q.x), floori(q.y))):
				return false
	return true
