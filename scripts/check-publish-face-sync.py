#!/usr/bin/env python3
"""发布面同步门禁：白名单导出面 × 两器的排除口径 × 私有面黑名单 —— 三者不许漂移。

**为什么需要它（2026-09-16 批 2'.6 实测撞上的形态，不是假想）**：
`publish/private-paths.txt` 黑名单里后来收进的三个「发布机制自身」脚本
（`scripts/publish-preflight.sh` / `scripts/check-public-tree-hazards.py` /
`scripts/check-placeholder-residue.py`）只进了镜像器自带的 `DROP_EXACT`，
**没有**同步进 `scripts/publish-public.sh` 的 `EXCLUDES=( … )`（B8 声明的「单一真源」，
镜像器解析的就是这个数组）⇒ **旧器（应急后备通道）真跑时把这三个文件导出，随后被自己的
私有面门禁抓住而整批自中止**（一个字节都不推）；两器口径静默漂移，而两器一致性验收
`scripts/test-publish-parity.sh` 要跑约 6 分钟才看得见。本门禁是**秒级**的那一面镜子。

**判据（任一不满足 ⇒ rc=1；用法/缺件/无区分度 ⇒ rc=2）**：
  ① 漂移 —— 白名单选中的路径里，镜像器**丢**（`map_path` 为 None）而旧器**不排**（不在
     `EXCLUDES`）⇒ 红。旧器真跑会把该文件导进快照 ⇒ 被私有面门禁抓住 ⇒ 整批自中止。
  ② 两边都漏 —— 命中私有面黑名单、且**两器都没处理**（旧器不排 + 镜像器不丢）⇒ 红。
     两器都会把私有面路径带进公开面（旧器门禁 / 镜像器末树门禁各自中止）。
  ③ 有区分度（守卫）—— 黑名单解析为空 / 白名单为空 / 白名单里没有 `wall/` / 候选集里
     没有一条 `wall/` 路径 / EXCLUDES 一条都解析不出来 ⇒ **rc=2 拒绝给结论**
     （「配置没了所以全绿」是最难发现的假绿）。
  ④ `--self-test` 必须先过（本门禁**自检不过就拒绝扫真目标**，与
     `scripts/scan-replacement-residue.py` 同一纪律）。

**单一真源（一律复用，绝不另抄一份解析器）**：
  · 白名单解析 / EXCLUDES 解析 —— `scripts/check-history-secrets.py`
    （`load_whitelist` / `published_path` / `load_excludes` / `excluded`；镜像器解析的也是它）
  · 私有面黑名单解析 + `!` 豁免 —— `scripts/check-public-tree-private.py`
  · 镜像器的丢弃口径 —— `publish/mirror-public-lib.py` 的 `map_path`（它内含
    `DROP_EXACT` / `DROP_PREFIX`）

**口径（为什么 `publish/` 下的路径不在本门禁内）**：旧器对 `publish/` 是**结构性**处置
（先把整个 `publish/` 导进快照、再 `rm -rf` 掉，只留 overlay 拷出来的那几件），不是
`EXCLUDES` 驱动的 ⇒ 拿 `EXCLUDES` 去比它没有意义。故候选集**排除 `publish/`**，
并在输出里明写这条边界；`publish/` 的映射一致性由 `publish/parity-contract.tsv` 与
`publish/parity-compare.py` 负责。

用法：
    python3 scripts/check-publish-face-sync.py                 # 查本仓（自动先跑自检）
    python3 scripts/check-publish-face-sync.py --repo <仓根>    # 查别的仓（夹具/演练）
    python3 scripts/check-publish-face-sync.py --self-test      # 只跑合成夹具自检
可选参数（默认取 `--repo` 下的标准位置）：`--whitelist` / `--private-paths` /
`--publish-sh` / `--mirror-lib`。
"""
import argparse
import importlib.util
import os
import shutil
import subprocess
import sys
import tempfile

# 本门禁**自己**所在的仓（复用的门禁脚本一律从这里加载，不随 --repo 走）
REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

WALL_ENTRY = "wall/"


