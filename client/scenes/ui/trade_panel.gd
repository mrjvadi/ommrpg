extends Modal
## Trade: the player marketplace (TON or gold), the NFT vault (mint items
## onto OMM Chain, claim them back onto a hero, sell them) and the custodial
## TON wallet (deposit by memo, withdraw to any TON address).

const NANO := 1_000_000_000.0
const SLOTS := ["", "weapon", "offhand", "head", "chest", "legs", "feet", "hands", "ring", "amulet"]
const SORTS := [["new", "Newest"], ["price_asc", "Cheapest"], ["price_desc", "Priciest"]]
const STATE_NAMES := {"vault": "In vault", "bound": "On a hero", "claiming": "Claiming...", "depositing": "Depositing...", "pending": "Minting..."}

var _tab := "market"
var _tabs: HBoxContainer
var _content: VBoxContainer
var _filters := {"currency": "", "slot": "", "rarity": "", "sort": "new", "mine": false}
var _wallet: Dictionary = {}
var _busy := false
var _armed := ""  # two-tap confirmation for spending actions

func _init(tab := "market") -> void:
	super("Trade")
	_tab = tab

func _ready() -> void:
	super()
	_tabs = HBoxContainer.new()
	body.add_child(_tabs)
	for pair in [["market", "Market"], ["vault", "My NFTs"], ["wallet", "TON wallet"]]:
		var b := UiKit.button(pair[1], func(): show_tab(pair[0]), 40)
		b.toggle_mode = true
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.set_meta("tab", pair[0])
		_tabs.add_child(b)
	_content = VBoxContainer.new()
	_content.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	body.add_child(_content)
	show_tab(_tab)

func show_tab(tab: String) -> void:
	_tab = tab
	_armed = ""
	for b in _tabs.get_children():
		b.button_pressed = b.get_meta("tab") == tab
	_clear()
	_content.add_child(UiKit.label("Loading...", 16, UiKit.MUTED))
	var w := await Api.get_json("/v1/ton")
	if w.ok:
		_wallet = w.data
	match tab:
		"market":
			await _market()
		"vault":
			await _vault()
		"wallet":
			_render_wallet()

## Called by the world when a trade-related push arrives.
func refresh() -> void:
	show_tab(_tab)

func _clear() -> void:
	for c in _content.get_children():
		c.queue_free()

func _policy() -> Dictionary:
	return _wallet.get("policy", {})

static func ton(nano) -> String:
	var v := float(nano) / NANO
	return ("%.3f" % v).rstrip("0").rstrip(".") + " TON"

static func price_text(currency: String, price) -> String:
	return ton(price) if currency == "TON" else "%d gold" % int(price)

func _balance_line() -> Control:
	var row := HBoxContainer.new()
	row.add_child(UiKit.label("Gold %d" % int(Game.wallet.get("gold", 0)), 17, UiKit.ACCENT))
	row.add_child(UiKit.label("   %s" % ton(_wallet.get("balance", 0)), 17, Color("4db8ff")))
	if int(_wallet.get("locked", 0)) > 0:
		row.add_child(UiKit.label("  (locked %s)" % ton(_wallet.locked), 14, UiKit.MUTED))
	return row

# ---------------------------------------------------------------- market

func _market() -> void:
	var q := "/v1/market?limit=60&sort=%s&currency=%s&slot=%s&rarity=%s%s" % [
		_filters.sort, _filters.currency, _filters.slot, _filters.rarity, "&mine=1" if _filters.mine else ""]
	var r := await Api.get_json(q)
	_clear()
	_content.add_child(_balance_line())
	_content.add_child(_market_filters())
	if not r.ok:
		_content.add_child(UiKit.para(r.error, 15, UiKit.DANGER))
		return
	var list: Array = r.data.get("listings", [])
	if list.is_empty():
		_content.add_child(UiKit.para("No listings match. Mint a rare item in \"My NFTs\" and put it up for sale!", 15, UiKit.MUTED))
	for l in list:
		_listing_card(l)

func _market_filters() -> Control:
	var g := GridContainer.new()
	g.columns = 2
	g.add_child(_option(["", "TON", "GOLD"], ["Any currency", "TON", "Gold"], _filters.currency, func(v): _filters.currency = v))
	g.add_child(_option([""] + Sprites.RARITIES, ["Any rarity"] + Sprites.RARITIES.map(func(x): return str(x).capitalize()), _filters.rarity, func(v): _filters.rarity = v))
	g.add_child(_option(SLOTS, ["Any slot"] + SLOTS.slice(1).map(func(x): return str(x).capitalize()), _filters.slot, func(v): _filters.slot = v))
	g.add_child(_option(SORTS.map(func(x): return x[0]), SORTS.map(func(x): return x[1]), _filters.sort, func(v): _filters.sort = v))
	var mine := CheckButton.new()
	mine.text = "Only my listings"
	mine.button_pressed = _filters.mine
	mine.toggled.connect(func(on): _filters.mine = on; show_tab("market"))
	g.add_child(mine)
	return g

