extends Modal
## Trade: the player marketplace (TON or gold), the NFT vault (mint items
## onto OMM Chain, claim them back onto a hero, sell them) and the custodial
## TON wallet (deposit by memo, withdraw to any TON address). Real-money
## actions are confirmed with Telegram's native popup.

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

func _init(tab := "market") -> void:
	super("Trade")
	_tab = tab
	scrollable = false
	max_size = Vector2(1120, 640)

func _ready() -> void:
	super()
	_content = VBoxContainer.new()
	_content.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_content.size_flags_vertical = Control.SIZE_EXPAND_FILL
	body.add_child(_content)
	show_tab(_tab)

func show_tab(tab: String) -> void:
	_tab = tab
	if is_instance_valid(_tabs):
		_tabs.queue_free()
	_tabs = UiKit.tabs([["market", "Market", "trade"], ["vault", "My NFTs", "vault"], ["wallet", "TON wallet", "ton"]], tab, show_tab, 44)
	body.add_child(_tabs)
	body.move_child(_tabs, 0)
	_clear()
	_content.add_child(UiKit.label("Loading...", 18, UiKit.MUTED))
	var w := await Api.get_json("/v1/ton")
	if w.ok:
		_wallet = w.data
		Game.ton_balance = int(_wallet.get("balance", 0))
		Game.wallet_changed.emit()
	if not is_inside_tree() or _tab != tab:
		return
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
	return ton(price) if currency == "TON" else "%s gold" % UiKit.num(price)

func _balances() -> HBoxContainer:
	var row := HBoxContainer.new()
	var g := UiKit.pill("gold", "gold")
	g.set_value(UiKit.short(Game.wallet.get("gold", 0)))
	row.add_child(g)
	var t := UiKit.pill("ton", "ton", func(): show_tab("wallet"))
	t.set_value(ton(_wallet.get("balance", 0)).trim_suffix(" TON"))
	row.add_child(t)
	return row

## Columns: a fixed-width side bar and a scrolling grid of cards.
func _split(side_width := 230.0) -> Array:
	var row := HBoxContainer.new()
	row.size_flags_vertical = Control.SIZE_EXPAND_FILL
	row.add_theme_constant_override("separation", 12)
	_content.add_child(row)
	var side := VBoxContainer.new()
	side.custom_minimum_size.x = side_width
	row.add_child(side)
	var scroll := ScrollContainer.new()
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	row.add_child(scroll)
	var grid := GridContainer.new()
	grid.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	grid.add_theme_constant_override("h_separation", 10)
	grid.add_theme_constant_override("v_separation", 10)
	scroll.add_child(grid)
	scroll.resized.connect(func(): grid.columns = maxi(1, int((scroll.size.x - 12) / 292.0)))
	return [side, grid]

# ---------------------------------------------------------------- market

func _market() -> void:
	var q := "/v1/market?limit=60&sort=%s&currency=%s&slot=%s&rarity=%s%s" % [
		_filters.sort, _filters.currency, _filters.slot, _filters.rarity, "&mine=1" if _filters.mine else ""]
	var r := await Api.get_json(q)
	if not is_inside_tree():
		return
	_clear()
	var parts := _split()
	var side: VBoxContainer = parts[0]
	var grid: GridContainer = parts[1]
	side.add_child(_balances())
	var f := UiKit.inset(10)
	side.add_child(f)
	var fcol := VBoxContainer.new()
	f.add_child(fcol)
	fcol.add_child(UiKit.label("Filters", 18, UiKit.GOLD))
	fcol.add_child(_option(["", "TON", "GOLD"], ["Any currency", "TON", "Gold"], _filters.currency, func(v): _filters.currency = v))
	fcol.add_child(_option([""] + Sprites.RARITIES, ["Any rarity"] + Sprites.RARITIES.map(func(x): return str(x).capitalize()), _filters.rarity, func(v): _filters.rarity = v))
	fcol.add_child(_option(SLOTS, ["Any slot"] + SLOTS.slice(1).map(func(x): return str(x).capitalize()), _filters.slot, func(v): _filters.slot = v))
	fcol.add_child(_option(SORTS.map(func(x): return x[0]), SORTS.map(func(x): return x[1]), _filters.sort, func(v): _filters.sort = v))
	var mine := CheckButton.new()
	mine.text = "Only mine"
	mine.button_pressed = _filters.mine
	mine.toggled.connect(func(on):
		_filters.mine = on
		show_tab("market"))
	fcol.add_child(mine)
	if not r.ok:
		side.add_child(UiKit.para(r.error, 15, UiKit.DANGER))
		return
	var list: Array = r.data.get("listings", [])
	if list.is_empty():
		var p := UiKit.para("No listings match. Mint a rare item in \"My NFTs\" and put it up for sale!", 17, UiKit.MUTED)
		p.custom_minimum_size.x = 300
		grid.add_child(p)
	for l in list:
		grid.add_child(_listing_card(l))

