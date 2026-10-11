extends Node

const MANIFEST := "res://snaptic/mods.json"
const VERSION := "0.1.0"
const MENU_SCENE := "res://ui/menu_screen/menu_screen_ui.tscn"
const MENU_LIST := "Everything Else/TitleMarginContainer/VBoxContainer/MenuButtons"
const MENU_TEMPLATE := "Options"
const DEFAULT_ANCHOR := "Play"

## Loaded mods in load order: {dir, id, name, version, dependencies, main}.
var mods: Array = []
var _menu_entries: Array[Dictionary] = []
var _scene_hooks: Dictionary = {}
var _menu: Node
var _menu_queued := false
var _mods_menu: Control
var _log_file: FileAccess
var _mutex := Mutex.new()

class Tap extends Logger:
	var sink: Callable

	func _log_message(message: String, error: bool) -> void:
		sink.call(("[err] " if error else "") + message.strip_edges(false, true))

	func _log_error(function: String, file: String, line: int, code: String, rationale: String, _editor_notify: bool, _error_type: int, _backtraces: Array) -> void:
		sink.call("[err] %s (%s:%d, %s)" % [rationale if rationale else code, file, line, function])


func _ready() -> void:
	_log_file = FileAccess.open("user://snaptic.log", FileAccess.WRITE)
	var tap := Tap.new()
	tap.sink = _write
	OS.add_logger(tap)
	_log("loader started")
	if FileAccess.file_exists(OS.get_executable_path().get_base_dir().path_join("snaptic_console")):
		_open_console()
	get_tree().node_added.connect(_on_node_added)
	on_scene(MENU_SCENE, _on_menu_scene)
	add_menu_button("Mods", _show_mods)
	var loaded: Array[String] = []
	for m in _manifest():
		var dir: String = m.dir
		var id: String = m.get("id", dir)
		for ext in m.get("extensions", []):
			_extend("res://snaptic/mods/%s/%s" % [dir, ext], id)
		if m.main:
			var script = load("res://snaptic/mods/%s/main.gd" % dir)
			if script == null:
				push_error("[snaptic] %s: main.gd failed to load" % id)
				continue
			var node: Node = script.new()
			node.name = id
			var info: Dictionary = m.duplicate()
			info["dir"] = "res://snaptic/mods/%s/" % dir
			node.set_meta("mod", info)
			get_tree().root.add_child.call_deferred(node)
		loaded.append(id)
		mods.append(m)
	_log("snapticbase %s, mods: %s" % [VERSION, ", ".join(loaded)])
	_banner(", ".join(loaded) if loaded else "none")


## Adds a main menu button below Play.
func add_menu_button(label: String, callback: Callable) -> void:
	_add_menu_entry(label, callback, DEFAULT_ANCHOR, false)


## Adds a main menu button directly below `anchor` (a native button's name or text,
## or another mod's button label). Buttons sharing an anchor are sorted alphabetically.
func add_menu_button_below(label: String, callback: Callable, anchor: String) -> void:
	_add_menu_entry(label, callback, anchor, false)


## Adds a main menu button directly above `anchor`.
func add_menu_button_above(label: String, callback: Callable, anchor: String) -> void:
	_add_menu_entry(label, callback, anchor, true)


## Calls `callback(root)` for every instance of the scene at `path`, once it is
## ready, including instances that already exist. Edit the nodes in place.
func on_scene(path: String, callback: Callable) -> void:
	if not _scene_hooks.has(path):
		_scene_hooks[path] = []
	_scene_hooks[path].append(callback)
	var pending: Array[Node] = [get_tree().root]
	while not pending.is_empty():
		var node: Node = pending.pop_back()
		if node.scene_file_path == path:
			_run_hook.call_deferred(callback, node)
		pending.append_array(node.get_children())


func _run_hook(callback: Callable, node: Node) -> void:
	if not is_instance_valid(node) or not callback.is_valid():
		return
	var done: Array = node.get_meta("snaptic_hooks", [])
	if callback in done:
		return
	done.append(callback)
	node.set_meta("snaptic_hooks", done)
	callback.call(node)


func _extend(path: String, id: String) -> void:
	var child: GDScript = load(path)
	if child == null:
		push_error("[snaptic] %s: %s failed to load" % [id, path])
		return
	child.reload()
	var base := child.get_base_script()
	if base == null or base.resource_path.is_empty():
		push_error("[snaptic] %s: %s must extend a script by path" % [id, path])
		return
	var target := base.resource_path
	child.take_over_path(target)
	_log("%s extends %s" % [id, target])


