extends Node
## Bridge to Telegram.WebApp (https://core.telegram.org/bots/webapps).
## The game only uses Telegram as its launcher: init data for login, full
## screen, safe-area insets and haptics. Outside Telegram (desktop, editor,
## plain browser tab) every call is a harmless no-op.

signal insets_changed

var available := false
var _tg: JavaScriptObject
var _cb_viewport: JavaScriptObject
var _cb_safe: JavaScriptObject
## Insets in CSS pixels (top, right, bottom, left) that the HUD must avoid:
## device safe area (notches) + Telegram's own full-screen controls.
var insets_css := Vector4.ZERO

func _ready() -> void:
	if not OS.has_feature("web"):
		return
	var window := JavaScriptBridge.get_interface("window")
	if window == null or window.Telegram == null or window.Telegram.WebApp == null:
		return
	_tg = window.Telegram.WebApp
	if str(_tg.initData) == "":
		return # the script is loaded but we were not launched from Telegram
	available = true
	_tg.ready()
	_tg.expand()
	if bool(_tg.isVersionAtLeast("8.0")):
		_tg.requestFullscreen()
	if bool(_tg.isVersionAtLeast("7.7")):
		_tg.disableVerticalSwipes()
	_tg.setHeaderColor("#101018")
	_tg.setBackgroundColor("#101018")
	_cb_viewport = JavaScriptBridge.create_callback(_on_js_event)
	_cb_safe = JavaScriptBridge.create_callback(_on_js_event)
	_tg.onEvent("viewportChanged", _cb_viewport)
	_tg.onEvent("safeAreaChanged", _cb_safe)
	_tg.onEvent("contentSafeAreaChanged", _cb_safe)
	_tg.onEvent("fullscreenChanged", _cb_safe)
	_refresh_insets()

func _on_js_event(_args: Array) -> void:
	_refresh_insets()

func _refresh_insets() -> void:
	if not available:
		return
	var v := Vector4.ZERO
	for key in ["safeAreaInset", "contentSafeAreaInset"]:
		var o = _tg.get(key)
		if o != null:
			v += Vector4(float(o.top), float(o.right), float(o.bottom), float(o.left))
	insets_css = v
	insets_changed.emit()

func init_data() -> String:
	return str(_tg.initData) if available else ""

func start_param() -> String:
	if not available or _tg.initDataUnsafe == null:
		return ""
	return str(_tg.initDataUnsafe.start_param)

## Insets converted to canvas units of the given viewport.
func insets_for(viewport: Viewport) -> Vector4:
	if insets_css == Vector4.ZERO:
		return Vector4.ZERO
	var dpr := float(JavaScriptBridge.eval("window.devicePixelRatio || 1", true))
	var window_px := Vector2(DisplayServer.window_get_size())
	var canvas := viewport.get_visible_rect().size
	if window_px.x <= 0:
		return Vector4.ZERO
	var k := dpr * canvas.x / window_px.x
	return insets_css * k

func haptic(kind := "light") -> void:
	if not available or _tg.HapticFeedback == null:
		return
	if kind in ["error", "success", "warning"]:
		_tg.HapticFeedback.notificationOccurred(kind)
	else:
		_tg.HapticFeedback.impactOccurred(kind)

func close() -> void:
	if available:
		_tg.close()