func _option(values: Array, labels: Array, current: String, set_value: Callable) -> OptionButton:
	var o := OptionButton.new()
	o.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	o.custom_minimum_size.y = 40
	for i in values.size():
		o.add_item(labels[i], i)
		if values[i] == current:
			o.select(i)
	o.item_selected.connect(func(i): set_value.call(values[i]); show_tab(_tab))
	return o

func _item_header(snap: Dictionary, extra: String) -> HBoxContainer:
	var row := HBoxContainer.new()
	var icon := TextureRect.new()
	icon.custom_minimum_size = Vector2(56, 56)
	icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	icon.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	row.add_child(icon)
	var item: Dictionary = snap.get("item", {})
	_load_icon(icon, item)
	var col := VBoxContainer.new()
	col.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	col.add_theme_constant_override("separation", 2)
	row.add_child(col)
	var rar := str(item.get("rarity", "common"))
	col.add_child(UiKit.para(str(snap.get("display_name", item.get("name", "?"))), 17, Sprites.rarity_color(rar), true))
	var st: Dictionary = snap.get("state", {})
	var line := "%s %s  ·  L%d" % [rar.capitalize(), str(item.get("slot", "")), int(st.get("level", 1))]
	if str(item.get("element", "")) != "":
		line += "  ·  " + str(item.element).capitalize()
	if extra != "":
		line += "  ·  " + extra
	col.add_child(UiKit.label(line, 13, UiKit.MUTED))
	return row

func _load_icon(icon: TextureRect, item: Dictionary) -> void:
	var tex := await Sprites.fetch(Sprites.icon_url(item))
	if is_instance_valid(icon):
		icon.texture = tex

func _card() -> VBoxContainer:
	var p := UiKit.panel()
	p.add_theme_stylebox_override("panel", UiKit.box(Color("14141f"), 8, 1, Color("2e2e48"), 10))
	_content.add_child(p)
	var col := VBoxContainer.new()
	p.add_child(col)
	return col

func _listing_card(l: Dictionary) -> void:
	var col := _card()
	var snap: Dictionary = l.get("token", {}).get("item", {})
	col.add_child(_item_header(snap, "NFT #%d" % int(l.token_id)))
	var row := HBoxContainer.new()
	col.add_child(row)
	var price := UiKit.label(price_text(str(l.currency), l.price), 19, Color("4db8ff") if l.currency == "TON" else UiKit.ACCENT, true)
	price.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(price)
	var key := "listing:%d" % int(l.id)
	if l.get("mine", false):
		row.add_child(UiKit.button("Cancel", func(): _act("/v1/market/%d/cancel" % int(l.id), {}, "Listing cancelled"), 40))
	else:
		var label := "Confirm %s?" % price_text(str(l.currency), l.price) if _armed == key else "Buy"
		var b := UiKit.button(label, func(): _buy(l, key), 40)
		if _armed == key:
			b.add_theme_color_override("font_color", UiKit.ACCENT)
		row.add_child(b)

func _buy(l: Dictionary, key: String) -> void:
	if _armed != key:
		_armed = key
		Telegram.haptic("light")
		show_tab(_tab)
		return
	_armed = ""
	await _act("/v1/market/%d/buy" % int(l.id), {"request_id": Api.request_id()}, "Bought! Find it in My NFTs")

func _act(path: String, payload: Dictionary, ok_text: String) -> bool:
	if _busy:
		return false
	_busy = true
	var r := await Api.post_json(path, payload)
	_busy = false
	if r.ok:
		Game.toast(ok_text, UiKit.GOOD)
		Telegram.haptic("success")
		Game.refresh_inventory()
		Game.refresh_character()
	else:
		Game.toast(r.error, UiKit.DANGER)
		Telegram.haptic("error")
	show_tab(_tab)
	return r.ok

# ---------------------------------------------------------------- vault