func get_mod(id: String) -> Dictionary:
	for m in mods:
		if m.get("id", m.dir) == id:
			return m
	return {}


func _on_node_added(node: Node) -> void:
	for callback in _scene_hooks.get(node.scene_file_path, []):
		_run_hook.call_deferred(callback, node)


func _on_menu_scene(node: Node) -> void:
	_menu = node
	_queue_menu_refresh()


func _queue_menu_refresh() -> void:
	if not _menu_queued:
		_menu_queued = true
		_refresh_menu.call_deferred()


func _refresh_menu() -> void:
	_menu_queued = false
	if not is_instance_valid(_menu):
		return
	var list := _menu.get_node_or_null(MENU_LIST)
	var template := list.get_node_or_null(MENU_TEMPLATE) if list else null
	if template == null:
		push_warning("[snaptic] menu layout not recognised, skipping buttons")
		return
	for entry in _menu_entries:
		if is_instance_valid(entry.get("button")):
			continue
		var button: Button = template.duplicate(Node.DUPLICATE_SCRIPTS)
		button.name = entry.label
		button.text = entry.label
		button.pressed.connect(AudioManager.play_sound_effect.bind(SFX.Name.MENU_SELECT))
		button.mouse_entered.connect(AudioManager.play_sound_effect.bind(SFX.Name.MENU_HOVER))
		button.pressed.connect(entry.callback)
		list.add_child(button)
		entry["button"] = button
	var order := _menu_order(list)
	for i in order.size():
		list.move_child(order[i], i)


func _add_menu_entry(label: String, callback: Callable, anchor: String, above: bool) -> void:
	_menu_entries.append({"label": label, "callback": callback, "anchor": anchor, "above": above})
	_queue_menu_refresh()


func _menu_order(list: Node) -> Array:
	var native: Array[Node] = []
	var own: Dictionary = {}
	for entry in _menu_entries:
		own[entry.button] = entry
	for child in list.get_children():
		if not own.has(child):
			native.append(child)

	var known := {}
	for n in native:
		known[n.name.to_lower()] = true
		known[str(n.get("text")).to_lower()] = true
	for entry in _menu_entries:
		known[entry.label.to_lower()] = true
	# unknown anchors fall back to below the default
	for entry in _menu_entries:
		if not known.has(entry.anchor.to_lower()):
			entry.anchor = DEFAULT_ANCHOR
			entry.above = false

	var order: Array = []
	var placed := {}
	for n in native:
		_place(n, [n.name, n.get("text")], order, placed)
	for entry in _menu_entries:
		if not placed.has(entry):
			order.append(entry.button)
	return order


func _place(node: Node, names: Array, order: Array, placed: Dictionary) -> void:
	var lowered := names.map(func(n): return str(n).to_lower())
	_place_group(true, lowered, order, placed)
	order.append(node)
	_place_group(false, lowered, order, placed)


func _place_group(above: bool, anchors: Array, order: Array, placed: Dictionary) -> void:
	var group: Array = []
	for entry in _menu_entries:
		if not placed.has(entry) and entry.above == above and entry.anchor.to_lower() in anchors:
			group.append(entry)
	group.sort_custom(func(a, b): return a.label.to_lower() < b.label.to_lower())
	for entry in group:
		if not placed.has(entry):
			placed[entry] = true
			_place(entry.button, [entry.label], order, placed)


func _show_mods() -> void:
	if not is_instance_valid(_mods_menu):
		var entries: Array = [{
			"dir": "", "name": "Snaptic Base", "version": VERSION,
			"authors": ["xNasuni"], "description": "the modding framework",
		}]
		for m in mods:
			entries.append({
				"dir": m.dir, "name": m.get("name", m.get("id", m.dir)),
				"version": m.get("version", ""), "authors": m.get("authors", []),
				"description": m.get("description", ""),
			})
		_mods_menu = load("res://snaptic/mods_menu.gd").new()
		_mods_menu.setup(entries)
		SettingsMenu.instance.get_parent().add_child(_mods_menu)
	_mods_menu.open(_menu)


func _log(msg: String) -> void:
	print("[snaptic] ", msg)


