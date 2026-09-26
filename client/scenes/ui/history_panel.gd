extends Modal
## The world's chronicle and its permanent world-firsts.

func _init() -> void:
	super("Chronicle")

func _ready() -> void:
	super()
	body.add_child(UiKit.label("Loading...", 16, UiKit.MUTED))
	var firsts := await Api.get_json("/v1/history/firsts?limit=20")
	var recent := await Api.get_json("/v1/history?limit=40")
	clear_body()
	body.add_child(UiKit.label("World firsts", 20, UiKit.ACCENT, true))
	_list(firsts, Color("ffd27a"))
	body.add_child(UiKit.label("Recent history", 20, UiKit.ACCENT, true))
	_list(recent, UiKit.TEXT)

func _list(r: Dictionary, color: Color) -> void:
	if not r.ok:
		body.add_child(UiKit.label(r.error, 15, UiKit.DANGER))
		return
	if r.data.entries.is_empty():
		body.add_child(UiKit.label("Nothing yet. Make history!", 15, UiKit.MUTED))
	for e in r.data.entries:
		var t := str(e.time).substr(0, 16).replace("T", " ")
		body.add_child(UiKit.para("%s  %s" % [t, str(e.summary)], 15, color))
