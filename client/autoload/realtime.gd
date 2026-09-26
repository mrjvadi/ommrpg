extends Node
## Centrifugo client (JSON protocol v2 over WebSocket) written for Godot.
##
## - connect_to(url, token): connects and authenticates with a JWT
## - call_rpc(method, data): awaitable RPC through the gateway's RPC proxy
## - subscribe(channel) / unsubscribe(channel)
## - signal publication(channel, data) for every push (including
##   server-side subscriptions such as the personal channel)
## It reconnects automatically with backoff and re-subscribes.

signal connected
signal disconnected(reason: String)
signal publication(channel: String, data: Dictionary)

class Pending:
	signal done(result: Dictionary)
	var deadline := 0.0

const RPC_TIMEOUT := 6.0

var _ws := WebSocketPeer.new()
var _url := ""
var _token := ""
var _next_id := 1
var _pending := {} # id -> Pending
var _connect_id := 0
var _subs := {} # channel -> true (desired client-side subscriptions)
var _state := "idle" # idle | connecting | connected | waiting
var _retry_at := 0.0
var _backoff := 0.5
var is_connected_now := false

func connect_to(url: String, token: String) -> void:
	_url = url
	_token = token
	_open()

func close() -> void:
	_state = "idle"
	_url = ""
	_ws.close()
	is_connected_now = false

func _open() -> void:
	_ws = WebSocketPeer.new()
	_connect_id = 0
	_ws.inbound_buffer_size = 1 << 20
	var err := _ws.connect_to_url(_url)
	if err != OK:
		_schedule_retry("connect error %d" % err)
		return
	_state = "connecting"

func _schedule_retry(reason: String) -> void:
	if is_connected_now:
		is_connected_now = false
		disconnected.emit(reason)
	_fail_pending(reason)
	if _url == "":
		_state = "idle"
		return
	_state = "waiting"
	_retry_at = Time.get_ticks_msec() / 1000.0 + _backoff
	_backoff = min(_backoff * 2.0, 10.0)

func _fail_pending(reason: String) -> void:
	for id in _pending.keys():
		var p: Pending = _pending[id]
		p.done.emit({"ok": false, "error": reason})
	_pending.clear()

func _process(_delta: float) -> void:
	var now := Time.get_ticks_msec() / 1000.0
	if _state == "waiting" and now >= _retry_at:
		_open()
	if _state == "idle" or _state == "waiting":
		return
	_ws.poll()
	match _ws.get_ready_state():
		WebSocketPeer.STATE_OPEN:
			if _state == "connecting" and _connect_id == 0:
				_connect_id = _send_cmd({"connect": {"token": _token, "name": "godot"}})
			while _ws.get_available_packet_count() > 0:
				_handle_frame(_ws.get_packet().get_string_from_utf8())
		WebSocketPeer.STATE_CLOSED:
			_connect_id = 0
			_schedule_retry("closed (%d)" % _ws.get_close_code())
			return
	for id in _pending.keys():
		var p: Pending = _pending[id]
		if now > p.deadline:
			_pending.erase(id)
			p.done.emit({"ok": false, "error": "timeout"})

func _send_cmd(cmd: Dictionary) -> int:
	var id := _next_id
	_next_id += 1
	cmd["id"] = id
	_ws.send_text(JSON.stringify(cmd))
	return id

func _handle_frame(text: String) -> void:
	for line in text.split("\n", false):
		var msg = JSON.parse_string(line)
		if not msg is Dictionary:
			continue
		if msg.is_empty():
			_ws.send_text("{}") # server ping -> pong
			continue
		if msg.has("push"):
			_handle_push(msg.push)
			continue
		var id := int(msg.get("id", 0))
		if id == _connect_id and _connect_id != 0:
			_connect_id = -1
			if msg.has("error"):
				push_warning("realtime connect rejected: %s" % str(msg.error))
				_url = "" if int(msg.error.get("code", 0)) in [101, 109] else _url
				_ws.close()
				return
			_state = "connected"
			_backoff = 0.5
			is_connected_now = true
			for ch in _subs.keys():
				_send_cmd({"subscribe": {"channel": ch}})
			connected.emit()
			continue
		if _pending.has(id):
			var p: Pending = _pending[id]
			_pending.erase(id)
			if msg.has("error"):
				p.done.emit({"ok": false, "error": str(msg.error.get("message", "error")), "code": int(msg.error.get("code", 0))})
			elif msg.has("rpc"):
				p.done.emit({"ok": true, "data": msg.rpc.get("data", {})})
			else:
				p.done.emit({"ok": true, "data": {}})

func _handle_push(push: Dictionary) -> void:
	var ch := str(push.get("channel", ""))
	if push.has("pub"):
		var data = push.pub.get("data", {})
		if data is Dictionary:
			publication.emit(ch, data)

## Awaitable RPC. Returns { ok, data } or { ok: false, error }.
func call_rpc(method: String, data := {}) -> Dictionary:
	if _state != "connected":
		return {"ok": false, "error": "not connected"}
	var p := Pending.new()
	p.deadline = Time.get_ticks_msec() / 1000.0 + RPC_TIMEOUT
	var id := _send_cmd({"rpc": {"method": method, "data": data}})
	_pending[id] = p
	var res: Dictionary = await p.done
	return res

func subscribe(channel: String) -> void:
	if _subs.has(channel):
		return
	_subs[channel] = true
	if _state == "connected":
		_send_cmd({"subscribe": {"channel": channel}})

func unsubscribe(channel: String) -> void:
	if not _subs.has(channel):
		return
	_subs.erase(channel)
	if _state == "connected":
		_send_cmd({"unsubscribe": {"channel": channel}})

func subscriptions() -> Array:
	return _subs.keys()
