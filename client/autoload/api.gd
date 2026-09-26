extends Node
## Minimal async REST client for the gateway. Every call returns a
## Dictionary: { ok: bool, status: int, data: Variant, error: String }.
##
##   var r := await Api.get_json("/v1/me")
##   if r.ok: print(r.data)

var token := ""

func _request(method: int, path: String, body = null) -> Dictionary:
	var http := HTTPRequest.new()
	http.timeout = 15.0
	add_child(http)
	var headers := PackedStringArray(["Content-Type: application/json", "Accept: application/json"])
	if token != "":
		headers.append("Authorization: Bearer " + token)
	var payload := "" if body == null else JSON.stringify(body)
	var err := http.request(Cfg.url(path), headers, method, payload)
	if err != OK:
		http.queue_free()
		return {"ok": false, "status": 0, "data": null, "error": "request failed (%d)" % err}
	var res: Array = await http.request_completed
	http.queue_free()
	var status: int = res[1]
	var text: String = (res[3] as PackedByteArray).get_string_from_utf8()
	var data = JSON.parse_string(text) if text != "" else null
	if res[0] != HTTPRequest.RESULT_SUCCESS:
		return {"ok": false, "status": 0, "data": null, "error": "network error"}
	if status >= 200 and status < 300:
		return {"ok": true, "status": status, "data": data, "error": ""}
	var msg := "HTTP %d" % status
	if data is Dictionary and data.has("error"):
		msg = str(data.error.get("message", msg))
	return {"ok": false, "status": status, "data": data, "error": msg}

func get_json(path: String) -> Dictionary:
	return await _request(HTTPClient.METHOD_GET, path)

func post_json(path: String, body = {}) -> Dictionary:
	return await _request(HTTPClient.METHOD_POST, path, body)

func put_json(path: String, body) -> Dictionary:
	return await _request(HTTPClient.METHOD_PUT, path, body)

## A random id for idempotent actions (enhance, salvage).
func request_id() -> String:
	var bytes := Crypto.new().generate_random_bytes(12)
	return bytes.hex_encode()
