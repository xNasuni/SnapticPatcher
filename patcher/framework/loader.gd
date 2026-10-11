extends Node

const MANIFEST := "res://snaptic/mods.json"
const VERSION := "0.1"

var mods: Array = []
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
	var loaded: Array[String] = []
	for m in _manifest():
		var id: String = m.name
		if m.main:
			var script = load("res://snaptic/mods/%s/main.gd" % id)
			if script == null:
				push_error("[snaptic] %s: main.gd failed to load" % id)
				continue
			var node: Node = script.new()
			node.name = id
			node.set_meta("mod_dir", "res://snaptic/mods/%s/" % id)
			get_tree().root.add_child.call_deferred(node)
		loaded.append(id)
		mods.append(m)
	_log("framework %s, mods: %s" % [VERSION, ", ".join(loaded)])
	_banner(", ".join(loaded) if loaded else "none")


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


func texture(path: String) -> ImageTexture:
	var img := Image.load_from_file(path)
	return ImageTexture.create_from_image(img) if img else null


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
