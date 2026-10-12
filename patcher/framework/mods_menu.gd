extends "res://snaptic/menu_panel.gd"

const SIZE := Vector2(560, 300)
const ACCENT := Color(0.5882353, 0.5882353, 1)

var _blank: Texture2D
var _entries: Array = []
var _name: Label
var _meta: Label
var _desc: Label
var _settings: Button
var _current := 0


func setup(entries: Array) -> void:
	_entries = entries
	build("Mods", SIZE)

	var image := Image.create(16, 16, false, Image.FORMAT_RGBA8)
	image.fill(Color(0.23, 0.23, 0.36))
	_blank = ImageTexture.create_from_image(image)

	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 8)
	content.add_child(row)

	var list := _column(row, 3.0)
	var detail := _column(row, 7.0)

	var header := HBoxContainer.new()
	detail.add_child(header)
	_name = Label.new()
	_name.add_theme_font_size_override("font_size", 18)
	_name.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	header.add_child(_name)
	_settings = Button.new()
	_settings.text = "Settings"
	_settings.disabled = true
	_settings.pressed.connect(func() -> void: SnapticBase.open_settings(_entries[_current].id, self))
	header.add_child(style_button(_settings))
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
		button.pressed.connect(_select.bind(i))
		list.add_child(style_button(button))

	add_back()
	if list.get_child_count() > 0:
		list.get_child(0).button_pressed = true
		_select(0)


func _column(row: Control, ratio: float) -> VBoxContainer:
	var scroll := ScrollContainer.new()
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	scroll.size_flags_stretch_ratio = ratio
	row.add_child(scroll)
	var column := VBoxContainer.new()
	column.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	column.add_theme_constant_override("separation", 4)
	scroll.add_child(column)
	return column


func _select(i: int) -> void:
	_current = i
	var e: Dictionary = _entries[i]
	_settings.disabled = not SnapticBase.has_settings(e.id)
	_settings.tooltip_text = "no settings for this mod" if _settings.disabled else "open settings"
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
