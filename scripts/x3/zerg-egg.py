#!/usr/bin/env python3
"""zerg-egg —— 卵的单文件封装（`*.egg`）工具：pack / info / verify。

格式 v1（**读写同源**：索引的生成为解析的唯一实现，见 `build_index`/`parse_index`）：

    ┌── 固定前缀 32 字节 ──┐┌──── 索引 JSON（UTF-8，长度可变）────┐┌─ 补零到 ALIGN ─┐
    │ magic(4) ver(4)      ││ ...                                  ││                │
    │ align(u64) len(u64)  ││                                      ││                │
    └──────────────────────┘└──────────────────────────────────────┘└────────────────┘
    ┌── 数据段（每段按索引里的 align 对齐，段间补零）─────────────────────────┐
    │ file1 字节 │ pad │ file2 字节 │ pad │ ... │ （预留）签名块              │
    └─────────────────────────────────────────────────────────────────────────┘
    ┌── 数据段（每段按 ALIGN 对齐，段间补零）──────────────────────────────┐
    │ file1 字节 │ pad │ file2 字节 │ pad │ ... │ （预留）签名块            │
    └─────────────────────────────────────────────────────────────────────┘

索引 JSON 字段：
    magic     固定 "ZEGG"（识别用）
    version   格式版本（当前 1）
    align     页对齐（**按平台取**：Linux 4096 / Apple Silicon 16384）
    tool      {name, version}  产出它的工具
    created_at ISO8601 UTC
    entries   [{path, off, size, sha256, mode}]   path 为相对路径，禁止 ".."

规则（规格的一部分）：
    · align 必须是 2 的幂且 ≥ 4096；**读端按索引里记录的 align 校验**，不硬编码；
    · 每个数据段的偏移必须对齐到 align；段内容 sha256 必须与索引一致，否则 verify 失败；
    · **固定 32 字节前缀**给出（magic, version, align, index_len）⇒ 读端先读前缀即知 align 与索引长度，
      不必先知道 align（这是"读端无法先验知道 align"这一死结的正解）；
    · 签名块位置预留（v1 不用；未来放 index["sig"] 及其覆盖范围）。

用法：
    zerg-egg.py pack   <源目录> <输出.egg> [--align N]
    zerg-egg.py info   <file.egg>
    zerg-egg.py verify <file.egg> [--rebuild-into DIR]
    zerg-egg.py --self-test          # 含"能失败"的负例
"""
import argparse
import datetime
import hashlib
import json
import os
import shutil
import struct
import sys
import tempfile

MAGIC = "ZEGG"
FORMAT_VERSION = 1
TOOL_NAME = "zerg-egg"
TOOL_VERSION = "1.0.0"
MIN_ALIGN = 4096
HEADER_PREFIX = 32   # 固定前缀：magic(4)+ver(4)+align(u64)+index_len(u64)+保留(8)
CHUNK_SIZE = 64 << 20  # 分块大小（§13.2：卵的传输按块 sha 校验 + 断点续传）
LEN_SLOT = 8          # 兼容旧笔记的命名，实际不再使用尾槽


def _align_of(path_or_arg=None):
    """默认对齐：按平台取（Apple Silicon 16K / 其他 4K）。"""
    if sys.platform == "darwin":
        return 16384
    return 4096


def _is_pow2(n):
    return n > 0 and (n & (n - 1)) == 0


def _sha256_file(path, buf=1 << 20):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(buf), b""):
            h.update(b)
    return h.hexdigest()


def build_prefix(align, index_len):
    """固定 32 字节前缀的**唯一实现**（parse_index 与之同源）。"""
    return MAGIC.encode() + struct.pack("<I", FORMAT_VERSION) + struct.pack("<Q", align) + \
        struct.pack("<Q", index_len) + b"\0" * (HEADER_PREFIX - 24)


def _pad(out, align):
    off = out.tell()
    pad = (-off) % align
    if pad:
        out.write(b"\0" * pad)


def collect_entries(src):
    """遍历源目录，产出条目（顺序确定：目录与文件都排序）。"""
    entries = []
    for root, dirs, files in os.walk(src):
        dirs.sort()
        for fn in sorted(files):
            fp = os.path.join(root, fn)
            if os.path.islink(fp):
                raise ValueError("v1 不支持符号链接：%s" % fp)
            rel = os.path.relpath(fp, src)
            if rel.startswith(".."):
                raise ValueError("越界路径：%s" % rel)
            st = os.stat(fp)
            entries.append({"path": rel.replace(os.sep, "/"), "size": st.st_size,
                            "sha256": _sha256_file(fp), "mode": st.st_mode & 0o777, "_src": fp})
    return entries