func _option(values: Array, labels: Array, current: String, set_value: Callable) -> OptionButton:
	var o := OptionButton.new()
	o.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	o.custom_minimum_size.y = 44
	o.add_theme_font_size_override("font_size", 17)
	for i in values.size():
		o.add_item(labels[i], i)
		if values[i] == current:
			o.select(i)
	o.item_selected.connect(func(i):
		set_value.call(values[i])
		show_tab(_tab))
	return o

## A card: slot on the left, name / details / action on the right.
func _card(snap: Dictionary, extra: String) -> Array:
	var p := PanelContainer.new()
	p.add_theme_stylebox_override("panel", UiKit.panel_box(10))
	p.custom_minimum_size.x = 280
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 10)
	p.add_child(row)
	var s := ItemSlot.new(78)
	if not snap.is_empty() and snap.has("item"):
		s.set_item(snap)
	s.disabled = true
	row.add_child(s)
	var col := VBoxContainer.new()
	col.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	col.add_theme_constant_override("separation", 3)
	row.add_child(col)
	var item: Dictionary = snap.get("item", {})
	var rar := str(item.get("rarity", "common"))
	var nm := UiKit.label(str(snap.get("display_name", item.get("name", "?"))), 16, Sprites.rarity_color(rar), true, 4)
	nm.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	nm.custom_minimum_size.x = 160
	col.add_child(nm)
	var line := "%s %s" % [rar.capitalize(), str(item.get("slot", ""))]
	if str(item.get("element", "")) != "":
		line += " · " + str(item.element).capitalize()
	if extra != "":
		line += " · " + extra
	col.add_child(UiKit.label(line, 13, UiKit.MUTED, true, 3))
	return [p, col]

func _listing_card(l: Dictionary) -> Control:
	var parts := _card(l.get("token", {}).get("item", {}), "NFT #%d" % int(l.token_id))
	var col: VBoxContainer = parts[1]
	var row := HBoxContainer.new()
	col.add_child(row)
	var is_ton := str(l.currency) == "TON"
	var pr := HBoxContainer.new()
	pr.add_theme_constant_override("separation", 2)
	pr.add_child(UiKit.icon("ton" if is_ton else "gold", 24, "ton" if is_ton else "gold"))
	pr.add_child(UiKit.label(price_text(str(l.currency), l.price).trim_suffix(" TON").trim_suffix(" gold"), 20, UiKit.TON if is_ton else UiKit.GOLD))
	pr.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(pr)
	if l.get("mine", false):
		row.add_child(UiKit.button("Cancel", func(): _act("/v1/market/%d/cancel" % int(l.id), {}, "Listing cancelled"), 40, "gray"))
	else:
		row.add_child(UiKit.button("Buy", func(): _buy(l), 40, "blue" if is_ton else "orange"))
	return parts[0]

func _buy(l: Dictionary) -> void:
	var name := str(l.get("token", {}).get("item", {}).get("display_name", "this item"))
	var price := price_text(str(l.currency), l.price)
	var ok := false
	if str(l.currency) == "TON":
		ok = await Popups.money_confirm("Buy for %s?" % price, "%s will be paid from your TON balance." % name, "Pay %s" % price)
	else:
		ok = await Popups.confirm("Buy for %s?" % price, name, "Buy", "orange")
	if ok:
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
	if is_inside_tree():
		show_tab(_tab)
	return r.ok

# ---------------------------------------------------------------- vault

func _vault() -> void:
	var tr := await Api.get_json("/v1/vault")
	var inv := await Game.refresh_inventory()
	if not is_inside_tree():
		return
	_clear()
	var parts := _split()
	var side: VBoxContainer = parts[0]
	var grid: GridContainer = parts[1]
	side.add_child(_balances())
	var min_rar := str(_policy().get("mint_min_rarity", "rare"))
	side.add_child(UiKit.para("Your NFTs live on OMM Chain. %s or better items from your bag can be minted and sold for gold or TON." % min_rar.capitalize(), 15, UiKit.MUTED))
	var tokens: Array = tr.data.get("tokens", []) if tr.ok else []
	var bound := {}
	for t in tokens:
		if str(t.state) == "bound":
			bound[str(t.item_id)] = t
	for t in tokens:
		grid.add_child(_token_card(t))
	var any := false
	for it in inv.get("items", []):
		if Sprites.rarity_index(str(it.item.rarity)) < Sprites.rarity_index(min_rar) or bound.has(str(it.id)):
			continue
		any = true
		var cparts := _card(it, "in bag")
		var col: VBoxContainer = cparts[1]
		if str(it.get("equipped", "")) != "":
			col.add_child(UiKit.label("Unequip it first to mint", 13, UiKit.MUTED, true, 3))
		else:
			col.add_child(UiKit.button("Mint as NFT", func(): _mint(it), 40, "purple"))
		grid.add_child(cparts[0])
	if tokens.is_empty() and not any:
		var p := UiKit.para("No NFTs yet and no eligible items in your bag.", 17, UiKit.MUTED)
		p.custom_minimum_size.x = 300
		grid.add_child(p)

