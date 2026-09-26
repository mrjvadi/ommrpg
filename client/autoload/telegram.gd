extends Node
## Bridge to Telegram.WebApp (Mini Apps, Bot API 8.0+). Outside Telegram
## (desktop, editor, plain browser tab) every call is a harmless no-op.
##
## What the game uses:
## * full screen (`requestFullscreen`) and, once the phone is held
##   sideways, `lockOrientation` so the landscape game does not flip back;
## * safe-area + content-safe-area insets (notch, Telegram's own buttons);
## * the native Back button (closes the top window) and Settings button;
## * native popups (`showPopup`) for real-money confirmations;
## * closing confirmation while playing, vertical-swipe lock, haptics,
##   "add to home screen" and theme colours.

signal insets_changed
signal back_pressed
signal settings_pressed
signal fullscreen_changed(on: bool)
signal active_changed(active: bool)
signal _popup_done(id: String)

var available := false
var platform := ""
var version := ""
var fullscreen_error := ""
## True while the landscape game is drawn rotated on a portrait screen
## (set by main.gd); insets are rotated accordingly.
var rotated := false
## Insets in CSS pixels (top, right, bottom, left) that the HUD must avoid:
## device safe area (notches) + Telegram's own full-screen controls.
var insets_css := Vector4.ZERO

var _tg: JavaScriptObject
var _window: JavaScriptObject
var _cbs := [] # keep JS callbacks alive
var _popup_busy := false
var _haptics := true

func _ready() -> void:
	if not OS.has_feature("web"):
		return
	_window = JavaScriptBridge.get_interface("window")
	if _window == null or _window.Telegram == null or _window.Telegram.WebApp == null:
		return
	_tg = _window.Telegram.WebApp
	if str(_tg.initData) == "":
		return # the script is loaded but we were not launched from Telegram
	available = true
	platform = str(_tg.platform)
	version = str(_tg.version)
	_tg.ready()
	_tg.expand()
	if at_least("7.7"):
		_tg.disableVerticalSwipes()
	var bg := "#" + UiKit.BG.to_html(false)
	_tg.setHeaderColor(bg)
	_tg.setBackgroundColor(bg)
	if at_least("7.10"):
		_tg.setBottomBarColor(bg)
	_on("viewportChanged", func(_a): _refresh_insets())
	_on("safeAreaChanged", func(_a): _refresh_insets())
	_on("contentSafeAreaChanged", func(_a): _refresh_insets())
	_on("fullscreenChanged", func(_a):
		_refresh_insets()
		fullscreen_changed.emit(is_fullscreen()))
	_on("fullscreenFailed", func(a):
		fullscreen_error = str(a[0].error) if a.size() > 0 and a[0] != null else "failed")
	_on("activated", func(_a): active_changed.emit(true))
	_on("deactivated", func(_a): active_changed.emit(false))
	if at_least("6.1"):
		var back := JavaScriptBridge.create_callback(func(_a): back_pressed.emit())
		_cbs.append(back)
		_tg.BackButton.onClick(back)
	if at_least("7.0"):
		var settings := JavaScriptBridge.create_callback(func(_a): settings_pressed.emit())
		_cbs.append(settings)
		_tg.SettingsButton.onClick(settings)
		_tg.SettingsButton.show()
	# showPopup takes a plain JS object; build it from JSON in JS land.
	JavaScriptBridge.eval("window.__ommPopup = function (json, cb) { window.Telegram.WebApp.showPopup(JSON.parse(json), cb); };", true)
	request_fullscreen()
	_refresh_insets()

func _on(event: String, fn: Callable) -> void:
	var cb := JavaScriptBridge.create_callback(fn)
	_cbs.append(cb)
	_tg.onEvent(event, cb)

func at_least(v: String) -> bool:
	return available and bool(_tg.isVersionAtLeast(v))

## Telegram on a phone or tablet (not desktop / web versions).
func is_mobile() -> bool:
	return platform in ["android", "android_x", "ios"]

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

