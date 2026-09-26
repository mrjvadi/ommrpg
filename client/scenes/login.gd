extends Control
## Logs in with Telegram Mini App init data. Outside Telegram (development)
## a username login is offered when the server allows it.

signal logged_in

var _status: Label
var _box: VBoxContainer

func _ready() -> void:
	UiKit.full_rect(self)
	var bg := ColorRect.new()
	bg.color = UiKit.BG
	add_child(UiKit.full_rect(bg))
	var center := CenterContainer.new()
	add_child(UiKit.full_rect(center))
	var p := UiKit.panel()
	p.custom_minimum_size = Vector2(380, 0)
	center.add_child(p)
	_box = VBoxContainer.new()
	p.add_child(_box)
	var title := UiKit.label("OMMRPG", 44, UiKit.ACCENT, true)
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_box.add_child(title)
	var sub := UiKit.para("A world that writes itself", 18, UiKit.MUTED)
	sub.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_box.add_child(sub)
	_status = UiKit.para("Connecting...", 18, UiKit.TEXT)
	_status.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_box.add_child(_status)
	_start()

func _start() -> void:
	var cfg := await Api.get_json("/v1/config")
	if not cfg.ok:
		_status.text = "Server unreachable: " + cfg.error
		_box.add_child(UiKit.button("Retry", func(): get_tree().reload_current_scene()))
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
		name_edit.custom_minimum_size = Vector2(0, 48)
		var saved := _load_dev_name()
		name_edit.text = saved
		_box.add_child(name_edit)
		_box.add_child(UiKit.button("Enter", func():
			_save_dev_name(name_edit.text)
			_status.text = "Signing in..."
			_finish(await Api.post_json("/v1/auth/dev", {"username": name_edit.text}))))
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