func _mint(it: Dictionary) -> void:
	if await Popups.confirm("Mint as NFT?", "%s moves to your vault on OMM Chain. You can claim it back any time." % str(it.display_name), "Mint", "purple"):
		await _act("/v1/vault/mint", {"item_id": str(it.id), "request_id": Api.request_id()}, "Minted onto OMM Chain")

func _token_card(t: Dictionary) -> Control:
	var state := str(t.state)
	var extra := "#%d · %s" % [int(t.id), STATE_NAMES.get(state, state)]
	if int(t.get("listing_id", 0)) > 0:
		extra += " · for sale"
	var parts := _card(t.get("item", {}), extra)
	var col: VBoxContainer = parts[1]
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 6)
	col.add_child(row)
	var id := int(t.id)
	if state == "vault" and int(t.get("listing_id", 0)) == 0:
		row.add_child(UiKit.button("Claim", func(): _act("/v1/vault/%d/claim" % id, {}, "The item is in your bag"), 38, "green"))
		row.add_child(UiKit.button("Sell", func(): _sell(t), 38, "orange"))
	elif state == "bound":
		if str(t.get("bound_character", "")) == str(Game.character.get("id", "")):
			row.add_child(UiKit.button("To vault", func(): _act("/v1/vault/%d/deposit" % id, {}, "Stored in your vault"), 38, "blue"))
			row.add_child(UiKit.button("Unmint", func():
				if await Popups.confirm("Unmint?", "The NFT is burned on OMM Chain and the item becomes a normal item again.", "Unmint", "red"):
					_act("/v1/vault/%d/burn" % id, {}, "Unminted: it is a normal item again"), 38, "red"))
		else:
			row.add_child(UiKit.label("On another hero", 13, UiKit.MUTED, true, 3))
	return parts[0]

func _sell(t: Dictionary) -> void:
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
	var form := SellForm.new(t, currencies, p)
	form.submit.connect(func(currency, amount):
		_act("/v1/vault/%d/list" % int(t.id), {"currency": currency, "price": amount, "request_id": Api.request_id()}, "Listed on the market"))
	Popups.open(form)

# ---------------------------------------------------------------- wallet

