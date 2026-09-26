extends Node
## Resolves where the backend lives. In the browser everything is served from
## the same origin (nginx), so the API is "<origin>/api" and the realtime
## socket is "<ws-origin>/connection/websocket". On desktop builds and in the
## editor the defaults point at a local development stack; override them with
## command line args: --api=http://host:8080 --ws=ws://host:8000/connection/websocket

var api_base := "http://localhost:8080/api"
var ws_override := ""
var dev_username := ""
var autotest := false
var shot_dir := ""
## Draw the game rotated (as on a portrait phone) for testing: --rotate / ?rotate=1
var force_rotate := false

func _ready() -> void:
	if OS.has_feature("web"):
		var origin := str(JavaScriptBridge.eval("window.location.origin", true))
		if origin != "" and origin != "null":
			api_base = origin + "/api"
		# ?dev_user=NAME&autotest=1 (development builds only; the server
		# must also allow dev logins)
		var query := str(JavaScriptBridge.eval("window.location.search", true))
		for pair in query.trim_prefix("?").split("&", false):
			var kv := pair.split("=")
			if kv.size() == 2 and kv[0] == "dev_user":
				dev_username = kv[1].uri_decode()
			elif kv.size() == 2 and kv[0] == "autotest" and kv[1] == "1":
				autotest = true
			elif kv.size() == 2 and kv[0] == "rotate" and kv[1] == "1":
				force_rotate = true
	for arg in OS.get_cmdline_user_args() + OS.get_cmdline_args():
		if arg.begins_with("--api="):
			api_base = arg.substr(6).trim_suffix("/")
			if not api_base.ends_with("/api"):
				api_base += "/api"
		elif arg.begins_with("--ws="):
			ws_override = arg.substr(5)
		elif arg.begins_with("--dev-user="):
			dev_username = arg.substr(11)
		elif arg == "--autotest":
			autotest = true
		elif arg == "--rotate":
			force_rotate = true
		elif arg.begins_with("--shots="):
			shot_dir = arg.substr(8)

## Turns the server-provided realtime url (possibly relative) into a ws url.
func realtime_url(server_value: String) -> String:
	if ws_override != "":
		return ws_override
	if server_value.begins_with("ws://") or server_value.begins_with("wss://"):
		return server_value
	var base := api_base.trim_suffix("/api")
	if base.begins_with("https://"):
		return "wss://" + base.substr(8) + server_value
	if base.begins_with("http://"):
		return "ws://" + base.substr(7) + server_value
	return server_value

## Absolute url for a server path like "/api/v1/sprites/..." or "/v1/sprites/...".
func url(path: String) -> String:
	if path.begins_with("http://") or path.begins_with("https://"):
		return path
	if path.begins_with("/api/"):
		return api_base.trim_suffix("/api") + path
	return api_base + path

## Saves a screenshot when running with --shots=DIR (used by the autotest to
## verify the UI renders correctly).
func shot(name: String) -> void:
	if shot_dir == "" or DisplayServer.get_name() == "headless":
		return
	await RenderingServer.frame_post_draw
	var img := get_viewport().get_texture().get_image()
	img.save_png(shot_dir.path_join(name + ".png"))
	print("screenshot ", name)
