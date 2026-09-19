#!/usr/bin/env python3
"""kb_docs_sync v1.0.0 —— 项目文档 → 知识库同步（变更检测 + 候选门控）

设计: Zerg-内部文档/项目文档/v2.5.8/设计-v2.5.8-文档体系-模型读取成本-20260903.md 6.2（★ 2026-09-19 二次分家批5：该件现位置 = 同级 `Zerg-内部文档/项目文档/…`，仓内不再有）
★ 2026-09-19 「项目文档」二次分家批2：取源根 `DOCS` **双认**（仓内 `docs/项目文档` 优先、缺则同级
  仓外 `Zerg-内部文档/项目文档/`）——与 `docs/site/export-and-build.sh` 的 `ZERG_DOCS_ALT` 同根同默认；
  **两处都缺 ⇒ exit 2 不给结论**（原行为是打印「未找到当前版本目录」后 exit 0 = 假绿）。
  ★ 版本值**未动**（本脚本行为已改 ⇒ 按铁律 4 本该升版本；但 `scripts/gates/check-tool-version-sync.sh`
  明写「改抬头前先由 Mr2109 定版本值，勿自动顺延」⇒ 版本值**待拍**，见 tools/kb_docs_sync.md 履历登记）。
  ★ 批3 后 `file_path`（= relpath 到 REPO）会带 `../Zerg-内部文档/` 前缀 ⇒ KB 侧按**文件名**建索引
  （registry 键 = basename）不受影响；`file_path` 写法待拍。
核心: 元数据注册表 + content_hash 变更检测 —— 只处理新增/变更 —— 产出候选条目（不直连 knowledge.db——
      知识库铁律: 入库走 MCP kb_*——脚本做检测与候选——入库由核验者执行）

用法:
  python3 kb_docs_sync.py                 # dry-run: 检测变更 → 打印候选清单（不写库）
  python3 kb_docs_sync.py --mark <file>   # 核验入库后标记注册表（更新 hash/ts）
注册表: <volume-path>（持久——记录已入库 file_path+hash）
候选输出: <volume-path>（核验者据此 kb_add——domain=虫族）
"""
import json, os, re, sys, hashlib, datetime

KB_DIR = "<volume-path>"
REGISTRY = os.path.join(KB_DIR, "kb_docs_registry.json")
CANDIDATES = os.path.join(KB_DIR, "kb_sync_candidates.json")
REPO = "<repo>"
#   ★ 2026-09-19 「项目文档」二次分家**批2**（双认 · 先仓内、缺则仓外取源根 · 缺件 rc=2）：
#     `docs/项目文档/` 整目录将在**批3** 移入同级仓外目录 `Zerg-内部文档/项目文档/`（工作树之外）⇒
#     取源根两处都认：仓内 `docs/项目文档` 优先，缺则仓外 `../Zerg-内部文档/项目文档`（与
#     `docs/site/export-and-build.sh` 的 `ZERG_DOCS_ALT` 同根同默认）。**两处都不在盘上** ⇒
#     本脚本 `exit 2` 不给结论（原先静默打印「未找到当前版本目录」后 `exit 0` = 假绿）。
DOCS_CANDIDATES = (os.path.join(REPO, "docs", "项目文档"),
                   os.path.abspath(os.path.join(REPO, "..", "Zerg-内部文档", "项目文档")))
DOCS = next((p for p in DOCS_CANDIDATES if os.path.isdir(p)), DOCS_CANDIDATES[0])

# 入库"关键文档"清单（G3——随版本演化——当前版由 current_version_dir() 动态取）
def current_version_dir():
    if not os.path.isdir(DOCS):          # ★ 批2：两处取源根都不在盘上 ⇒ 不给结论（调用方报 rc=2）
        return None
    vs = [d for d in os.listdir(DOCS) if os.path.isdir(os.path.join(DOCS, d)) and re.match(r"^v\d+\.\d+\.\d+$", d)]
    if not vs:
        return None
    def key(v):
        return tuple(int(x) for x in v[1:].split("."))
    return os.path.join(DOCS, max(vs, key=key))

def key_docs(ver_dir):
    """关键文档 glob 清单（当前真相层——G3）"""
    pats = ["00-架构*.md", "使用-虫族指南*.md", "变更-*.md", "01-模块-主控*.md",
            "04-模块-任务*.md", "06-模块-子端*.md", "07-模块-UI*.md", "设计-v2.5.8-*.md"]
    out = {}
    for p in pats:
        for f in sorted(__import__("glob").glob(os.path.join(ver_dir, p))):
            out[os.path.basename(f)] = f
    return out