def build_index(entries, align):
    """生成索引 JSON 的**唯一实现**（parse_index 与之同源）。"""
    return {
        "magic": MAGIC,
        "version": FORMAT_VERSION,
        "align": align,
        "tool": {"name": TOOL_NAME, "version": TOOL_VERSION},
        "created_at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "entries": [{k: e[k] for k in ("path", "off", "size", "sha256", "mode")} for e in entries],
    }


def pack(src, egg, align=None):
    align = align or _align_of()
    if not _is_pow2(align) or align < MIN_ALIGN:
        raise ValueError("align 必须是 2 的幂且 >= %d（收到 %r）" % (MIN_ALIGN, align))
    entries = collect_entries(src)
    if not entries:
        raise ValueError("源目录里没有文件：%s" % src)
    with open(egg, "wb") as out:
        out.write(b"\0" * align)                      # 头部块占位
        for e in entries:
            _pad(out, align)
            e["off"] = out.tell()
            with open(e["_src"], "rb") as f:
                for b in iter(lambda: f.read(1 << 20), b""):
                    out.write(b)
            assert out.tell() - e["off"] == e["size"], "写入长度与统计不符"
        _pad(out, align)
        idx = json.dumps(build_index(entries, align), ensure_ascii=False, separators=(",", ":")).encode()
        if HEADER_PREFIX + len(idx) > align:
            raise ValueError("索引过大（%d 字节）放不进 %d 字节的头块" % (len(idx), align))
        out.seek(0)
        out.write(build_prefix(align, len(idx)))
        out.write(idx)
    return entries


def parse_index(path):
    """解析索引（与 build_index 同源的唯一读端）。"""
    with open(path, "rb") as f:
        hdr = f.read(HEADER_PREFIX)
        if len(hdr) < HEADER_PREFIX:
            raise ValueError("文件太短，不足固定前缀 %d 字节" % HEADER_PREFIX)
        magic = hdr[0:4]
        ver = struct.unpack("<I", hdr[4:8])[0]
        align = struct.unpack("<Q", hdr[8:16])[0]
        n = struct.unpack("<Q", hdr[16:24])[0]
        if magic != MAGIC.encode():
            raise ValueError("magic 不符（不是 %s 文件）：%r" % (MAGIC, magic))
        if ver != FORMAT_VERSION:
            raise ValueError("格式版本不支持：%r" % ver)
        if not _is_pow2(align) or align < MIN_ALIGN:
            raise ValueError("前缀里的 align 不合理：%r" % align)
        if not (0 < n <= align - HEADER_PREFIX):
            raise ValueError("索引长度字段不合理：%r" % n)
        idx = json.loads(f.read(n).decode("utf-8"))
    if idx.get("magic") != MAGIC:
        raise ValueError("magic 不符（不是 %s 文件）" % MAGIC)
    if idx.get("version") != FORMAT_VERSION:
        raise ValueError("格式版本不支持：%r" % idx.get("version"))
    align = idx.get("align")
    if not _is_pow2(align or 0) or align < MIN_ALIGN:
        raise ValueError("align 字段不合理：%r" % align)
    return idx


def xfer(src, dst, chunk_size=CHUNK_SIZE, quiet=False):
    """按块可续传地把一枚卵拷到目标（§13.2）。

    行为：逐块比较目标与源（块内 sha256）⇒ 缺的/坏的块才写 ⇒ **中断后重跑即续传**，
    且能顺手修掉传输中损坏的块。返回 (写入块数, 总块数)；源比目标短时按源长度截断。
    """
    import os as _os
    total = _os.path.getsize(src)
    n_chunks = (total + chunk_size - 1) // chunk_size
    wrote = 0
    if not _os.path.exists(dst):
        open(dst, "wb").close()
    with open(src, "rb") as fs, open(dst, "r+b" if _os.path.getsize(dst) else "w+b") as fd:
        for i in range(n_chunks):
            off = i * chunk_size
            want = min(chunk_size, total - off)
            fs.seek(off)
            data = fs.read(want)
            h = hashlib.sha256(data).hexdigest()
            got = b""
            if _os.path.getsize(dst) >= off + want:
                fd.seek(off)
                got = fd.read(want)
            if hashlib.sha256(got).hexdigest() != h:
                fd.seek(off)
                fd.write(data)
                wrote += 1
        fd.truncate(total)
    if not quiet:
        print("xfer：%d/%d 块需写（其余已就绪）⇒ %s" % (wrote, n_chunks, dst))
    return wrote, n_chunks


