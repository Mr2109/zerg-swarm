import hashlib, json, os, sys, time
ALIGN = 16384
def sha(p):
    h = hashlib.sha256()
    with open(p, "rb") as f:
        for b in iter(lambda: f.read(1 << 20), b""): h.update(b)
    return h.hexdigest()
def pack(src, egg):
    entries = []
    out = open(egg, "wb")
    out.write(b"\0" * ALIGN)  # 头部留给索引
    for root, dirs, files in os.walk(src):
        dirs.sort()
        for fn in sorted(files):
            fp = os.path.join(root, fn)
            rel = os.path.relpath(fp, src)
            data_off = out.tell()
            with open(fp, "rb") as f:
                for b in iter(lambda: f.read(1 << 20), b""): out.write(b)
            size = out.tell() - data_off
            pad = (-size) % ALIGN
            out.write(b"\0" * pad)
            entries.append({"path": rel, "off": data_off, "size": size, "sha256": sha(fp),
                            "mode": os.stat(fp).st_mode & 0o777})
    idx = json.dumps({"v": 1, "align": ALIGN, "entries": entries}, ensure_ascii=False).encode()
    out.seek(0); out.write(idx)
    out.seek(ALIGN - 8); out.write(len(idx).to_bytes(8, "little"))   # 长度放头块末尾，读端才自洽
    out.close()
    return entries
def rebuild(egg, dst, verify=True):
    raw = open(egg, "rb").read(ALIGN)
    n = int.from_bytes(raw[-8:], "little")
    assert 0 < n <= ALIGN - 8, "索引长度异常"
    idx = json.loads(raw[:n])
    bad = []
    f = open(egg, "rb")
    for e in idx["entries"]:
        tgt = os.path.join(dst, e["path"]); os.makedirs(os.path.dirname(tgt), exist_ok=True)
        f.seek(e["off"]); data = f.read(e["size"])
        open(tgt, "wb").write(data)
        if verify:
            h = hashlib.sha256(data).hexdigest()
            if h != e["sha256"]: bad.append(e["path"])
    f.close()
    return idx, bad
