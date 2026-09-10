#!/usr/bin/env bash
# publish-public.sh — 私有权威仓 → 公开快照仓（Google Copybara SQUASH 模式的等价实现）
#
# 2026-09-11 设计：docs/项目文档/v2.5.9/设计-publish-public快照发布-20260911.md（Mr2109已拍板 8/8）
#
# 做的事（全部只读私有仓，只写输出目录）：
#   1. 按 publish/whitelist.txt 导出文件树（rsync --files-from）
#   2. 套用 publish/ 下的发布专属文件（LICENSE/NOTICE/CONTRIBUTING/SECURITY/.env.example/config.example.yaml）
#   3. 快照变换：删私有 fleet.yaml（换 fleet.example.yaml）；ui/Cargo.toml 解除示例虫茧跨仓依赖；套用 replace-rules.tsv
#   4. 门禁扫描（明文凭据/私有路径/人物名/内网地址）——命中即中止；有 gitleaks 则再跑一层
#   5. 生成单个压平提交（作者身份可配，附 GitOrigin-RevId 溯源 trailer）
#   6. --push 时推送到公开仓（默认只准备，不推送）
#
# 用法：
#   scripts/publish-public.sh                      # 导出到 /tmp/zerg-public-<ts>，只准备
#   scripts/publish-public.sh --out ~/zerg-snap    # 指定输出目录
#   scripts/publish-public.sh --dry-run            # 只导出+变换+扫描，不做 git init
#   scripts/publish-public.sh --push               # 完成后推送到 origin（需先设 --remote）
#   scripts/publish-public.sh --remote git@github.com:Mr2109/zerg-swarm.git
#
# 注意：本脚本**绝不**修改私有仓。可重复执行（输出目录已存在时会先清空其中的 git 元数据）。

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WHITELIST="${REPO_ROOT}/publish/whitelist.txt"
RULES="${REPO_ROOT}/publish/replace-rules.tsv"
TS="$(date +%Y%m%d-%H%M%S)"
OUT=""
PUSH=0
DRY_RUN=0
REMOTE="${ZERG_PUBLIC_REMOTE:-}"

while [ $# -gt 0 ]; do
  case "$1" in
    --out) OUT="$2"; shift 2 ;;
    --push) PUSH=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --remote) REMOTE="$2"; shift 2 ;;
    -h|--help) sed -n '2,30p' "$0"; exit 0 ;;
    *) echo "未知参数: $1" >&2; exit 2 ;;
  esac
done
OUT="${OUT:-/tmp/zerg-public-${TS}}"

# 单提交的作者身份（决策 6：集体版权行 + GitHub noreply 提交身份；可用环境变量覆盖）
: "${SNAPSHOT_AUTHOR_NAME:=The Zerg Swarm Authors}"
: "${SNAPSHOT_AUTHOR_EMAIL:=2109+Mr2109@users.noreply.github.com}"

say() { printf '\n\033[1m== %s\033[0m\n' "$*"; }

say "0/6 环境检查"
[ -f "$WHITELIST" ] || { echo "缺少 $WHITELIST" >&2; exit 1; }
[ -f "$RULES" ]     || { echo "缺少 $RULES" >&2; exit 1; }
command -v rsync >/dev/null || { echo "需要 rsync" >&2; exit 1; }
command -v python3 >/dev/null || { echo "需要 python3（用于替换规则与扫描）" >&2; exit 1; }
PRIV_HEAD="$(cd "$REPO_ROOT" && git rev-parse HEAD 2>/dev/null || echo unknown)"
echo "私有仓 HEAD: ${PRIV_HEAD:0:12}"
echo "输出目录:    ${OUT}"

say "1/6 导出白名单文件树"
rm -rf "$OUT"
mkdir -p "$OUT"
# 关键：以 **git 索引** 为源（`git ls-files`），而不是目录递归——
# rsync 不认 .gitignore，直接递归会把编译产物（zerg-agent/zerg-api/… 60MB+）一起带走。
python3 - "$REPO_ROOT" "$WHITELIST" "${OUT}.list" <<'PY'
import subprocess, sys
repo, wl, outlist = sys.argv[1], sys.argv[2], sys.argv[3]
entries = []
for ln in open(wl, encoding="utf-8"):
    ln = ln.strip()
    if not ln or ln.startswith("#"):
        continue
    entries.append(ln)