func _write(line: String) -> void:
	_mutex.lock()
	if _log_file:
		_log_file.store_line(line)
		_log_file.flush()
	_mutex.unlock()


func _open_console() -> void:
	var path := ProjectSettings.globalize_path("user://snaptic.log").replace("/", "\\")
	var cmd := "$Host.UI.RawUI.WindowTitle='snaptic console'; Get-Content -Wait -Tail 500 -LiteralPath '%s'" % path
	OS.create_process("powershell.exe", ["-NoLogo", "-NoExit", "-Command", cmd])


## Loads a file shipped in a mod folder at runtime: images become textures, ogg,
## mp3 and 16 bit pcm wav become audio streams, txt/json/cfg become strings and
## anything else raw bytes. Returns null if it can't be read.
func load_asset(path: String) -> Variant:
	if not FileAccess.file_exists(path):
		push_error("[snaptic] asset not found: " + path)
		return null
	var bytes := FileAccess.get_file_as_bytes(path)
	match path.get_extension().to_lower():
		"png", "jpg", "jpeg", "webp":
			var image := Image.new()
			var err := ERR_FILE_UNRECOGNIZED
			match path.get_extension().to_lower():
				"png":
					err = image.load_png_from_buffer(bytes)
				"webp":
					err = image.load_webp_from_buffer(bytes)
				_:
					err = image.load_jpg_from_buffer(bytes)
			return ImageTexture.create_from_image(image) if err == OK else null
		"ogg":
			return AudioStreamOggVorbis.load_from_buffer(bytes)
		"mp3":
			var mp3 := AudioStreamMP3.new()
			mp3.data = bytes
			return mp3
		"wav":
			return _wav(bytes)
		"txt", "json", "cfg", "md":
			return bytes.get_string_from_utf8()
	return bytes


func _wav(bytes: PackedByteArray) -> AudioStreamWAV:
	var stream := AudioStreamWAV.new()
	var pos := 12
	var found := false
	while pos + 8 <= bytes.size():
		var id := bytes.slice(pos, pos + 4).get_string_from_ascii()
		var size := bytes.decode_u32(pos + 4)
		if id == "fmt ":
			if bytes.decode_u16(pos + 8) != 1 or bytes.decode_u16(pos + 22) != 16:
				push_error("[snaptic] wav must be 16 bit pcm")
				return null
			stream.stereo = bytes.decode_u16(pos + 10) == 2
			stream.mix_rate = bytes.decode_u32(pos + 12)
			stream.format = AudioStreamWAV.FORMAT_16_BITS
		elif id == "data":
			stream.data = bytes.slice(pos + 8, pos + 8 + size)
			found = true
		pos += 8 + size + (size & 1)
	return stream if found else null


func _manifest() -> Array:
	if not FileAccess.file_exists(MANIFEST):
		return []
	var data = JSON.parse_string(FileAccess.get_file_as_string(MANIFEST))
	return data if data is Array else []


func _banner(mods_text: String) -> void:
	var layer := CanvasLayer.new()
	layer.layer = 128
	var panel := PanelContainer.new()
	var box := StyleBoxFlat.new()
	box.bg_color = Color(0.05, 0.05, 0.07, 0.55)
	box.set_corner_radius_all(6)
	box.content_margin_left = 12
	box.content_margin_right = 12
	box.content_margin_top = 6
	box.content_margin_bottom = 6
	panel.add_theme_stylebox_override("panel", box)
	panel.position = Vector2(16, 16)
	panel.modulate.a = 0.0
	panel.mouse_filter = Control.MOUSE_FILTER_IGNORE

	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 6)
	panel.add_child(row)
	row.add_child(_label("snaptic patcher |", Color(0.94, 0.94, 0.96)))
	row.add_child(_label(mods_text, Color(0.65, 0.89, 0.75)))

	layer.add_child(panel)
	add_child(layer)
	var tween := create_tween()
	tween.tween_property(panel, "modulate:a", 1.0, 0.4)
	tween.tween_interval(3.5)
	tween.tween_property(panel, "modulate:a", 0.0, 0.8)
	tween.tween_callback(layer.queue_free)


func _label(text: String, color: Color) -> Label:
	var label := Label.new()
	label.text = text
	label.add_theme_font_size_override("font_size", 14)
	label.add_theme_color_override("font_color", color)
	return label
