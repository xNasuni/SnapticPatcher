extends Control

signal closed

const THEME := "res://ui/_shared/themes/menu_screen_theme.tres"
const CLICK_BUTTON := "res://levels/menu_screen_background/prefabs/click_button/click_button.gd"
const BACK_GROUP := &"back_button"
const SIZE := Vector2(560, 300)
const FADE := 0.2
const ACCENT := Color(0.5882353, 0.5882353, 1)

var _menu: Node
var _menu_tween: Tween
var _tween: Tween
var _blank: Texture2D
var _entries: Array = []
var _name: Label
var _meta: Label
var _desc: Label


## entries: [{dir, name, version, authors, description}]
func setup(entries: Array) -> void:
	_entries = entries
	theme = load(THEME)
	set_anchors_preset(PRESET_FULL_RECT)
	mouse_filter = MOUSE_FILTER_IGNORE
	z_index = 2
	visible = false

	var image := Image.create(16, 16, false, Image.FORMAT_RGBA8)
	image.fill(Color(0.23, 0.23, 0.36))
	_blank = ImageTexture.create_from_image(image)

	var center := CenterContainer.new()
	center.set_anchors_preset(PRESET_FULL_RECT)
	center.mouse_filter = MOUSE_FILTER_IGNORE
	add_child(center)

	var frame := VBoxContainer.new()
	frame.add_theme_constant_override("separation", 0)
	center.add_child(frame)

	var tabs := TabContainer.new()
	tabs.custom_minimum_size = SIZE
	tabs.tab_alignment = TabBar.ALIGNMENT_CENTER
	frame.add_child(tabs)

	var page := MarginContainer.new()
	page.name = "Mods"
	for side in ["left", "top", "right", "bottom"]:
		page.add_theme_constant_override("margin_" + side, 8)
	tabs.add_child(page)

	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 8)
	page.add_child(row)

	var list_scroll := ScrollContainer.new()
	list_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	list_scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	list_scroll.size_flags_stretch_ratio = 3.0
	row.add_child(list_scroll)

	var list := VBoxContainer.new()
	list.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	list.add_theme_constant_override("separation", 4)
	list_scroll.add_child(list)

	var detail_scroll := ScrollContainer.new()
	detail_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	detail_scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	detail_scroll.size_flags_stretch_ratio = 7.0
	row.add_child(detail_scroll)

	var detail := VBoxContainer.new()
	detail.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	detail_scroll.add_child(detail)

	var header := HBoxContainer.new()
	detail.add_child(header)
	_name = Label.new()
	_name.add_theme_font_size_override("font_size", 18)
	_name.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	header.add_child(_name)
	var settings := Button.new()
	settings.text = "Settings"
	settings.disabled = true  # until mods can register settings
	header.add_child(settings)
	_meta = Label.new()
	_meta.add_theme_color_override("font_color", ACCENT)
	detail.add_child(_meta)
	_desc = Label.new()
	_desc.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_desc.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	detail.add_child(_desc)

	var group := ButtonGroup.new()
	for i in _entries.size():
		var e: Dictionary = _entries[i]
		var button := Button.new()
		button.toggle_mode = true
		button.button_group = group
		button.text = e.name
		button.icon = _icon(e.dir)
		button.expand_icon = true
		button.clip_text = true
		button.alignment = HORIZONTAL_ALIGNMENT_LEFT
		button.add_theme_constant_override("icon_max_width", 24)
		_style(button)
		button.pressed.connect(_select.bind(i))
		list.add_child(button)

	var actions := HBoxContainer.new()
	actions.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	frame.add_child(actions)
	var back := Button.new()
	back.text = "Back"
	back.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	back.add_to_group(BACK_GROUP)
	_style(back)
	back.pressed.connect(close)
	actions.add_child(back)

	if list.get_child_count() > 0:
		list.get_child(0).button_pressed = true
		_select(0)


func open(menu: Node) -> void:
	_menu = menu
	var content: Control = menu.get("everything_else")
	var duration: float = menu.get("tween_duration")
	if content:
		_menu_tween = Helpers.fade_control(content, _menu_tween, false, duration, MOUSE_FILTER_IGNORE)
	_tween = Helpers.fade_control(self, _tween, true, duration, MOUSE_FILTER_IGNORE)


func close() -> void:
	var duration := FADE
	if is_instance_valid(_menu):
		duration = _menu.get("tween_duration")
		var content: Control = _menu.get("everything_else")
		if content:
			_menu_tween = Helpers.fade_control(content, _menu_tween, true, duration, MOUSE_FILTER_IGNORE)
	_tween = Helpers.fade_control(self, _tween, false, duration)
	closed.emit()


func _select(i: int) -> void:
	var e: Dictionary = _entries[i]
	_name.text = e.name
	var meta: String = "v" + e.version
	if not e.authors.is_empty():
		meta += "  by " + ", ".join(e.authors)
	_meta.text = meta
	_desc.text = e.description if e.description != "" else "no description"


func _icon(dir: String) -> Texture2D:
	var path := "res://snaptic/mods/%s/mod.png" % dir
	if dir != "" and FileAccess.file_exists(path):
		var image := Image.new()
		if image.load_png_from_buffer(FileAccess.get_file_as_bytes(path)) == OK:
			return ImageTexture.create_from_image(image)
	return _blank


func _style(button: Button) -> void:
	button.set_script(load(CLICK_BUTTON))
	button.pressed.connect(AudioManager.play_sound_effect.bind(SFX.Name.MENU_SELECT))
	button.mouse_entered.connect(AudioManager.play_sound_effect.bind(SFX.Name.MENU_HOVER))
