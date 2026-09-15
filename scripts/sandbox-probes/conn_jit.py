import json, platform, socket, sys, threading, ctypes, mmap
r = {}
# ① loopback 连通性：bind 之后自己连自己并回一个字节
try:
    srv = socket.socket(); srv.bind(("127.0.0.1", 0)); srv.listen(1)
    port = srv.getsockname()[1]
    def acc():
        c, _ = srv.accept(); c.sendall(b"ok"); c.close()
    threading.Thread(target=acc, daemon=True).start()
    c = socket.create_connection(("127.0.0.1", port), timeout=5)
    r["loopback_connect"] = c.recv(8).decode(); c.close(); srv.close()
except Exception as e:
    r["loopback_connect"] = "FAIL:%s" % type(e).__name__
# ② W^X JIT：RW→写码→mprotect RX→调用
code = bytes.fromhex("40058052c0035fd6") if platform.machine() in ("arm64", "aarch64") else bytes.fromhex("b82a000000c3")
try:
    m = mmap.mmap(-1, 4096, prot=mmap.PROT_READ | mmap.PROT_WRITE)
    m.write(code)
    addr = ctypes.addressof(ctypes.c_char.from_buffer(m))
    rc = ctypes.CDLL(None).mprotect(ctypes.c_void_p(addr), ctypes.c_size_t(4096), 5)
    r["wx_jit_call"] = "ok:%d" % ctypes.CFUNCTYPE(ctypes.c_int)(addr)() if rc == 0 else "FAIL:mprotect"
except Exception as e:
    r["wx_jit_call"] = "FAIL:%s" % type(e).__name__
print(json.dumps(r))