tracked = subprocess.run(["git", "-C", repo, "ls-files"], capture_output=True, text=True, check=True).stdout.splitlines()
picked = []
for f in tracked:
    for e in entries:
        if e.endswith("/"):
            if f.startswith(e) or f == e.rstrip("/"):
                picked.append(f)
                break
        elif f == e:
            picked.append(f)
            break
open(outlist, "w", encoding="utf-8").write("\n".join(sorted(set(picked))) + "\n")
print(f"  从索引挑选 {len(set(picked))} 个文件（白名单 {len(entries)} 条）")
PY
rsync -a --files-from="${OUT}.list" "$REPO_ROOT/" "$OUT/"

# 排除项（白名单目录内的噪声/私有物）
EXCLUDES=(
  "tools/ocr/venv"
  "tools/ocr/.venv"
  "gateway/fleet.yaml"
  "gateway/fleet.yaml.bak"
  "gateway/TASKS.md"
  "gateway/deploy_x3.sh"
  "gateway/deploy_mini.sh"
  "gateway/install.sh"
  "gateway/deploy"
  "gateway/scripts/fleet_logs.py"
  "scripts/publish-public.sh"   # 发布工具自身不进快照（含私有规则表引用）
)
# 编译缓存 / 原生产物（即使被跟踪也不该出现在源码快照里）
for e in "${EXCLUDES[@]}"; do rm -rf "${OUT:?}/$e"; done
# 通用噪声（白名单递归可能带进来的）
find "$OUT" -name '*.orig' -delete
find "$OUT" -name '*.rej' -delete
find "$OUT" -name '*.bak' -delete
find "$OUT" -type f -name '*.[0-9][0-9][0-9][0-9][0-9][0-9][0-9]*' -delete 2>/dev/null || true
find "$OUT" -name '__pycache__' -type d -prune -exec rm -rf {} + 2>/dev/null || true
find "$OUT" -type f \( -name '*.pyc' -o -name '*.pyo' -o -name '*.o' -o -name '*.a' -o -name '*.so' -o -name '*.dylib' -o -name '*.class' \) -delete 2>/dev/null || true
echo "导出文件数: $(find "$OUT" -type f | wc -l | tr -d ' ')"