def verify(path, rebuild_into=None):
    """校验头块、对齐、逐段 sha256；可选重建到目录。返回 (problems, index)。"""
    problems = []
    idx = parse_index(path)
    align = idx["align"]
    size_file = os.path.getsize(path)
    with open(path, "rb") as f:
        for e in idx["entries"]:
            if e["off"] % align != 0:
                problems.append("%s 偏移未对齐（%d %% %d != 0）" % (e["path"], e["off"], align))
            if e["off"] + e["size"] > size_file:
                problems.append("%s 越出文件末尾" % e["path"])
                continue
            f.seek(e["off"])
            h = hashlib.sha256()
            left = e["size"]
            while left > 0:
                b = f.read(min(1 << 20, left))
                if not b:
                    problems.append("%s 数据截断" % e["path"])
                    break
                h.update(b)
                left -= len(b)
            if h.hexdigest() != e["sha256"]:
                problems.append("%s sha256 不符" % e["path"])
    if rebuild_into:
        os.makedirs(rebuild_into, exist_ok=True)
        with open(path, "rb") as f:
            for e in idx["entries"]:
                tgt = os.path.join(rebuild_into, e["path"])
                os.makedirs(os.path.dirname(tgt), exist_ok=True)
                f.seek(e["off"])
                with open(tgt, "wb") as g:
                    left = e["size"]
                    while left > 0:
                        b = f.read(min(1 << 20, left))
                        if not b:
                            break
                        g.write(b)
                        left -= len(b)
                os.chmod(tgt, e["mode"])
    return problems, idx


