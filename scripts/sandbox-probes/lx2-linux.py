import os, socket, json
w = os.environ.get("PROBE_DIR", "/tmp/lxwork"); os.makedirs(w, exist_ok=True)
r = {}
for k in ("a.sock", "f.txt"):
    p = os.path.join(w, k)
    if os.path.exists(p): os.unlink(p)
try:
    s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM); s.bind(os.path.join(w, "a.sock")); s.listen(1); r["unix_bind"] = "ok"
except Exception as e: r["unix_bind"] = "FAIL:%s" % type(e).__name__
try:
    t = socket.socket(); t.bind(("127.0.0.1", 0)); t.listen(1); r["tcp_loopback_bind"] = "ok:%d" % t.getsockname()[1]
except Exception as e: r["tcp_loopback_bind"] = "FAIL:%s" % type(e).__name__
try:
    open(os.path.join(w, "f.txt"), "w").write("x"); r["write_work"] = "ok"
except Exception as e: r["write_work"] = "FAIL:%s" % type(e).__name__
try:
    socket.create_connection(("<worker-host>", 8580), timeout=4); r["lan_connect"] = "OPEN(可达)"
except Exception as e: r["lan_connect"] = "%s" % type(e).__name__
try:
    socket.create_connection(("1.1.1.1", 443), timeout=4); r["wan_connect"] = "OPEN(可达)"
except Exception as e: r["wan_connect"] = "%s" % type(e).__name__
print(json.dumps(r))
