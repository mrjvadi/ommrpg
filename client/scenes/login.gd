extends Control
## Logs in with Telegram Mini App init data. Outside Telegram (development)
## a username login is offered when the server allows it.

signal logged_in

var _status: Label
var _box: VBoxContainer

func _ready() -> void:
	UiKit.full_rect(self)
	add_child(UiKit.backdrop())
	var center := CenterContainer.new()
	add_child(UiKit.full_rect(center))
	var col := VBoxContainer.new()
	col.alignment = BoxContainer.ALIGNMENT_CENTER
	col.add_theme_constant_override("separation", 14)
	center.add_child(col)
	var logo := HBoxContainer.new()
	logo.alignment = BoxContainer.ALIGNMENT_CENTER
	logo.add_child(UiKit.icon("attack", 64, "gold"))
	var title := UiKit.title("OMMRPG", 64)
	logo.add_child(title)
	logo.add_child(UiKit.icon("hero", 64, "gold"))
	col.add_child(logo)
	var sub := UiKit.label("A world that writes itself", 20, UiKit.MUTED, true, 5)
	sub.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	col.add_child(sub)
	var p := UiKit.window(18)
	p.custom_minimum_size = Vector2(440, 0)
	col.add_child(p)
	_box = VBoxContainer.new()
	_box.add_theme_constant_override("separation", 12)
	p.add_child(_box)
	_status = UiKit.para("Connecting...", 18, UiKit.TEXT, true)
	_status.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_box.add_child(_status)
	_start()

func _start() -> void:
	var cfg := await Api.get_json("/v1/config")
	if not cfg.ok:
		_status.text = "Server unreachable: " + cfg.error
		_box.add_child(UiKit.button("Retry", func(): get_tree().reload_current_scene(), 52, "orange"))
		return
	if Telegram.available:
		_status.text = "Signing in with Telegram..."
		var r := await Api.post_json("/v1/auth/telegram", {"init_data": Telegram.init_data()})
		_finish(r)
		return
	if cfg.data.get("dev_login", false):
		if Cfg.dev_username != "":
			_finish(await Api.post_json("/v1/auth/dev", {"username": Cfg.dev_username}))
			return
		_status.text = "Development login"
		var name_edit := LineEdit.new()
		name_edit.placeholder_text = "username"
		name_edit.custom_minimum_size = Vector2(0, 52)
		name_edit.text = _load_dev_name()
		_box.add_child(name_edit)
		_box.add_child(UiKit.button("Enter", func():
			_save_dev_name(name_edit.text)
			_status.text = "Signing in..."
			_finish(await Api.post_json("/v1/auth/dev", {"username": name_edit.text})), 56, "orange"))
		return
	_status.text = "Open this game from its Telegram bot to play."

func _finish(r: Dictionary) -> void:
	if not r.ok:
		_status.text = "Login failed: " + r.error
		return
	Api.token = r.data.token
	Game.account = r.data.account
	HudLayout.load_remote()
	logged_in.emit()

func _load_dev_name() -> String:
	if FileAccess.file_exists("user://dev_user.txt"):
		return FileAccess.get_file_as_string("user://dev_user.txt").strip_edges()
	return ""

func _save_dev_name(n: String) -> void:
	var f := FileAccess.open("user://dev_user.txt", FileAccess.WRITE)
	if f:
		f.store_string(n)
