extends Node
## Root of the app: switches between the login, character select and world
## screens.

const LoginScreen := preload("res://scenes/login.gd")
const SelectScreen := preload("res://scenes/character_select.gd")
const WorldScene := preload("res://scenes/world/world.gd")

var _current: Node

func _ready() -> void:
	UiKit.install()
	get_tree().root.size_changed.connect(_adapt_orientation)
	_adapt_orientation()
	show_login()

## Keeps UI readable on phones: the base canvas is 540x960 in portrait and
## 960x540 in landscape, so stretching never shrinks the UI too much.
func _adapt_orientation() -> void:
	var win := get_tree().root.size
	var want := Vector2i(960, 540) if win.x > win.y else Vector2i(540, 960)
	if get_tree().root.content_scale_size != want:
		get_tree().root.content_scale_size = want

func _swap(n: Node) -> void:
	if _current:
		_current.queue_free()
	_current = n
	add_child(n)

func show_login() -> void:
	var s := LoginScreen.new()
	s.logged_in.connect(show_select)
	_swap(s)

func show_select() -> void:
	var s := SelectScreen.new()
	s.play.connect(show_world)
	_swap(s)

func show_world() -> void:
	var w := WorldScene.new()
	w.exit_to_menu.connect(func():
		Realtime.close()
		show_select())
	_swap(w)