func _render_wallet() -> void:
	_clear()
	if _wallet.is_empty():
		_content.add_child(UiKit.para("The TON wallet is unavailable right now.", 17, UiKit.DANGER))
		return
	var w := _wallet
	var p := _policy()
	var row := HBoxContainer.new()
	row.size_flags_vertical = Control.SIZE_EXPAND_FILL
	row.add_theme_constant_override("separation", 14)
	_content.add_child(row)
	var left := VBoxContainer.new()
	left.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(left)
	var right := VBoxContainer.new()
	right.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(right)

	var bal := HBoxContainer.new()
	bal.add_child(UiKit.icon("ton", 52, "ton"))
	var bcol := VBoxContainer.new()
	bcol.add_theme_constant_override("separation", -4)
	bcol.add_child(UiKit.label(ton(w.balance), 34, UiKit.TON, true, 7))
	var sub := "Network: %s" % str(w.get("network", ""))
	if int(w.locked) > 0:
		sub += "  ·  %s locked" % ton(w.locked)
	bcol.add_child(UiKit.label(sub, 14, UiKit.MUTED, true, 3))
	bal.add_child(bcol)
	left.add_child(bal)
	left.add_child(UiKit.label("Deposit", 20, UiKit.GOLD))
	left.add_child(UiKit.para("Send TON to this address with the memo (comment) below. The memo identifies your account; transfers without it cannot be credited.", 14, UiKit.MUTED))
	left.add_child(_copy_row("Address", str(w.deposit_address)))
	left.add_child(_copy_row("Memo", str(w.deposit_memo)))
	left.add_child(UiKit.button("Open in TON wallet", func():
		Telegram.open_link("https://app.tonkeeper.com/transfer/%s?text=%s" % [str(w.deposit_address), str(w.deposit_memo).uri_encode()]), 48, "blue", "wallet"))

	right.add_child(UiKit.label("Withdraw", 20, UiKit.GOLD))
	if not p.get("withdraw_enabled", true):
		right.add_child(UiKit.para("Withdrawals are paused by the operators.", 15, UiKit.DANGER))
	else:
		var addr := LineEdit.new()
		addr.placeholder_text = "TON address (EQ... / UQ...)"
		addr.custom_minimum_size.y = 44
		right.add_child(addr)
		var wr := HBoxContainer.new()
		right.add_child(wr)
		var amt := LineEdit.new()
		amt.placeholder_text = "Amount in TON"
		amt.virtual_keyboard_type = LineEdit.KEYBOARD_TYPE_NUMBER_DECIMAL
		amt.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		amt.custom_minimum_size.y = 44
		wr.add_child(amt)
		wr.add_child(UiKit.button("Withdraw", func():
			var nano := int(round(amt.text.strip_edges().to_float() * NANO))
			var to := addr.text.strip_edges()
			if nano <= 0 or to.length() < 40:
				Game.toast("Enter an address and an amount", UiKit.DANGER)
				return
			if await Popups.money_confirm("Withdraw %s?" % ton(nano), "To %s…%s. Network fee %s." % [to.substr(0, 6), to.substr(to.length() - 6), ton(p.get("withdraw_fee", 0))], "Withdraw"):
				_act("/v1/ton/withdraw", {"to_address": to, "amount": nano, "request_id": Api.request_id()}, "Withdrawal requested"), 44, "blue"))
		right.add_child(UiKit.para("Minimum %s, network fee %s. Large withdrawals are reviewed by an operator before sending." % [ton(p.get("withdraw_min", 0)), ton(p.get("withdraw_fee", 0))], 13, UiKit.MUTED))
	var hist := UiKit.inset(8)
	hist.size_flags_vertical = Control.SIZE_EXPAND_FILL
	right.add_child(hist)
	var scroll := ScrollContainer.new()
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	hist.add_child(scroll)
	var list := VBoxContainer.new()
	list.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	list.add_theme_constant_override("separation", 2)
	scroll.add_child(list)
	for x in w.get("withdrawals", []):
		var col: Color = {"sent": UiKit.GOOD, "failed": UiKit.DANGER, "rejected": UiKit.DANGER}.get(str(x.status), UiKit.TEXT)
		list.add_child(UiKit.label("↗ %s  →  %s…  %s" % [ton(x.amount), str(x.to_address).substr(0, 8), str(x.status)], 14, col, true, 3))
	for e in w.get("ledger", []):
		var d := int(e.delta)
		list.add_child(UiKit.label("%s  %s%s  %s" % [str(e.created_at).substr(5, 11).replace("T", " "), "+" if d > 0 else "", ton(d), str(e.reason)], 14, UiKit.GOOD if d > 0 else UiKit.TEXT, true, 3))
	if list.get_child_count() == 0:
		list.add_child(UiKit.label("No transactions yet", 14, UiKit.MUTED, true, 3))

func _copy_row(label: String, value: String) -> Control:
	var row := HBoxContainer.new()
	var l := UiKit.label(label, 15, UiKit.MUTED)
	l.custom_minimum_size.x = 70
	row.add_child(l)
	var v := LineEdit.new()
	v.text = value
	v.editable = false
	v.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	v.custom_minimum_size.y = 42
	v.add_theme_font_size_override("font_size", 15)
	row.add_child(v)
	row.add_child(UiKit.button("Copy", func():
		DisplayServer.clipboard_set(value)
		Game.toast(label + " copied", UiKit.GOOD), 42, "gray"))
	return row


class SellForm extends Modal:
	## Currency and price for a new listing.
	signal submit(currency: String, amount: int)

	func _init(t: Dictionary, currencies: Array, p: Dictionary) -> void:
		super("Sell NFT #%d" % int(t.id))
		max_size = Vector2(560, 420)
		scrollable = false
		fit_content = true
		var cur := OptionButton.new()
		cur.custom_minimum_size.y = 46
		for c in currencies:
			cur.add_item("TON" if c == "TON" else "Gold")
		body.add_child(cur)
		var price := LineEdit.new()
		price.placeholder_text = "Price"
		price.custom_minimum_size.y = 48
		price.virtual_keyboard_type = LineEdit.KEYBOARD_TYPE_NUMBER_DECIMAL
		body.add_child(price)
		var fee := float(p.get("fee_bps", 250)) / 100.0
		var min_ton := ("%.3f" % (float(p.get("min_price_ton", 0)) / 1e9)).rstrip("0").rstrip(".")
		body.add_child(UiKit.para("Marketplace fee %.1f%%. Minimum price %s TON or %d gold." % [fee, min_ton, int(p.get("min_price_gold", 0))], 15, UiKit.MUTED))
		var b := UiKit.button("List for sale", func():
			var currency: String = currencies[cur.selected]
			var v := price.text.strip_edges().to_float()
			var amount := int(round(v * 1e9)) if currency == "TON" else int(v)
			if amount <= 0:
				Game.toast("Enter a price", UiKit.DANGER)
				return
			close()
			submit.emit(currency, amount), 54, "orange")
		body.add_child(b)