def file_hash(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        h.update(f.read())
    return h.hexdigest()[:16]

def load_registry():
    if os.path.exists(REGISTRY):
        try:
            return json.load(open(REGISTRY, encoding="utf-8"))
        except Exception:
            pass
    return {}

def save_registry(reg):
    os.makedirs(KB_DIR, exist_ok=True)
    json.dump(reg, open(REGISTRY, "w", encoding="utf-8"), ensure_ascii=False, indent=1)

def extract_candidate(fpath):
    """从文档提取候选条目素材（标题/版本/内容要点——供核验者精炼后 kb_add）"""
    try:
        lines = open(fpath, encoding="utf-8").read().split("\n")
    except Exception:
        return None
    title = ""
    for ln in lines[:6]:
        if ln.startswith("#"):
            title = ln.lstrip("# ").strip()
            break
    body = [ln.strip() for ln in lines[1:40] if ln.strip() and not ln.startswith("```")]
    preview = " ".join(body)[:400]
    rel = os.path.relpath(fpath, REPO)
    return {
        "title": title or os.path.basename(fpath),
        "file_path": rel,           # 含版本号——同路径覆盖制（升级替换）
        "domain": "虫族",
        "source": "项目文档",
        "content_preview": preview, # 核验者精炼为 content 后 kb_add
        "kb_add_note": f"kb_add title=<标题> domain=虫族 file_path={rel} content=<精炼要点>",
    }

def main():
    ver_dir = current_version_dir()
    if not ver_dir:
        print("未找到当前版本目录——两处取源根都不在盘上：")
        for p in DOCS_CANDIDATES:
            print("  %s" % p)
        print("⇒ 判据不可判（不给结论）· rc=2")
        return 2
    ver = os.path.basename(ver_dir)
    reg = load_registry()
    docs = key_docs(ver_dir)
    changed, added, unchanged = [], [], []
    for name, fpath in sorted(docs.items()):
        h = file_hash(fpath)
        prev = reg.get(name)
        if prev is None:
            added.append(name)
        elif prev.get("hash") != h:
            changed.append(name)
        else:
            unchanged.append(name)
    print(f"kb_docs_sync: 当前版 {ver}——关键文档 {len(docs)} 份（新增 {len(added)}/变更 {len(changed)}/未变 {len(unchanged)}）")
    if not added and not changed:
        print("无变更——知识库已同步（无需操作）")
        return 0
    cands = []
    for name in added + changed:
        c = extract_candidate(docs[name])
        if c:
            c["change"] = "新增" if name in added else "变更(覆盖)"
            cands.append(c)
            print(f"  [{c['change']}] {c['file_path']}")
    json.dump(cands, open(CANDIDATES, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print(f"\n候选条目 → {CANDIDATES}")
    print("下一步（核验门控——G2）:")
    for c in cands:
        print(f"  · {c['file_path']} —— {c['kb_add_note']}")
    print("全部入库后执行: python3 kb_docs_sync.py --mark 全部  （更新注册表——下次只检测真变更）")
    return 0

if __name__ == "__main__":
    if "--mark" in sys.argv:
        reg = load_registry()
        ver_dir = current_version_dir()
        #   ★ 2026-09-19 二次分家批2：`--mark` 同样先要有取源根（两处都缺 ⇒ rc=2 不给结论，
        #     不许「零命中静默标记完」——那会把注册表按 0 份当同步完成）。
        if not ver_dir:
            print("未找到当前版本目录——两处取源根都不在盘上：")
            for p in DOCS_CANDIDATES:
                print("  %s" % p)
            print("⇒ 判据不可判（不给结论）· rc=2")
            sys.exit(2)
        docs = key_docs(ver_dir)
        if "--mark" in sys.argv and len(sys.argv) > sys.argv.index("--mark") + 1:
            arg = sys.argv[sys.argv.index("--mark") + 1]
            if arg == "全部":
                for name, fpath in docs.items():
                    reg[name] = {"hash": file_hash(fpath), "ts": datetime.datetime.now().isoformat(timespec="seconds")}
                save_registry(reg)
                print(f"注册表已更新（{len(docs)} 份全部标记）")
            else:
                name = os.path.basename(arg)
                if name in docs:
                    reg[name] = {"hash": file_hash(docs[name]), "ts": datetime.datetime.now().isoformat(timespec="seconds")}
                    save_registry(reg)
                    print(f"已标记: {name}")
                else:
                    print(f"未找到关键文档: {name}")
    else:
        sys.exit(main())