say "2/6 套用发布专属文件（publish/ → 快照）"
cp -p "$REPO_ROOT/publish/LICENSE"                 "$OUT/LICENSE"
cp -p "$REPO_ROOT/publish/NOTICE"                  "$OUT/NOTICE"
cp -p "$REPO_ROOT/publish/THIRD_PARTY_LICENSES.md" "$OUT/THIRD_PARTY_LICENSES.md"
cp -p "$REPO_ROOT/publish/CONTRIBUTING.md"         "$OUT/CONTRIBUTING.md"
cp -p "$REPO_ROOT/publish/SECURITY.md"             "$OUT/SECURITY.md"
[ -f "$REPO_ROOT/.env.example" ] && cp -p "$REPO_ROOT/.env.example" "$OUT/.env.example"
cp -p "$REPO_ROOT/publish/config.example.yaml"     "$OUT/gateway/fleet.example.yaml"
[ -f "$REPO_ROOT/publish/gitleaks.toml" ] && cp -p "$REPO_ROOT/publish/gitleaks.toml" "$OUT/.gitleaks.toml"
# 公开文档（C 批成果，源在 publish/docs/）
if [ -d "$REPO_ROOT/publish/docs" ]; then
  mkdir -p "$OUT/docs/design"
  for f in "$REPO_ROOT"/publish/docs/*.md; do
    [ -e "$f" ] || continue
    b="$(basename "$f")"
    case "$b" in
      README.md|README.zh-CN.md) cp -p "$f" "$OUT/$b" ;;   # README 家族放仓库根（语言切换互链）
      *)                         cp -p "$f" "$OUT/docs/$b" ;;
    esac
  done
  for f in "$REPO_ROOT"/publish/docs/design/*.md; do
    [ -e "$f" ] || continue
    cp -p "$f" "$OUT/docs/design/$(basename "$f")"
  done
  echo "  公开文档: $(find "$OUT/docs" -name '*.md' | wc -l | tr -d ' ') 篇（含 README）"
fi
mkdir -p "$OUT/.github/workflows"
cp -p "$REPO_ROOT/publish/ci/ci.yml"               "$OUT/.github/workflows/ci.yml"
cp -p "$REPO_ROOT/publish/ci/release-agent.yml"    "$OUT/.github/workflows/release-agent.yml"
mkdir -p "$OUT/docs/design"
[ -f "$OUT/docs/skills/tool-upgrade.md" ] && mv "$OUT/docs/skills/tool-upgrade.md" "$OUT/docs/design/工具升级规范.md"
# 发布专属文件本身不进快照
rm -rf "$OUT/publish"

say "3/6 快照变换（跨仓依赖解除 + 替换规则）"
# 3a. ui/Cargo.toml：彻底解除 zerg-cocoon 跨仓 path 依赖（保留同名空 feature 以消除 cfg 警告）
if [ -f "$OUT/ui/Cargo.toml" ]; then
  python3 - "$OUT/ui/Cargo.toml" <<'PY'
import re, sys
p = sys.argv[1]
s = open(p, encoding="utf-8").read()
s = re.sub(r'^.*zerg-roundtable\s*=\s*\{\s*path\s*=.*$', '', s, flags=re.M)
s = re.sub(r'^\s*default\s*=\s*\["zerg-roundtable"\]\s*$', '', s, flags=re.M)
s = re.sub(r'^\s*zerg-roundtable\s*=\s*\["dep:zerg-roundtable"\]\s*$',
           'zerg-roundtable = []  # 公开快照：该集装箱（独立仓）不随发布——保留同名空 feature 以消除 cfg 警告', s, flags=re.M)
open(p, "w", encoding="utf-8").write(s)
print("  Cargo.toml: 已解除跨仓依赖")
PY
fi
# 3b. 替换规则（字面 + 正则）
python3 - "$OUT" "$RULES" <<'PY'
import os, re, sys
out, rules_path = sys.argv[1], sys.argv[2]
rules, regex_rules = [], []
for ln in open(rules_path, encoding="utf-8"):
    if ln.startswith("#") or not ln.strip():
        continue
    parts = ln.rstrip("\n").split("\t")
    if len(parts) < 2:
        continue
    pat, rep = parts[0], parts[1]
    if pat.startswith("regex:"):
        regex_rules.append((re.compile(pat[6:]), rep))
    else:
        rules.append((pat, rep))
changed = 0
for root, dirs, files in os.walk(out):
    dirs[:] = [d for d in dirs if d not in (".git", "target", "node_modules")]
    for fn in files:
        fp = os.path.join(root, fn)
        try:
            s = open(fp, encoding="utf-8").read()
        except (UnicodeDecodeError, OSError):
            continue
        o = s
        for pat, rep in rules:
            if pat in s:
                s = s.replace(pat, rep)
        for rx, rep in regex_rules:
            s = rx.sub(rep, s)
        if s != o:
            open(fp, "w", encoding="utf-8").write(s)
            changed += 1
print(f"  替换规则：改动 {changed} 个文件（字面 {len(rules)} 条 / 正则 {len(regex_rules)} 条）")
PY

say "4/6 门禁扫描（命中即中止）"
python3 - "$OUT" <<'PY'
import os, re, sys
out = sys.argv[1]
PATTERNS = [
    (r'x3gw-shared\w*',                 "网关令牌明文"),
    (r'21092109',                       "已知机器密码"),
    (r'/Volumes/(AHZ|BXC)',             "私有卷路径"),
    (r'~',                    "私有家目录"),
    (r'Mr2109',                            "人物称谓"),
    (r'\b(192\.168\.\d+\.\d+|10\.0\.\d+\.\d+)\b', "内网地址（未替换）"),
    (r'Mr2109deMac-Studio',               "主机名"),
    (r'-----BEGIN [A-Z ]*PRIVATE KEY-----', "私钥"),
    (r'AKIA[0-9A-Z]{16}',               "AWS Access Key"),
    (r'gh[pousr]_[A-Za-z0-9]{20,}',     "GitHub Token"),
    (r'sk-[A-Za-z0-9]{20,}',            "OpenAI 风格 Key"),
]
hits = []
BIG = 5 * 1024 * 1024   # 单文件 >5MB 视为异常（源码快照不该有）
for root, dirs, files in os.walk(out):
    for fn in files:
        fp = os.path.join(root, fn)
        try:
            if os.path.getsize(fp) > BIG:
                hits.append((os.path.relpath(fp, out), 0, "超大文件(>5MB，疑似二进制/资产)", f"{os.path.getsize(fp)//1024//1024}MB"))
        except OSError:
            pass
for root, dirs, files in os.walk(out):
    dirs[:] = [d for d in dirs if d not in (".git", "target", "node_modules")]
    for fn in files:
        fp = os.path.join(root, fn)
        rel = os.path.relpath(fp, out)
        ALLOWED_BINARY = (".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".webp",
                          ".ttf", ".otf", ".woff", ".woff2", ".pdf")
        try:
            txt = open(fp, encoding="utf-8").read()
        except UnicodeDecodeError:
            # 不可按文本解码 = 二进制/编译产物：源码快照里除白名单类型外一律视为可疑
            if not rel.lower().endswith(ALLOWED_BINARY):
                hits.append((rel, 0, "二进制/编译产物（源码快照不应包含）", ""))
            continue
        except OSError:
            continue
        for i, line in enumerate(txt.splitlines(), 1):
            for rx, why in PATTERNS:
                if re.search(rx, line):
                    hits.append((rel, i, why, line.strip()[:100]))
print(f"  扫描完成：命中 {len(hits)} 处")
for rel, i, why, sample in hits[:40]:
    print(f"    ❌ {why}  {rel}:{i}\n       {sample}")
if hits:
    sys.exit(1)
PY
if command -v gitleaks >/dev/null 2>&1; then
  echo "  → 追加 gitleaks 通用密钥扫描"
  if [ -f "$OUT/.gitleaks.toml" ]; then
    ( cd "$OUT" && gitleaks detect --no-git --redact -v -c .gitleaks.toml ) || { echo "❌ gitleaks 命中——中止发布" >&2; exit 1; }
  else
    ( cd "$OUT" && gitleaks detect --no-git --redact -v ) || { echo "❌ gitleaks 命中——中止发布" >&2; exit 1; }
  fi
else
  echo "  （未安装 gitleaks——跳过通用密钥扫描；brew install gitleaks 可启用）"
fi

if [ "$DRY_RUN" = "1" ]; then
  say "dry-run：完成导出/变换/扫描，停在 git 之前。目录：$OUT"
  exit 0
fi

# 4b. 文档双语门禁（配对完整 / 标题结构对齐 / 链接有效 / 源声明）
if [ -f "$REPO_ROOT/scripts/check_docs.py" ]; then
  if python3 "$REPO_ROOT/scripts/check_docs.py" "$OUT"; then
    echo "  文档门禁: 通过"
  else
    echo "  ❌ 文档门禁未通过——已中止导出" >&2
    exit 1
  fi
fi

# 4c. 版本号一致性门禁（UI ↔ Go 单一来源）
if [ -f "$REPO_ROOT/scripts/check_version.py" ]; then
  if python3 "$REPO_ROOT/scripts/check_version.py" "$REPO_ROOT" >/dev/null; then
    echo "  版本号门禁: 通过"
  else
    echo "  ❌ 版本号不一致——已中止导出" >&2
    python3 "$REPO_ROOT/scripts/check_version.py" "$REPO_ROOT" >&2
    exit 1
  fi
fi

say "5/6 生成单个压平提交（SQUASH）"
cd "$OUT"
rm -rf .git
git init -q -b main
git add -A
GIT_AUTHOR_NAME="$SNAPSHOT_AUTHOR_NAME" GIT_AUTHOR_EMAIL="$SNAPSHOT_AUTHOR_EMAIL" \
GIT_COMMITTER_NAME="$SNAPSHOT_AUTHOR_NAME" GIT_COMMITTER_EMAIL="$SNAPSHOT_AUTHOR_EMAIL" \
git commit -q -m "Zerg Swarm (虫族) — 公开快照

源码快照，非开发历史。完整开发史保留在私有权威仓。

GitOrigin-RevId: ${PRIV_HEAD}
License: Apache-2.0 (see LICENSE)
Third-party: see THIRD_PARTY_LICENSES.md"
echo "提交: $(git log --oneline -1)"
echo "文件: $(git ls-files | wc -l | tr -d ' ') 个 / 体积: $(du -sh . | cut -f1)"
echo "作者: $(git log -1 --format='%an <%ae>')"

if [ "$PUSH" = "1" ]; then
  say "6/6 推送到公开仓"
  [ -n "$REMOTE" ] || { echo "需要 --remote 或 ZERG_PUBLIC_REMOTE" >&2; exit 2; }
  git remote add origin "$REMOTE" 2>/dev/null || git remote set-url origin "$REMOTE"
  git push -f origin main
  echo "已推送: $REMOTE"
else
  say "6/6 未推送（准备就绪）"
  echo "推送命令：  cd $OUT && git remote add origin <公开仓> && git push -f origin main"
  echo "或重跑：    scripts/publish-public.sh --push --remote <公开仓>"
fi
