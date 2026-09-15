import mmap, platform, json, ctypes
r = {}
mach = platform.machine()
code = bytes.fromhex("40058052c0035fd6") if mach in ("arm64", "aarch64") else bytes.fromhex("b82a000000c3")
try:
    m = mmap.mmap(-1, 4096, prot=mmap.PROT_READ | mmap.PROT_WRITE | mmap.PROT_EXEC); m.close(); r["mmap_RWX"] = "ok"
except Exception as e: r["mmap_RWX"] = "FAIL:%s" % type(e).__name__
try:
    m = mmap.mmap(-1, 4096, prot=mmap.PROT_READ | mmap.PROT_WRITE | mmap.PROT_EXEC)
    m.write(code)
    fn = ctypes.CFUNCTYPE(ctypes.c_int)(ctypes.addressof(ctypes.c_char.from_buffer(m)))
    r["call_jit_code"] = "ok:%d" % fn(); m.close()
except Exception as e: r["call_jit_code"] = "FAIL:%s" % type(e).__name__
try:
    m = mmap.mmap(-1, 4096, prot=mmap.PROT_READ | mmap.PROT_WRITE); m.write(code)
    libc = ctypes.CDLL(None)
    addr = ctypes.addressof(ctypes.c_char.from_buffer(m))
    rc = libc.mprotect(ctypes.c_void_p(addr), ctypes.c_size_t(4096), 5)
    r["mprotect_to_RX"] = "ok" if rc == 0 else "FAIL:rc=%d" % rc
except Exception as e: r["mprotect_to_RX"] = "FAIL:%s" % type(e).__name__
print(json.dumps(r))
