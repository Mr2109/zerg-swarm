import os, socket, json, sys
# 出网目标可用环境覆盖（默认与历史回执一致 = 1.1.1.1:443）。
# 宿主本身没有出网时（实测：X3 基线对公网即 Timeout）这一格**没有区分度** ⇒ 必须改用局域网目标，
# 否则「出网被拦」测出来是假的（§10.7 结论 2 的测量学教训）。目标写进输出，让证据自描述。
EG_HOST = os.environ.get("PROBE_EGRESS_HOST", "1.1.1.1")
EG_PORT = int(os.environ.get("PROBE_EGRESS_PORT", "443"))
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
    socket.create_connection((EG_HOST, EG_PORT), timeout=3); r["egress"] = "OPEN"
except Exception as e: r["egress"] = "BLOCKED:%s" % type(e).__name__
r["egress_target"] = "%s:%d" % (EG_HOST, EG_PORT)
print(json.dumps(r))