def load_module(name, path):
    if not os.path.exists(path):
        print("✗ 缺少要复用的资产：%s（本门禁不接受缺件）" % path, file=sys.stderr)
        raise SystemExit(2)
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        print("✗ 无法加载 %s" % path, file=sys.stderr)
        raise SystemExit(2)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


HSEC = load_module("face_sync_hsec", os.path.join(REPO, "scripts", "check-history-secrets.py"))
PRIV = load_module("face_sync_priv", os.path.join(REPO, "scripts", "check-public-tree-private.py"))


def tracked_files(repo):
    r = subprocess.run(["git", "-C", repo, "ls-files"], capture_output=True, text=True)
    if r.returncode != 0:
        print("✗ git ls-files 失败：%s（rc=%d）" % (repo, r.returncode), file=sys.stderr)
        raise SystemExit(2)
    return [ln for ln in r.stdout.splitlines() if ln]


def require_paths(paths):
    """返回缺件的键列表（缺件一律 rc=2，绝不静默跳过）。"""
    return [k for k, v in paths.items() if not os.path.exists(v)]


def classify(repo, paths):
    """→ (candidates, mirror_dropped, private_hits, entries, excludes, n_wall)"""
    entries = HSEC.load_whitelist(paths["whitelist"])
    excludes = HSEC.load_excludes(paths["publish_sh"])
    pdirs, pfiles, pallowed = PRIV.load_rules(paths["private_paths"])
    mirror = load_module("face_sync_mirror", paths["mirror_lib"])

    def private_hit(rel):
        if PRIV.is_allowed(rel, pallowed):
            return None
        for d in pdirs:
            if rel.startswith(d):
                return d
        if rel in pfiles:
            return rel
        return None

    candidates, dropped, hits, wall = [], [], [], []
    for rel in tracked_files(repo):
        if not HSEC.published_path(rel, entries):
            continue
        if rel.startswith("publish/"):      # 结构性处置，不在本门禁口径内（见文件头注释）
            continue
        candidates.append(rel)
        if rel.startswith(WALL_ENTRY):
            wall.append(rel)
        m = private_hit(rel)
        if m:
            hits.append((rel, m))
        if mirror.map_path(rel) is None:    # 镜像器自带清单丢弃
            dropped.append(rel)
    return candidates, dropped, hits, wall, entries, excludes


def evaluate(repo, paths):
    """→ (rc, lines)。本函数只做判定，不落盘、不 exec。"""
    missing = require_paths(paths)
    if missing:
        return 2, ["  ✗ 缺件（%s）⇒ 拒绝给结论（rc=2）" % "、".join(sorted(missing))]
    cand, dropped, hits, wall, entries, excludes = classify(repo, paths)
    pdirs, pfiles, pallowed = PRIV.load_rules(paths["private_paths"])
    lines = []
    lines.append("发布面同步门禁 —— 候选 %d 条（白名单 %d 条 · 黑名单 %d 目录+%d 文件 · EXCLUDES %d 条）"
                 % (len(cand), len(entries), len(pdirs), len(pfiles), len(excludes)))
    lines.append("  候选里的 wall/ 路径：%d 条" % len(wall))
    lines.append("  镜像器自带清单丢弃（map_path=None）：%d 条" % len(dropped))
    lines.append("  命中私有面黑名单：%d 条" % len(hits))

    # ③ 有区分度守卫（先判，避免「配置没了所以全绿」）
    if not cand:
        return 2, lines + ["  ✗ 守卫：候选集为空 ⇒ 拒绝给结论（rc=2）"]
    if not entries:
        return 2, lines + ["  ✗ 守卫：白名单解析为空 ⇒ 拒绝给结论（rc=2）"]
    if not excludes:
        return 2, lines + ["  ✗ 守卫：EXCLUDES 一条都解析不出来 ⇒ 拒绝给结论（rc=2）"]
    if not pdirs and not pfiles:
        return 2, lines + ["  ✗ 守卫：私有面黑名单解析为空 ⇒ 拒绝给结论（rc=2）"]
    if WALL_ENTRY not in entries and not any(e.rstrip("/") == WALL_ENTRY.rstrip("/") for e in entries):
        return 2, lines + ["  ✗ 守卫：白名单里没有 %s ⇒ 拒绝给结论（rc=2）" % WALL_ENTRY]
    if not wall:
        return 2, lines + ["  ✗ 守卫：候选集里没有一条 %s 路径 ⇒ 拒绝给结论（rc=2）" % WALL_ENTRY]

    bad = 0
    # ① 漂移：镜像器丢、旧器不排 ⇒ 旧器真跑会导出它（自中止）
    drift = [p for p in dropped if not HSEC.excluded(p, excludes)]
    if drift:
        bad = 1
        lines.append("  ✗ 判据① 漂移 %d 条 —— 镜像器丢、旧器不排（旧器真跑会导出 ⇒ 被私有面门禁抓住而整批自中止）：" % len(drift))
        for p in drift[:40]:
            lines.append("      %s" % p)
    else:
        lines.append("  ✅ 判据① 漂移：0 条（镜像器丢弃的路径旧器全都排除了）")

    # ② 两边都漏：命中黑名单且两器都没处理
    leak = [(p, m) for p, m in hits if not HSEC.excluded(p, excludes) and p not in dropped]
    if leak:
        bad = 1
        lines.append("  ✗ 判据② 两边都漏 %d 条 —— 命中私有面黑名单且两器都没处理（两器都会把私有面路径带进公开面）：" % len(leak))
        for p, m in leak[:40]:
            lines.append("      %-60s ← 规则 %s" % (p, m))
    else:
        lines.append("  ✅ 判据② 两边都漏：0 条（命中黑名单的路径两器都已处理）")

    return bad, lines