def cmd_info(path):
    idx = parse_index(path)
    print("卵文件：%s" % path)
    print("  magic=%s version=%s align=%d（%d KiB）" % (idx["magic"], idx["version"], idx["align"], idx["align"] // 1024))
    print("  产出工具=%s %s  产出时间=%s" % (idx["tool"]["name"], idx["tool"]["version"], idx["created_at"]))
    print("  段数=%d  文件大小=%.1f KiB" % (len(idx["entries"]), os.path.getsize(path) / 1024))
    for e in idx["entries"]:
        print("    - %-40s %9d B  off=%d  %s" % (e["path"], e["size"], e["off"], e["sha256"][:16]))
    return 0


def _self_test():
    ok = 0
    with tempfile.TemporaryDirectory() as t:
        src = os.path.join(t, "tree")
        os.makedirs(os.path.join(src, "model"))
        os.makedirs(os.path.join(src, "tokenizer"))
        open(os.path.join(src, "config.json"), "w").write('{"arch":"demo"}')
        open(os.path.join(src, "tokenizer", "tokenizer.json"), "w").write('{"v":1}')
        for i in (1, 2):
            with open(os.path.join(src, "model", "shard-%d.safetensors" % i), "wb") as f:
                f.write(os.urandom(120000))
        egg = os.path.join(t, "demo.egg")
        entries = pack(src, egg, align=4096)
        assert len(entries) == 4, "条目数不对"
        ok += 1
        probs, idx = verify(egg, rebuild_into=os.path.join(t, "rebuilt"))
        assert not probs, "干净卵不该有问题：%r" % probs
        ok += 1
        # 与源逐字节相同
        for e in entries:
            a = open(os.path.join(src, e["path"]), "rb").read()
            b = open(os.path.join(t, "rebuilt", e["path"]), "rb").read()
            assert hashlib.sha256(a).digest() == hashlib.sha256(b).digest(), "重建内容不同"
        ok += 1
        # 负例①：改一个字节 ⇒ verify 必须失败
        with open(egg, "r+b") as f:
            f.seek(idx["entries"][0]["off"] + 10)
            f.write(b"\xff")
        probs, _ = verify(egg)
        assert probs and any("sha256" in p for p in probs), "改了字节却没报出来"
        ok += 1
        # 负例②：截断 ⇒ 必须失败
        with open(egg, "rb") as f:
            data = f.read()
        trunc = os.path.join(t, "trunc.egg")
        open(trunc, "wb").write(data[: idx["entries"][-1]["off"] + 1])
        probs, _ = verify(trunc)
        assert probs, "截断却没报出来"
        ok += 1
        # 负例③：magic 被破坏 ⇒ 必须拒绝
        bad = os.path.join(t, "bad.egg")
        open(bad, "wb").write(b"XXXX" + data[4:])   # 只坏 magic
        try:
            parse_index(bad)
            raise AssertionError("magic 坏了却没拒绝")
        except ValueError:
            ok += 1
        # 负例④：align 非 2 的幂 ⇒ 必须拒绝
        try:
            pack(src, os.path.join(t, "a.egg"), align=3000)
            raise AssertionError("align=3000 竟被接受")
        except ValueError:
            ok += 1
    # xfer ①：全新拷贝 ⇒ 必须与源逐字节相同
    # 注意：必须用**独立的**源树 —— 前面的负例动过原树，复用会假红（本轮实测踩到）。
    src_x = os.path.join(t, "xtree")
    os.makedirs(os.path.join(src_x, "sub"), exist_ok=True)
    open(os.path.join(src_x, "a.bin"), "wb").write(os.urandom(100000))
    open(os.path.join(src_x, "sub", "b.bin"), "wb").write(os.urandom(50000))
    src2, dst2 = os.path.join(t, "d.egg"), os.path.join(t, "d.copy")
    pack(src_x, src2, align=4096)
    w1, n1 = xfer(src2, dst2, chunk_size=64 * 1024, quiet=True)
    assert w1 == n1, "全新拷贝应写全部块（%d/%d）" % (w1, n1)
    assert open(src2, "rb").read() == open(dst2, "rb").read(), "拷贝后内容不同"
    ok += 1
    # xfer ②：中断（把目标截一半）⇒ 重跑必须续传补齐且逐字节相同
    half = os.path.getsize(dst2) // 2
    with open(dst2, "r+b") as f:
        f.truncate(half)
    w2, _ = xfer(src2, dst2, chunk_size=64 * 1024, quiet=True)
    assert 0 < w2 < n1, "半截目标只应补一部分块（实得 %d）" % w2
    assert open(src2, "rb").read() == open(dst2, "rb").read(), "续传后内容不同"
    ok += 1
    # xfer ③：坏块 ⇒ 必须被发现并修复
    with open(dst2, "r+b") as f:
        f.seek(0)
        f.write(b"XXXX")
    w3, _ = xfer(src2, dst2, chunk_size=64 * 1024, quiet=True)
    assert w3 >= 1, "坏块必须被重写（实得 %d）" % w3
    assert open(src2, "rb").read() == open(dst2, "rb").read(), "修复后内容不同"
    ok += 1
    print("self-test 通过（%d 项：3 正例 + 4 负例 + 3 条 xfer 用例）" % ok)
    return 0


def main():
    ap = argparse.ArgumentParser(description="卵的单文件封装工具（*.egg）")
    ap.add_argument("cmd", nargs="?", choices=["pack", "info", "verify", "xfer"])
    ap.add_argument("a", nargs="?")
    ap.add_argument("b", nargs="?")
    ap.add_argument("--align", type=int, default=None)
    ap.add_argument("--rebuild-into", default=None)
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--chunk-size", type=int, default=CHUNK_SIZE, help="xfer 的分块大小（默认 64 MiB）")
    args = ap.parse_args()
    if args.self_test:
        return _self_test()
    if args.cmd == "pack":
        if not (args.a and args.b):
            print("用法：pack <源目录> <输出.egg>", file=sys.stderr)
            return 64
        entries = pack(args.a, args.b, align=args.align)
        print("已打包 %d 个文件 → %s（align=%d，%.1f KiB）" % (
            len(entries), args.b, args.align or _align_of(), os.path.getsize(args.b) / 1024))
        return 0
    if args.cmd == "info":
        return cmd_info(args.a)
    if args.cmd == "xfer":
        if not (args.a and args.b):
            print("用法：xfer <源.egg> <目标.egg>", file=sys.stderr)
            return 64
        wrote, n = xfer(args.a, args.b, chunk_size=args.chunk_size)
        probs, _ = verify(args.b)
        if probs:
            print("✗ 目标校验失败 %d 项：" % len(probs))
            return 1
        print("✓ 目标与索引一致（本次写 %d/%d 块）" % (wrote, n))
        return 0
    if args.cmd == "verify":
        probs, idx = verify(args.a, rebuild_into=args.rebuild_into)
        if probs:
            print("✗ 校验失败 %d 项：" % len(probs))
            for p in probs:
                print("   - " + p)
            return 1
        print("✓ 校验通过（%d 段，align=%d）%s" % (
            len(idx["entries"]), idx["align"], "，已重建到 " + args.rebuild_into if args.rebuild_into else ""))
        return 0
    ap.print_help()
    return 64


if __name__ == "__main__":
    sys.exit(main())