func _vault() -> void:
	var tr := await Api.get_json("/v1/vault")
	var inv := await Game.refresh_inventory()
	_clear()
	_content.add_child(_balance_line())
	var tokens: Array = tr.data.get("tokens", []) if tr.ok else []
	var bound := {}
	for t in tokens:
		if str(t.state) == "bound":
			bound[str(t.item_id)] = t
	_content.add_child(UiKit.label("Your NFTs on OMM Chain", 19, UiKit.ACCENT, true))
	if tokens.is_empty():
		_content.add_child(UiKit.para("You have no NFTs yet.", 15, UiKit.MUTED))
	for t in tokens:
		_token_card(t)
	var min_rar := str(_policy().get("mint_min_rarity", "rare"))
	_content.add_child(UiKit.label("Mint from your bag", 19, UiKit.ACCENT, true))
	_content.add_child(UiKit.para("%s or better items can become tradeable NFTs. Minting moves the item into your vault and records it on OMM Chain." % min_rar.capitalize(), 14, UiKit.MUTED))
	var any := false
	for it in inv.get("items", []):
		if Sprites.rarity_index(str(it.item.rarity)) < Sprites.rarity_index(min_rar) or bound.has(str(it.id)):
			continue
		any = true
		var col := _card()
		col.add_child(_item_header(it, "equipped" if str(it.get("equipped", "")) != "" else ""))
		if str(it.get("equipped", "")) != "":
			col.add_child(UiKit.label("Unequip it first to mint", 14, UiKit.MUTED))
			continue
		var key := "mint:" + str(it.id)
		col.add_child(UiKit.button("Confirm mint?" if _armed == key else "Mint as NFT", func(): _mint(it, key), 40))
	if not any:
		_content.add_child(UiKit.para("No eligible items in your bag.", 15, UiKit.MUTED))

func _mint(it: Dictionary, key: String) -> void:
	if _armed != key:
		_armed = key
		show_tab("vault")
		return
	_armed = ""
	await _act("/v1/vault/mint", {"item_id": str(it.id), "request_id": Api.request_id()}, "Minted onto OMM Chain")

func _token_card(t: Dictionary) -> void:
	var col := _card()
	var state := str(t.state)
	var extra := "NFT #%d  ·  %s" % [int(t.id), STATE_NAMES.get(state, state)]
	if int(t.get("listing_id", 0)) > 0:
		extra += "  ·  for sale"
	col.add_child(_item_header(t.get("item", {}), extra))
	var row := HBoxContainer.new()
	col.add_child(row)
	var id := int(t.id)
	if state == "vault" and int(t.get("listing_id", 0)) == 0:
		row.add_child(UiKit.button("Claim to hero", func(): _act("/v1/vault/%d/claim" % id, {}, "The item is in your bag"), 40))
		row.add_child(UiKit.button("Sell...", func(): _sell_form(col, t), 40))
	elif state == "bound":
		if str(t.get("bound_character", "")) == str(Game.character.get("id", "")):
			row.add_child(UiKit.button("Back to vault", func(): _act("/v1/vault/%d/deposit" % id, {}, "Stored in your vault"), 40))
			var key := "burn:%d" % id
			row.add_child(UiKit.button("Really unmint?" if _armed == key else "Unmint", func(): _burn(id, key), 40))
		else:
			row.add_child(UiKit.label("On another hero", 14, UiKit.MUTED))

func _burn(id: int, key: String) -> void:
	if _armed != key:
		_armed = key
		show_tab("vault")
		return
	_armed = ""
	await _act("/v1/vault/%d/burn" % id, {}, "Unminted: it is a normal item again")

func _sell_form(col: VBoxContainer, t: Dictionary) -> void:
	var p := _policy()
	var rar := Sprites.rarity_index(str(t.rarity))
	var currencies := []
	if rar >= Sprites.rarity_index(str(p.get("ton_min_rarity", "epic"))):
		currencies.append("TON")
	if rar >= Sprites.rarity_index(str(p.get("gold_min_rarity", "rare"))):
		currencies.append("GOLD")
	if currencies.is_empty():
		Game.toast("This rarity cannot be traded", UiKit.DANGER)
		return
	var form := HBoxContainer.new()
	col.add_child(form)
	var cur := OptionButton.new()
	for c in currencies:
		cur.add_item(c)
	form.add_child(cur)
	var price := LineEdit.new()
	price.placeholder_text = "Price"
	price.virtual_keyboard_type = LineEdit.KEYBOARD_TYPE_NUMBER_DECIMAL
	price.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	form.add_child(price)
	var fee := float(p.get("fee_bps", 250)) / 100.0
	col.add_child(UiKit.para("Marketplace fee %.1f%%. Min price %s or %d gold." % [fee, ton(p.get("min_price_ton", 0)), int(p.get("min_price_gold", 0))], 13, UiKit.MUTED))
	form.add_child(UiKit.button("List", func():
		var currency: String = currencies[cur.selected]
		var v := price.text.strip_edges().to_float()
		var amount := int(round(v * NANO)) if currency == "TON" else int(v)
		if amount <= 0:
			Game.toast("Enter a price", UiKit.DANGER)
			return
		_act("/v1/vault/%d/list" % int(t.id), {"currency": currency, "price": amount, "request_id": Api.request_id()}, "Listed on the market"), 40))
	price.grab_focus()