# ── 自检：合成夹具（每条都能红）────────────────────────────────────────────────────────
FIXTURE_LIB = '''# 夹具用的最小 mirror lib（只提供本门禁读的三个名字）
DROP_PREFIX = ()
DROP_EXACT = {%s}


def map_path(rel):
    if rel in DROP_EXACT or any(rel.startswith(p) for p in DROP_PREFIX):
        return None
    return rel
'''

FIXTURES = [
    # (名字, whitelist, private_paths, excludes, drop_exact, 期望 rc[, 选项])
    ("ok-both-dropped", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 0),
    ("drift-mirror-only", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/unrelated-but-real.py"], ["scripts/mech.py"], 1),
    ("both-leak", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/unrelated-but-real.py"], [], 1),
    ("no-private-rules", ["scripts/", "wall/"], [],
     ["scripts/mech.py"], ["scripts/mech.py"], 2),
    ("whitelist-without-wall", ["scripts/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 2),
    ("candidate-without-wall", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 2, {"no_wall_files": True}),
    ("missing-whitelist", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 2, {"delete_whitelist": True}),
]

FIXTURE_TRACKED = ["scripts/mech.py", "scripts/keep.py", "wall/src/lib.rs", "wall/Cargo.toml"]


def make_fixture(root, wl, priv, excl, drop, opts):
    os.makedirs(os.path.join(root, "publish"), exist_ok=True)
    os.makedirs(os.path.join(root, "scripts"), exist_ok=True)
    os.makedirs(os.path.join(root, "wall/src"), exist_ok=True)
    with open(os.path.join(root, "publish", "whitelist.txt"), "w", encoding="utf-8") as fh:
        fh.write("# 夹具白名单\n" + "\n".join(wl) + "\n")
    with open(os.path.join(root, "publish", "private-paths.txt"), "w", encoding="utf-8") as fh:
        fh.write("# 夹具黑名单\n" + "\n".join(priv) + "\n")
    ex_lines = "\n".join('  "%s"' % e for e in excl)
    with open(os.path.join(root, "scripts", "publish-public.sh"), "w", encoding="utf-8") as fh:
        fh.write("#!/usr/bin/env bash\nEXCLUDES=(\n%s\n)\n" % ex_lines)
    with open(os.path.join(root, "publish", "mirror-public-lib.py"), "w", encoding="utf-8") as fh:
        fh.write(FIXTURE_LIB % ", ".join('"%s"' % d for d in drop))
    tracked = list(FIXTURE_TRACKED)
    if opts.get("no_wall_files"):
        tracked = [t for t in tracked if not t.startswith("wall/")]
    for rel in tracked:
        p = os.path.join(root, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        open(p, "w", encoding="utf-8").write("x\n")
    subprocess.run(["git", "init", "-q", "-b", "main", root], check=True)
    subprocess.run(["git", "-C", root, "add", "-A"], check=True,
                   env=dict(os.environ, GIT_AUTHOR_NAME="t", GIT_AUTHOR_EMAIL="t@e",
                            GIT_COMMITTER_NAME="t", GIT_COMMITTER_EMAIL="t@e"))
    if opts.get("delete_whitelist"):
        os.unlink(os.path.join(root, "publish", "whitelist.txt"))


def paths_for(repo):
    return {
        "whitelist": os.path.join(repo, "publish", "whitelist.txt"),
        "private_paths": os.path.join(repo, "publish", "private-paths.txt"),
        "publish_sh": os.path.join(repo, "scripts", "publish-public.sh"),
        "mirror_lib": os.path.join(repo, "publish", "mirror-public-lib.py"),
    }


def self_test():
    base = tempfile.mkdtemp(prefix="face-sync-selftest-")
    print("自检工作区：%s" % base)
    pass_n = fail_n = 0
    for fx in FIXTURES:
        name, wl, priv, excl, drop, want = fx[0], fx[1], fx[2], fx[3], fx[4], fx[5]
        opts = fx[6] if len(fx) > 6 else {}
        root = os.path.join(base, name)
        os.makedirs(root, exist_ok=True)
        make_fixture(root, wl, priv, excl, drop, opts)
        try:
            rc, _lines = evaluate(root, paths_for(root))
        except SystemExit as exc:            # 缺件 ⇒ 2（与真跑同语义）
            rc = exc.code if isinstance(exc.code, int) else 2
        okc = (rc == want)
        if okc:
            pass_n += 1
            print("  ✅ %-24s rc=%d（期望 %d）" % (name, rc, want))
        else:
            fail_n += 1
            print("  ❌ %-24s rc=%d（期望 %d）" % (name, rc, want))
    shutil.rmtree(base, ignore_errors=True)
    print("自检：%d 过 / %d 红（%d 条用例）" % (pass_n, fail_n, pass_n + fail_n))
    return 0 if fail_n == 0 else 1


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", default=REPO)
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    ap.add_argument("--whitelist")
    ap.add_argument("--private-paths", dest="private_paths")
    ap.add_argument("--publish-sh", dest="publish_sh")
    ap.add_argument("--mirror-lib", dest="mirror_lib")
    args = ap.parse_args()

    if args.self_test:
        return self_test()

    if self_test() != 0:
        print("\n✗ 自检未过 ⇒ 拒绝扫真目标（rc=2）")
        return 2

    repo = os.path.abspath(args.repo)
    paths = paths_for(repo)
    for k, v in (("whitelist", args.whitelist), ("private_paths", args.private_paths),
                 ("publish_sh", args.publish_sh), ("mirror_lib", args.mirror_lib)):
        if v:
            paths[k] = os.path.abspath(v)
    for k, v in paths.items():
        if not os.path.exists(v):
            print("✗ 缺件（%s）：%s ⇒ 拒绝给结论（rc=2）" % (k, v))
            return 2

    print("仓：%s" % repo)
    print("口径：候选集排除 `publish/`（旧器对它是结构性处置，不由 EXCLUDES 驱动）\n")
    rc, lines = evaluate(repo, paths)
    for ln in lines:
        print(ln)
    if rc == 0:
        print("\n结果: 通过 —— 两器排除口径一致，私有面命中两器都已处理")
    elif rc == 1:
        print("\n结果: ✗ 发布面不同步（详见上面 ✗ 行）—— 真跑发布器会自中止/或两器漂移")
    else:
        print("\n结果: ✗ 拒绝给结论（无区分度或用法问题，rc=2）")
    return rc


if __name__ == "__main__":
    sys.exit(main())