## Insets converted to canvas units of the given viewport (top, right,
## bottom, left). When the game is drawn rotated, the screen's right edge is
## the game's top edge and so on.
func insets_for(viewport: Viewport) -> Vector4:
	if insets_css == Vector4.ZERO or viewport == null:
		return Vector4.ZERO
	var dpr := float(JavaScriptBridge.eval("window.devicePixelRatio || 1", true))
	var window_px := Vector2(DisplayServer.window_get_size())
	var canvas := viewport.get_visible_rect().size
	if window_px.x <= 0 or window_px.y <= 0:
		return Vector4.ZERO
	if rotated:
		var k := dpr * canvas.x / window_px.y
		return Vector4(insets_css.y, insets_css.z, insets_css.w, insets_css.x) * k
	return insets_css * (dpr * canvas.x / window_px.x)

# ---------------------------------------------------------------- screen

func request_fullscreen() -> void:
	if at_least("8.0") and not is_fullscreen():
		fullscreen_error = ""
		_tg.requestFullscreen()

func exit_fullscreen() -> void:
	if at_least("8.0") and is_fullscreen():
		_tg.exitFullscreen()

func is_fullscreen() -> bool:
	return at_least("8.0") and bool(_tg.isFullscreen)

## Locks the current orientation (call it once the phone is sideways).
func lock_orientation(on: bool) -> void:
	if not at_least("8.0"):
		return
	if on and not bool(_tg.isOrientationLocked):
		_tg.lockOrientation()
	elif not on and bool(_tg.isOrientationLocked):
		_tg.unlockOrientation()

## Asks before Telegram closes the game (enabled while in the world).
func confirm_closing(on: bool) -> void:
	if not at_least("6.2"):
		return
	if on:
		_tg.enableClosingConfirmation()
	else:
		_tg.disableClosingConfirmation()

func set_back_button(on: bool) -> void:
	if not at_least("6.1"):
		return
	if on:
		_tg.BackButton.show()
	else:
		_tg.BackButton.hide()

# ---------------------------------------------------------------- popups & feedback

## Native Telegram popup. `buttons` are {id, type, text} with type one of
## default, destructive, ok, close, cancel (max 3). Returns the pressed id,
## "" when dismissed, or "unsupported" outside Telegram.
func popup(title: String, message: String, buttons: Array) -> String:
	if not at_least("6.2") or _popup_busy:
		return "unsupported"
	_popup_busy = true
	var params := {"title": title.substr(0, 64), "message": message.substr(0, 256), "buttons": buttons.slice(0, 3)}
	var cb := JavaScriptBridge.create_callback(func(a): _popup_done.emit(str(a[0]) if a.size() > 0 and a[0] != null else ""))
	_cbs.append(cb)
	_window.__ommPopup(JSON.stringify(params), cb)
	var id: String = await _popup_done
	_popup_busy = false
	return id

func haptic(kind := "light") -> void:
	if not available or not _haptics or _tg.HapticFeedback == null or not at_least("6.1"):
		return
	if kind in ["error", "success", "warning"]:
		_tg.HapticFeedback.notificationOccurred(kind)
	elif kind == "selection":
		_tg.HapticFeedback.selectionChanged()
	else:
		_tg.HapticFeedback.impactOccurred(kind)

func set_haptics(on: bool) -> void:
	_haptics = on

func haptics_enabled() -> bool:
	return _haptics

## "unsupported", "unknown", "added" or "missed".
func home_screen_status() -> String:
	if not at_least("8.0"):
		return "unsupported"
	var cb := JavaScriptBridge.create_callback(func(a): _popup_done.emit("home:" + (str(a[0]) if a.size() > 0 else "unknown")))
	_cbs.append(cb)
	_tg.checkHomeScreenStatus(cb)
	while true:
		var id: String = await _popup_done
		if id.begins_with("home:"):
			return id.substr(5)
	return "unknown"

func add_to_home_screen() -> void:
	if at_least("8.0"):
		_tg.addToHomeScreen()

## Opens an external link (TON wallets, docs) outside the game.
func open_link(url: String) -> void:
	if available and url.begins_with("https://t.me/"):
		_tg.openTelegramLink(url)
	elif available:
		_tg.openLink(url)
	else:
		OS.shell_open(url)

func close() -> void:
	if available:
		_tg.close()