# ---------------------------------------------------------------- wallet

func _render_wallet() -> void:
	_clear()
	if _wallet.is_empty():
		_content.add_child(UiKit.para("The TON wallet is unavailable right now.", 15, UiKit.DANGER))
		return
	var w := _wallet
	var p := _policy()
	_content.add_child(UiKit.label(ton(w.balance), 34, Color("4db8ff"), true))
	if int(w.locked) > 0:
		_content.add_child(UiKit.label("%s locked in pending withdrawals" % ton(w.locked), 14, UiKit.MUTED))
	_content.add_child(UiKit.label("Network: %s" % str(w.get("network", "")), 13, UiKit.MUTED))

	_content.add_child(UiKit.label("Deposit", 19, UiKit.ACCENT, true))
	_content.add_child(UiKit.para("Send TON to this address with the memo (comment) below. The memo identifies your account; transfers without it cannot be credited.", 14, UiKit.MUTED))
	_content.add_child(_copy_row("Address", str(w.deposit_address)))
	_content.add_child(_copy_row("Memo", str(w.deposit_memo)))
	_content.add_child(UiKit.button("Open in TON wallet", func():
		OS.shell_open("https://app.tonkeeper.com/transfer/%s?text=%s" % [str(w.deposit_address), str(w.deposit_memo).uri_encode()])))

	_content.add_child(UiKit.label("Withdraw", 19, UiKit.ACCENT, true))
	if not p.get("withdraw_enabled", true):
		_content.add_child(UiKit.para("Withdrawals are paused by the operators.", 14, UiKit.DANGER))
	else:
		var addr := LineEdit.new()
		addr.placeholder_text = "TON address (EQ... / UQ...)"
		_content.add_child(addr)
		var row := HBoxContainer.new()
		_content.add_child(row)
		var amt := LineEdit.new()
		amt.placeholder_text = "Amount in TON"
		amt.virtual_keyboard_type = LineEdit.KEYBOARD_TYPE_NUMBER_DECIMAL
		amt.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		row.add_child(amt)
		row.add_child(UiKit.button("Withdraw", func():
			var nano := int(round(amt.text.strip_edges().to_float() * NANO))
			if nano <= 0 or addr.text.strip_edges().length() < 40:
				Game.toast("Enter an address and an amount", UiKit.DANGER)
				return
			_act("/v1/ton/withdraw", {"to_address": addr.text.strip_edges(), "amount": nano, "request_id": Api.request_id()}, "Withdrawal requested"), 44))
		_content.add_child(UiKit.para("Minimum %s, network fee %s. Large withdrawals are reviewed by an operator before sending." % [ton(p.get("withdraw_min", 0)), ton(p.get("withdraw_fee", 0))], 13, UiKit.MUTED))

	var wds: Array = w.get("withdrawals", [])
	if not wds.is_empty():
		_content.add_child(UiKit.label("Withdrawals", 19, UiKit.ACCENT, true))
		for x in wds:
			var col: Color = {"sent": UiKit.GOOD, "failed": UiKit.DANGER, "rejected": UiKit.DANGER}.get(str(x.status), UiKit.TEXT)
			_content.add_child(UiKit.label("%s  →  %s…  %s" % [ton(x.amount), str(x.to_address).substr(0, 8), str(x.status)], 14, col))
	var ledger: Array = w.get("ledger", [])
	if not ledger.is_empty():
		_content.add_child(UiKit.label("History", 19, UiKit.ACCENT, true))
		for e in ledger:
			var d := int(e.delta)
			_content.add_child(UiKit.label("%s  %s%s  %s" % [str(e.created_at).substr(0, 16).replace("T", " "), "+" if d > 0 else "", ton(d), str(e.reason)], 14, UiKit.GOOD if d > 0 else UiKit.TEXT))

func _copy_row(label: String, value: String) -> Control:
	var row := HBoxContainer.new()
	row.add_child(UiKit.label(label, 14, UiKit.MUTED))
	var v := LineEdit.new()
	v.text = value
	v.editable = false
	v.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	v.add_theme_font_size_override("font_size", 15)
	row.add_child(v)
	row.add_child(UiKit.button("Copy", func():
		DisplayServer.clipboard_set(value)
		Game.toast(label + " copied", UiKit.GOOD), 40))
	return row
