import os, socket, json, sys
w = os.environ.get("PROBE_DIR", "/tmp/ipc-test/work"); os.makedirs(w, exist_ok=True)
r = {}
try:
    sp = os.path.join(w, "a.sock")
    if os.path.exists(sp): os.unlink(sp)   # 空间内临时区必须先清干净，否则 EADDRINUSE
    s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM); s.bind(sp); s.listen(1)
    r["unix_bind"] = "ok"
except Exception as e: r["unix_bind"] = "FAIL:%s:%s" % (type(e).__name__, e.errno)
try:
    t = socket.socket(); t.bind(("127.0.0.1", 0)); t.listen(1); r["tcp_bind"] = "ok:%d" % t.getsockname()[1]
except Exception as e: r["tcp_bind"] = "FAIL:%s" % type(e).__name__
try:
    open(os.path.join(w, "f.txt"), "w").write("x"); r["write_work"] = "ok"
except Exception as e: r["write_work"] = "FAIL:%s" % type(e).__name__
try:
    socket.create_connection(("1.1.1.1", 443), timeout=3); r["egress"] = "OPEN"
except Exception as e: r["egress"] = "BLOCKED:%s" % type(e).__name__
print(json.dumps(r))
