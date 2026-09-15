import os, socket, json
# 出网目标可用环境覆盖（默认与历史回执一致 = 局域网 <worker-host>:8580 / 公网 1.1.1.1:443）。
# 判据 7 的门禁只用**局域网**那一格：宿主本身无出网时公网格没有区分度（§10.7 结论 2）。目标写进输出。
LAN_HOST = os.environ.get("PROBE_LAN_HOST", "<worker-host>")
LAN_PORT = int(os.environ.get("PROBE_LAN_PORT", "8580"))
WAN_HOST = os.environ.get("PROBE_WAN_HOST", "1.1.1.1")
WAN_PORT = int(os.environ.get("PROBE_WAN_PORT", "443"))
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
    socket.create_connection((LAN_HOST, LAN_PORT), timeout=4); r["lan_connect"] = "OPEN(可达)"
except Exception as e: r["lan_connect"] = "%s" % type(e).__name__
try:
    socket.create_connection((WAN_HOST, WAN_PORT), timeout=4); r["wan_connect"] = "OPEN(可达)"
except Exception as e: r["wan_connect"] = "%s" % type(e).__name__
r["lan_target"] = "%s:%d" % (LAN_HOST, LAN_PORT)
r["wan_target"] = "%s:%d" % (WAN_HOST, WAN_PORT)
print(json.dumps(r))
