#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-doc-meta.py —— 文档 frontmatter 元数据门（6 件套必填 + 值域封闭 + 额外字段禁入）

规格（唯一真源 = 仓内文件，不复制其判定）
  设计稿 `docs/01-设计/设计-文档体系-v1.0.md`（v1.2 稿）
    §3.4 ①  type 值域封闭（6 值）与三条可机器判的判据（枚举 / 目录默认值 / adr 缺 status）
    §3.4 ②  frontmatter 6 件套必填集 + 三条硬规矩（未申报字段 = 错 · schema 用 JSON Schema · type/status 正交）
    §3.4 ④  title ≤ 60 字（本仓标题 p99 = 59）
    §5.6 ②  双语字段（translation_of / source_lang / translation_status / translation_reviewer / ai_assisted）
    附录B ⑲  status 信任层级 = authoritative|draft|historical|speculation（**四值**）
    附录B ⑮  行式键值优先 · ⑲/㉔ ai_assisted = human|ai|mixed
  调研稿 `docs/调研/调研-文档分块落地-分面与规范.md`
    §B1.3 推荐字段集（含 reviewed_at：authoritative 时必填）· §B2 type 值域与目录 → type 映射
    §A1.3 ③ **回填期口径：字段「不合法即红」，「缺失只告警」** ⇒ 本脚本默认 `--missing=warn`
  字段与值域的判定一律从 `docs/site/frontmatter-schema.json` 读出（脚本内不长第二份值域）。

设计取舍（三条，都是被咬过的）
  ① **零三方依赖**：本机 python3 是 3.9.6，且仓根没有依赖清单文件（§16 #9）⇒ 不引 PyYAML，
     frontmatter 只解析 YAML 的一个**严格子集**；用了子集之外的写法（嵌套映射 / 块标量 / 锚点 /
     制表符缩进）一律 **rc=2「不给结论」**，绝不猜 —— 猜出来的判定与假绿同价。
  ② **schema 与判定同源**：schema 里出现本脚本未实现的关键字 ⇒ **rc=2**（否则 schema 加了规则、
     门还是绿的 ⇒ 假覆盖）。
  ③ **不算不合规的类别要显式剔出并计数**：`docs/调研/multi-agent-源码/`（第三方源码摘录）·
     `SKILL.md`（Hermes/技能格式是**另一套** frontmatter 契约：只有 name/description，不带文档
     6 件套）⇒ 剔除不是隐藏，报告里逐类给数。
     ★ 2026-09-19 开发文档分家：`docs/调研/multi-agent-源码/` 已 `cp -a` 到
     `Zerg-内部文档/调研/multi-agent-源码/`，但**源树仍在工作树里**（未跟踪，97 篇 md 在盘上）⇒
     `EXCL_PREFIX` 条目**保留不动**（仓内零命中才删；删了就会把 97 篇第三方摘录拉进扫描域）。
     ★ 原第三类「`docs/虫族文档/`（主树镜像副本 §2.7）」**已于 2026-09-19 退役** ⇒ 条目删除
     （它今天匹配 0 篇 ⇒ 删它不改任何计数；备份见 ~/zerg-backup/…）。

用法
    python3 scripts/check-doc-meta.py                       # 全仓文档面（默认 --missing=warn）
    python3 scripts/check-doc-meta.py --scope formal        # 正式面候选 39 篇（§8.4③）
    python3 scripts/check-doc-meta.py --scope formal --missing=fail   # 第一波回填的验收口径
    python3 scripts/check-doc-meta.py --target docs/常青 --json
    python3 scripts/check-doc-meta.py --list-rules
    python3 scripts/check-doc-meta.py --self-test           # 成对负控（好件必绿 / 坏件必红 / 缺件必 rc=2）
    python3 scripts/check-doc-meta.py --no-self-test ...    # 内部子进程用（防递归）

退出码（三档，与仓内 `scripts/edit-assert` / `precommit-gates.sh` 同语义）
    0 = 全绿（告警不阻断）
    1 = 有不合规项（**只报告，不改文件** —— 本脚本从不写目标树）
    2 = 不给结论（用法错 / 输入或 schema 缺件 / 有不可判定项 / schema 用了未实现的关键字 / 自检未过）
    ★ 优先级 2 > 1 > 0：只要有一条「判不了」，就不许报绿也不许报红。
    ★ 公开树侧（无 `docs/`，开发文档分家后正式面未进公开树）：本 scope = **BLOCKED、不适用** ——
      正式面候选目录一个都不存在 ⇒ rc=2 并逐条打印缺件路径；私有树正常判（判据一字未改）。
"""
import argparse
import datetime
import json
import os
import re
import subprocess
import sys
import tempfile

PY = sys.executable or "python3"

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT_DEFAULT = os.path.dirname(HERE)
SCHEMA_DEFAULT = os.path.join("docs", "site", "frontmatter-schema.json")

DOC_EXT = (".md", ".markdown", ".mdx")

# 扫描排除名单（逐字照抄《清单-命名规范化》§0 的 EXCL/PRE 口径，便于两份报告对账）
# ★ 唯一偏离：`docs/虫族文档/`（主树镜像副本）**已于 2026-09-19 退役**（备份见 ~/zerg-backup/…）
#   ⇒ 死条目删除（零命中条目留着 = 假覆盖）。镜像若重建，请把该条加回本列表。
#   ★ 反向探针（2026-09-19 实测）：删条目前后 --scope repo 扫描域 = 1770 篇、--scope docs = 1580 篇（一致）。
EXCL_DIRS = ("node_modules", "target", "__pycache__", ".history", ".obsidian", "dist",
             ".zerg", "zerg-wt", "venv", ".cargo", ".git")
EXCL_PREFIX = (".git/", "vendor/", "docs/调研/multi-agent-源码/",
               "tools/ocr/venv/")
#   ★ 2026-09-19 开发文档分家：`docs/调研/multi-agent-源码/` 已 `cp -a` 到 `Zerg-内部文档/调研/`，
#     但**源树仍在工作树里**（未跟踪，97 篇 md 在盘上）⇒ 本条**命中未清零 ⇒ 保留不动**
#     （口径：仓内零命中才删）。待源树真正移出仓后再删；剔出类别数不变。
#     剔出类别从 4 类变 3 类（镜像退役那次已先减一类），「剔出要显式计数」的口径不变。

# 冻结区（只登记不改：《清单-命名规范化》§2 + 设计稿 §8.1/§1.3）
#   ★ 2026-09-19 分家：`docs/issues/`（1086 篇引擎数据）整目录已迁至 `Zerg-内部文档/issues/` ⇒ 删第二项，
#     只留仍在仓内的 `docs/项目文档/`（17 套快照）。反向探针：删条目前后 `--scope repo`
#     扫描域 = 1770 篇 **逐位一致**（`docs/issues` 今天在盘上不存在）。
FROZEN_PREFIX = ("docs/项目文档/",)

# 正式面候选第一波（设计稿 §8.4③ / §16.1⑤ = 39 篇 + 双语正式面 docs/zh/ · docs/en/，§5.6②）
FORMAL_PREFIX = ("docs/项目文档/v2.5.10/", "docs/常青/", "docs/skills/", "docs/zh/", "docs/en/")

CORE_SIX = ("title", "type", "status", "source_of_truth", "owner", "updated_at")

# 另一套 frontmatter 契约（技能/工具链），不是文档 6 件套 —— 只剔除、不计不合规
SKILL_CONTRACT_KEYS = frozenset(("name", "description"))

# 目录 → type 默认值（§B2 表；只提供**默认值**，不作判据 ⇒ 不一致只告警 M16）
#   ★ 2026-09-19 分家：`docs/01-设计/` 与下面 UNSURE 里的四个目录**已迁出仓**（`Zerg-内部文档/`）⇒
#     本表**保留并留作反例**（分家后零命中；删了等于少一道「这些目录的 type 默认值」声明）。
TYPE_DEFAULT_EXACT = {
    "docs/skills/": "how-to",
    "docs/01-设计/": "explanation",
}
TYPE_DEFAULT_UNSURE = ("docs/调研/", "docs/02-调研/", "docs/03-评审/", "docs/thunderbolt/",
                       "docs/常青/")  # 规范给了「或」⇒ 本脚本不判（不发明）；前四项分家后零命中

RULES = [
    ("M1", "WARN", "文件无 frontmatter 块（未回填）",
     "§A1.3③「缺失只告警」；--missing=fail 时升为 FAIL（正式面第一波验收口径）"),
    ("M2", "FAIL", "6 件套缺项（title/type/status/source_of_truth/owner/updated_at）",
     "§3.4② 必填集"),
    ("M3", "FAIL", "type 不在封闭六值内",
     "§3.4① 判据①"),
    ("M4", "FAIL", "status 不在封闭四值内",
     "附录B ⑲（authoritative|draft|historical|speculation）"),
    ("M5", "FAIL", "ai_assisted 不在 human|ai|mixed",
     "附录B ㉔"),
    ("M6", "FAIL", "未申报字段（additionalProperties: false）",
     "§3.4② 硬规矩①"),
    ("M7", "FAIL", "字段类型 / 形状不对（非 string、source_of_truth 空串或空表、aliases 重复等）",
     "§3.4② 表 + schema 的 type/oneOf/minLength/uniqueItems"),
    ("M8", "FAIL", "title 空 或 > 60 字（中文按字符计）",
     "§3.4④（本仓标题 p99 = 59）"),
    ("M9", "FAIL", "updated_at 不是 YYYY-MM-DD 或不是合法日期",
     "§3.4② 表（date）"),
    ("M10", "FAIL", "updated_at 晚于今天",
     "§3.4② 表「合法日期，不得晚于今天」——JSON Schema 表达不了 ⇒ 运行期判据"),
    ("M11", "FAIL", "status: authoritative 而缺 reviewed_at",
     "§B1.3 / §3.4② 表（可选字段：authoritative 时必填）"),
    ("M12", "FAIL", "type: adr 而缺 status / 日期",
     "§3.4① 判据③"),
    ("M13", "FAIL", "frontmatter 内重复键（YAML 非法）",
     "仓内教训：重复键让解析器崩/静默取一条"),
    ("M14", "BLOCK", "frontmatter 用了本脚本未建模的写法（嵌套映射 / 块标量 / 锚点 / 制表符缩进）",
     "零三方依赖下的「不给结论」口径"),
    ("M15", "BLOCK", "schema 用了本脚本未实现的关键字",
     "判据同源：schema 加规则而门照绿 = 假覆盖"),
    ("M16", "WARN", "type 与目录默认值不一致",
     "§3.4① 判据②「告警不失败」（允许显式覆盖）"),
]

RULE_NOTE = {}
for _r in RULES:
    RULE_NOTE[_r[0]] = _r

META_KEYS = ("$schema", "$id", "title", "description", "$comment", "examples", "default",
             "deprecated", "readOnly", "writeOnly")
DATA_KEYWORDS = ("type", "enum", "const", "required", "properties", "additionalProperties",
                 "items", "minItems", "maxItems", "uniqueItems", "minLength", "maxLength",
                 "pattern", "format", "oneOf", "anyOf", "allOf", "not", "if", "then", "else")
ENUM_RULE = {"type": "M3", "status": "M4", "ai_assisted": "M5"}


# ── 扫描域 ─────────────────────────────────────────────────────────────
def in_excluded(rel):
    for p in EXCL_PREFIX:
        if rel.startswith(p):
            return True
    return False


def walk_docs(root, targets):
    """返回 [(rel_path, abs_path)]；rel_path 一律 POSIX 风格、相对仓根。"""
    out = []
    for t in targets:
        if os.path.isfile(t):
            rel = os.path.relpath(t, root).replace(os.sep, "/")
            if not in_excluded(rel):
                out.append((rel, os.path.abspath(t)))
            continue
        for dp, dns, fns in os.walk(t):
            rel = os.path.relpath(dp, root).replace(os.sep, "/")
            relp = "" if rel == "." else rel + "/"
            if in_excluded(relp):
                dns[:] = []
                continue
            dns[:] = [d for d in dns if d not in EXCL_DIRS and not d.startswith(".")]
            for f in sorted(fns):
                if f.startswith("."):
                    continue
                if not f.lower().endswith(DOC_EXT):
                    continue
                out.append((relp + f, os.path.join(dp, f)))
    out.sort()
    return out


# ── frontmatter：严格子集解析 ─────────────────────────────────────────
class Undecidable(Exception):
    pass


def split_frontmatter(text):
    """返回 (raw_block_or_None, body_start_line)。只认首行 '---' 形态（YAML frontmatter）。"""
    lines = text.split("\n")
    if not lines or lines[0].strip() != "---":
        return None, 1
    for i in range(1, min(len(lines), 400)):
        s = lines[i].strip()
        if s in ("---", "..."):
            return "\n".join(lines[1:i]), i + 2
    raise Undecidable("frontmatter 起始 '---' 未闭合（前 400 行内无结束行）")


def _scalar(tok, lineno):
    tok = tok.strip()
    if not tok:
        return ""
    if tok[0] in ("&", "*"):
        raise Undecidable("第 %d 行用了 YAML 锚点/别名（& / *）" % lineno)
    if tok[0] == "{":
        raise Undecidable("第 %d 行用了流式映射 {…}" % lineno)
    if tok[0] in ("|", ">") and len(tok) <= 2:
        raise Undecidable("第 %d 行用了块标量（| / >）" % lineno)
    if tok[0] in "\"'":
        if len(tok) < 2 or tok[-1] != tok[0]:
            raise Undecidable("第 %d 行的引号未闭合" % lineno)
        return tok[1:-1]
    # 未加引号的纯量：只剥「空格 + #」形式的行内注释
    m = re.match(r"^(.*?)\s+#", tok)
    if m:
        tok = m.group(1).strip()
    if tok in ("true", "True"):
        return True
    if tok in ("false", "False"):
        return False
    if tok in ("~", "null", "Null"):
        return None
    return tok


def _inline_list(tok, lineno):
    inner = tok.strip()[1:-1]
    items = []
    for part in inner.split(","):
        part = part.strip()
        if not part:
            continue
        items.append(_scalar(part, lineno))
    return items


def parse_frontmatter(raw):
    """把 frontmatter 块解成 dict。子集之外的写法 ⇒ Undecidable（不给结论）。"""
    data = {}
    order = []
    lines = raw.split("\n")
    pending = None          # 正在收集块列表的键
    for idx, line in enumerate(lines, start=1):
        if line.strip() == "" or line.lstrip().startswith("#"):
            continue
        if line.startswith("\t") or re.match(r"^\s+\t", line):
            raise Undecidable("第 %d 行用制表符缩进（YAML 不允许）" % idx)
        m = re.match(r"^(\s*)-\s*(.*)$", line)
        if m:
            if pending is None:
                raise Undecidable("第 %d 行是列表项，但前面没有键头" % idx)
            data[pending].append(_scalar(m.group(2), idx))
            continue
        m = re.match(r"^([A-Za-z0-9_.\-]+):(.*)$", line)
        if not m:
            if line.startswith(" ") or line.startswith("\t"):
                raise Undecidable("第 %d 行是嵌套结构（本脚本只建模顶层键）" % idx)
            raise Undecidable("第 %d 行不是 'key: value' 形态：%r" % (idx, line[:40]))
        if line.startswith(" "):
            raise Undecidable("第 %d 行有缩进（嵌套结构）" % idx)
        key, rest = m.group(1), m.group(2).strip()
        if key in data:
            data[key + "\x00dup"] = True      # 由调用方翻成 M13
            continue
        order.append(key)
        if rest == "":
            data[key] = []
            pending = key
            continue
        pending = None
        if rest[0] == "[":
            if not rest.endswith("]"):
                raise Undecidable("第 %d 行的行内列表未闭合" % idx)
            data[key] = _inline_list(rest, idx)
        else:
            data[key] = _scalar(rest, idx)
    dups = sorted(k[:-4] for k in data if k.endswith("\x00dup"))
    for k in list(data):
        if k.endswith("\x00dup"):
            del data[k]
    return data, dups


# ── 最小 JSON Schema 判定子集 ────────────────────────────────────────
class SchemaUnsupported(Exception):
    pass


def collect_keywords(schema, path="$"):
    """把 schema 里用到的关键字全收出来（含 $ 元键），用于「未实现关键字 ⇒ rc=2」。"""
    found = set()
    if isinstance(schema, dict):
        for k, v in schema.items():
            found.add(k)
            if k in ("properties",) and isinstance(v, dict):
                for sub in v.values():
                    found |= collect_keywords(sub, path)
                continue
            if k in ("items", "additionalProperties", "not", "if", "then", "else"):
                if isinstance(v, (dict, list)):
                    found |= collect_keywords(v, path)
                continue
            if k in ("oneOf", "anyOf", "allOf"):
                if isinstance(v, list):
                    for sub in v:
                        found |= collect_keywords(sub, path)
                continue
    elif isinstance(schema, list):
        for sub in schema:
            found |= collect_keywords(sub, path)
    return found


def _type_ok(value, want):
    if want == "object":
        return isinstance(value, dict)
    if want == "array":
        return isinstance(value, list)
    if want == "string":
        return isinstance(value, str)
    if want == "boolean":
        return isinstance(value, bool)
    if want == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    if want == "number":
        return isinstance(value, (int, float)) and not isinstance(value, bool)
    if want == "null":
        return value is None
    raise SchemaUnsupported("未知 type: %s" % want)


def _valid_date(s):
    m = re.match(r"^(\d{4})-(\d{2})-(\d{2})$", s)
    if not m:
        return False
    try:
        datetime.date(int(m.group(1)), int(m.group(2)), int(m.group(3)))
        return True
    except ValueError:
        return False


def validate(value, schema, prop=None, conj=None):
    """返回 [(rule_id, 定位, 说明)]。prop = 当前字段名（决定规则号）。"""
    out = []
    if not isinstance(schema, dict):
        return out
    if "enum" in schema:
        if value not in schema["enum"]:
            rule = ENUM_RULE.get(prop or "", "M7")
            out.append((rule, prop or "$", "值 %r 不在封闭值域 %s"
                        % (value, "/".join(str(x) for x in schema["enum"]))))
    if "const" in schema and value != schema["const"]:
        out.append(("M7", prop or "$", "值不等于 const %r" % (schema["const"],)))
    if "type" in schema:
        wants = schema["type"] if isinstance(schema["type"], list) else [schema["type"]]
        if not any(_type_ok(value, w) for w in wants):
            out.append(("M7", prop or "$", "类型不符（要 %s，实际 %s）"
                        % ("/".join(wants), type(value).__name__)))
            return out
    if isinstance(value, str):
        if "minLength" in schema and len(value) < schema["minLength"]:
            out.append(("M8" if prop == "title" else "M7", prop,
                        "长度 %d < minLength %d" % (len(value), schema["minLength"])))
        if "maxLength" in schema and len(value) > schema["maxLength"]:
            out.append(("M8" if prop == "title" else "M7", prop,
                        "长度 %d > maxLength %d" % (len(value), schema["maxLength"])))
        if "pattern" in schema and not re.search(schema["pattern"], value):
            out.append(("M9" if prop in ("updated_at", "reviewed_at") else "M7", prop,
                        "%r 不匹配 %s" % (value, schema["pattern"])))
        if schema.get("format") == "date" and not _valid_date(value):
            out.append(("M9" if prop in ("updated_at", "reviewed_at") else "M7", prop,
                        "%r 不是合法日期" % value))
    if isinstance(value, list):
        if "minItems" in schema and len(value) < schema["minItems"]:
            out.append(("M7", prop or "$", "列表长度 %d < minItems %d" % (len(value), schema["minItems"])))
        if "maxItems" in schema and len(value) > schema["maxItems"]:
            out.append(("M7", prop or "$", "列表长度 %d > maxItems %d" % (len(value), schema["maxItems"])))
        if schema.get("uniqueItems") and len(set(json.dumps(x, ensure_ascii=False) for x in value)) != len(value):
            out.append(("M7", prop or "$", "列表有重复项"))
        if "items" in schema:
            for i, item in enumerate(value):
                out += validate(item, schema["items"], prop, conj)
    if isinstance(value, dict):
        if "properties" in schema:
            for k, sub in schema["properties"].items():
                if k in value:
                    out += validate(value[k], sub, k, conj)
        if schema.get("additionalProperties") is False and "properties" in schema:
            extra = [k for k in value if k not in schema["properties"]]
            for k in sorted(extra):
                out.append(("M6", k, "未申报字段（%s 不在 schema 的 properties 里）" % k))
        req = schema.get("required") or []
        for k in req:
            if k not in value:
                out.append(("M2", k, "必填项缺失"))
    for key, rule in (("oneOf", "M7"), ("anyOf", "M7")):
        if key in schema:
            branches = schema[key]
            ok = False
            for b in branches:
                if not validate(value, b, prop, conj):
                    ok = True
                    break
            if not ok:
                out.append((rule, prop or "$", "%s 的 %d 个分支都不满足" % (key, len(branches))))
    if "allOf" in schema:
        for sub in schema["allOf"]:
            out += validate(value, sub, prop, conj)
    if "if" in schema:
        cond_ok = not validate(value, schema["if"], prop, conj)
        branch = schema.get("then") if cond_ok else schema.get("else")
        if branch:
            for v in validate(value, branch, prop, conj):
                if v[0] == "M2" and v[1] == "reviewed_at":
                    v = ("M11", v[1], "status 为 authoritative ⇒ reviewed_at 必填")
                elif v[0] == "M2" and v[1] in ("status", "updated_at"):
                    v = ("M12", v[1], "type 为 adr ⇒ %s 必填" % v[1])
                out.append(v)
    return out


def load_schema(root, schema_path):
    if not os.path.isfile(schema_path):
        raise Undecidable("schema 缺件：%s" % schema_path)
    with open(schema_path, "r", encoding="utf-8") as fh:
        txt = fh.read()
    try:
        schema = json.loads(txt)
    except ValueError as e:
        raise Undecidable("schema 不是合法 JSON：%s" % e)
    used = collect_keywords(schema)
    unknown = sorted(k for k in used
                     if k not in META_KEYS and k not in DATA_KEYWORDS and not k.startswith("x-"))
    if unknown:
        raise SchemaUnsupported("schema 用了本脚本未实现的关键字：%s" % ", ".join(unknown))
    return schema


# ── 单文件判定 ────────────────────────────────────────────────────────
def judge_file(rel, abspath, schema, missing_mode, today):
    """返回 dict(status=ok|violation|blocked|skipped, cls, items=[(rule, 定位, 说明)])"""
    try:
        with open(abspath, "r", encoding="utf-8", errors="replace") as fh:
            text = fh.read()
    except OSError as e:
        return {"status": "blocked", "cls": "IO", "items": [("M15", rel, "读不了：%s" % e)]}
    try:
        raw, _ = split_frontmatter(text)
    except Undecidable as e:
        return {"status": "blocked", "cls": "写法未建模", "items": [("M14", rel, str(e))]}
    if raw is None:
        cls = "技能契约件" if os.path.basename(rel) == "SKILL.md" else "未回填"
        if cls == "技能契约件":
            return {"status": "skipped", "cls": cls, "items": []}
        status = "violation" if missing_mode == "fail" else "warn"
        return {"status": status, "cls": cls, "items": [("M1", rel, "无 frontmatter 块")]}
    try:
        data, dups = parse_frontmatter(raw)
    except Undecidable as e:
        return {"status": "blocked", "cls": "写法未建模", "items": [("M14", rel, str(e))]}
    if set(data.keys()) and set(data.keys()) <= SKILL_CONTRACT_KEYS:
        return {"status": "skipped", "cls": "技能契约件", "items": []}
    items = [("M13", k, "重复键") for k in dups]
    try:
        items += validate(data, schema, None, None)
    except SchemaUnsupported as e:
        return {"status": "blocked", "cls": "schema 不支持", "items": [("M15", rel, str(e))]}
    # 运行期判据：updated_at 不得晚于今天（JSON Schema 表达不了）
    ua = data.get("updated_at")
    if isinstance(ua, str) and _valid_date(ua):
        if datetime.date(*[int(x) for x in ua.split("-")]) > today:
            items.append(("M10", "updated_at", "%s 晚于今天 %s" % (ua, today.isoformat())))
    # 目录默认值（告警）
    d = default_type_for(rel)
    if d and isinstance(data.get("type"), str) and data["type"] != d:
        items.append(("M16", "type", "type=%s 而该目录默认值 = %s" % (data["type"], d)))
    items = [i for i in items if i[0]]
    if any(i[0] in ("M2", "M3", "M4", "M5", "M6", "M7", "M8", "M9", "M10", "M11", "M12", "M13")
           for i in items):
        return {"status": "violation", "cls": "", "items": items}
    if items:
        return {"status": "warn", "cls": "", "items": items}
    return {"status": "ok", "cls": "", "items": []}


def default_type_for(rel):
    """目录 → type 默认值（§B2）。不确定的目录返回 None（不发明判据）。"""
    for pre, t in TYPE_DEFAULT_EXACT.items():
        if rel.startswith(pre):
            return t
    if rel.startswith("docs/项目文档/"):
        base = os.path.basename(rel)
        if re.match(r"^\d{2}-", base) and ("模块" in base or "架构" in base or "设计" in base or "体系" in base):
            return "reference"
        for pre, t in (("使用-", "how-to"), ("变更-", "record"), ("承接项-", "record"),
                       ("文档-代码对照台账", "record"), ("任务表-", "record"),
                       ("进度记录-", "record"), ("靶子表", "record")):
            if base.startswith(pre):
                return t
        return None
    if rel.startswith("docs/常青/"):
        base = os.path.basename(rel)
        if base.startswith("战略-"):
            return "explanation"
        if "接入" in base:
            return "reference"
        return None
    if any(rel.startswith(p) for p in TYPE_DEFAULT_UNSURE):
        return None
    return None


# ── 报告 ─────────────────────────────────────────────────────────────
def run_scan(root, targets, schema, missing_mode, count_frozen, max_examples, today):
    files = walk_docs(root, targets)
    if not files:
        raise Undecidable("扫描域为空（没有任何 %s 文档）" % "/".join(DOC_EXT))
    tally = {"ok": 0, "violation": 0, "warn": 0, "blocked": 0, "skipped": 0}
    frozen = {"ok": 0, "violation": 0, "warn": 0, "blocked": 0, "skipped": 0}
    per_rule = {}
    examples = {}
    skipped_cls = {}
    for rel, ap in files:
        is_frozen = any(rel.startswith(p) for p in FROZEN_PREFIX)
        res = judge_file(rel, ap, schema, missing_mode, today)
        bucket = frozen if (is_frozen and not count_frozen) else tally
        bucket[res["status"]] += 1
        if res["status"] == "skipped":
            skipped_cls[res["cls"]] = skipped_cls.get(res["cls"], 0) + 1
        for rule, loc, msg in res["items"]:
            key = (rule, "frozen" if (is_frozen and not count_frozen) else "live")
            per_rule[key] = per_rule.get(key, 0) + 1
            examples.setdefault(key, []).append("%s → %s: %s" % (rel, loc, msg))
    return {"files": len(files), "tally": tally, "frozen": frozen, "per_rule": per_rule,
            "examples": examples, "skipped_cls": skipped_cls}


def print_report(rep, args):
    t = rep["tally"]
    print("扫描域：%d 篇（%s）" % (rep["files"], "/".join(DOC_EXT)))
    print("  合规 %d · 不合规 %d · 告警 %d · 不可判定 %d · 剔除 %d"
          % (t["ok"], t["violation"], t["warn"], t["blocked"], t["skipped"]))
    if rep["frozen"] and any(rep["frozen"].values()):
        f = rep["frozen"]
        print("  冻结区（docs/项目文档/ · 只登记不改 ⇒ 不计入退码；原第二项 `docs/issues/` 已于"
              " 2026-09-19 分家至 `Zerg-内部文档/`、条目已删）："
              " 合规 %d · 不合规 %d · 告警 %d · 不可判定 %d · 剔除 %d"
              % (f["ok"], f["violation"], f["warn"], f["blocked"], f["skipped"]))
    if rep["skipped_cls"]:
        print("  剔出类别（不算不合规）："
              + " · ".join("%s %d" % (k, v) for k, v in sorted(rep["skipped_cls"].items())))
    for rule, note in sorted(RULE_NOTE.items()):
        n_live = rep["per_rule"].get((rule, "live"), 0)
        n_frozen = rep["per_rule"].get((rule, "frozen"), 0)
        if not n_live and not n_frozen:
            continue
        sev = note[1]
        if rule == "M1" and args.missing == "fail":
            sev = "FAIL*"
        print("  %s [%s] %s —— 命中 %d（冻结区 %d）" % (rule, sev, note[2], n_live, n_frozen))
        for line in rep["examples"].get((rule, "live"), [])[:args.max_examples]:
            print("      · %s" % line)
        if n_live > args.max_examples:
            print("      · …另 %d 条（--max-examples 调整）" % (n_live - args.max_examples))
    if args.missing == "fail":
        print("  ★ FAIL* = 本档由 --missing=fail 提升（默认 warn；§A1.3③ 回填期口径）")


def decide(rep, count_frozen):
    t = rep["tally"]
    if t["blocked"]:
        return 2, "BLOCKED_UNDECIDABLE：%d 篇判不了（不给结论）" % t["blocked"]
    if t["violation"]:
        return 1, "FAIL：%d 篇不合规" % t["violation"]
    return 0, "OK：合规 %d 篇（告警 %d 条不阻断）" % (t["ok"], t["warn"])


# ── 自检（成对负控：好件必绿 / 坏件必红 / 缺件必 rc=2）──────────────────
GOOD_DOC = """---
title: 好件
type: reference
status: draft
source_of_truth: tools/versions.json
owner: Mr2109
updated_at: 2026-09-18
ai_assisted: mixed
---

# 好件
"""

BAD_DOCS = [
    ("bad-missing-owner.md", "owner", "M2",
     "---\ntitle: 缺 owner\ntype: reference\nstatus: draft\nsource_of_truth: x\nupdated_at: 2026-09-18\n---\n"),
    ("bad-type.md", "type", "M3",
     "---\ntitle: 坏 type\ntype: guide\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"),
    ("bad-status.md", "status", "M4",
     "---\ntitle: 坏 status\ntype: reference\nstatus: reviewed\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"),
    ("bad-ai-assisted.md", "ai_assisted", "M5",
     "---\ntitle: 坏披露\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\nai_assisted: robot\n---\n"),
    ("bad-extra.md", "custom", "M6",
     "---\ntitle: 未申报字段\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\ncustom: 1\n---\n"),
    ("bad-shape.md", "source_of_truth", "M7",
     "---\ntitle: 形状不对\ntype: reference\nstatus: draft\nsource_of_truth: \"\"\nowner: o\nupdated_at: 2026-09-18\n---\n"),
    ("bad-title-long.md", "title", "M8",
     "---\ntitle: %s\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n" % ("长" * 61)),
    ("bad-date-fmt.md", "updated_at", "M9",
     "---\ntitle: 日期形态错\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026/09/18\n---\n"),
    ("bad-date-future.md", "updated_at", "M10",
     "---\ntitle: 未来日期\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2099-01-01\n---\n"),
    ("bad-authoritative.md", "reviewed_at", "M11",
     "---\ntitle: 权威但没复审\ntype: reference\nstatus: authoritative\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"),
    ("bad-dup-key.md", "title", "M13",
     "---\ntitle: 第一次\ntitle: 第二次\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"),
]
# M16（目录默认值不一致 = 告警）单列：它的判据依赖**相对仓根的路径**，
# 必须放在一棵有 docs/skills/ 的合成树里跑，见 self_test 的 dirdef 夹具。
WARN_DIR_DEFAULT_DOC = (
    "---\ntitle: 目录默认值不符\ntype: record\nstatus: draft\n"
    "source_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"
)
BLOCKED_DOCS = [
    ("blk-nested.md", "M14",
     "---\ntitle: 嵌套\nmeta:\n  a: 1\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"),
    ("blk-blockscalar.md", "M14",
     "---\ntitle: |\n  多行\n  文本\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"),
]


def _write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)


def _sub(script, *args):
    p = subprocess.run([PY, script] + list(args), stdout=subprocess.PIPE,
                       stderr=subprocess.STDOUT, universal_newlines=True)
    return p.returncode, p.stdout


def self_test(script_path):
    """成对负控。返回 (ok, 行列表)。★ 断言用例条数 —— 条数不符也算不自检。"""
    lines = []
    ok = True
    tmp = tempfile.mkdtemp(prefix="check-doc-meta-", dir="/tmp")
    schema_src = os.path.join(REPO_ROOT_DEFAULT, SCHEMA_DEFAULT)
    if not os.path.isfile(schema_src):
        return False, ["✗ 缺件：真 schema 不存在 %s ⇒ 自检不给结论" % schema_src]
    schema_dir = os.path.join(tmp, "schema")
    schema_dst = os.path.join(schema_dir, os.path.basename(SCHEMA_DEFAULT))
    os.makedirs(schema_dir, exist_ok=True)
    with open(schema_src, "r", encoding="utf-8") as fh:
        schema_text = fh.read()
    _write(schema_dst, schema_text)
    good = os.path.join(tmp, "good")
    _write(os.path.join(good, "a-good.md"), GOOD_DOC)
    bad = os.path.join(tmp, "bad")
    for name, _loc, _rule, text in BAD_DOCS:
        _write(os.path.join(bad, name), text)
    nofm = os.path.join(tmp, "nofm")
    _write(os.path.join(nofm, "legacy-no-fm.md"), "# 没有 frontmatter\n")
    _write(os.path.join(nofm, "legacy-second.md"), "# 第二篇也没有\n")
    blk = os.path.join(tmp, "blocked")
    for name, _rule, text in BLOCKED_DOCS:
        _write(os.path.join(blk, name), text)
    skill = os.path.join(tmp, "skill")
    _write(os.path.join(skill, "SKILL.md"),
           "---\nname: some-skill\ndescription: 技能契约件\n---\n\n# x\n")
    dirdef = os.path.join(tmp, "dirdef")
    _write(os.path.join(dirdef, "docs", "skills", "warn-dir-default.md"), WARN_DIR_DEFAULT_DOC)
    empty = os.path.join(tmp, "empty")
    os.makedirs(empty, exist_ok=True)

    base = ["--schema", schema_dst, "--repo-root", tmp, "--no-self-test"]

    def case(label, expect_rc, args, must_contain=(), must_not_contain=()):
        # argparse 的重复选项以**最后一次**为准 ⇒ 想换 --repo-root 的用例把新值写在 args 尾部
        rc, out = _sub(script_path, *(base + list(args)))
        good_case = (rc == expect_rc)
        for s in must_contain:
            if s not in out:
                good_case = False
        for s in must_not_contain:
            if s in out:
                good_case = False
        lines.append("%s %s（期望 rc=%d，实得 rc=%d）"
                     % ("✓" if good_case else "✗", label, expect_rc, rc))
        if not good_case:
            lines.append("    输出尾部：%s" % out.strip().splitlines()[-4:])
        return good_case

    # ① 好件必绿（--missing=fail ⇒ 好件里不许混进任何无 frontmatter 的件）
    ok &= case("好件必绿", 0, ["--target", good, "--missing=fail"], must_contain=["合规 1", "OK"])
    # ② 坏件必红，且每条命中它自己的规则号（逐条成对）
    for name, loc, rule, _text in BAD_DOCS:
        ok &= case("坏件必红 %s" % name, 1,
                   ["--target", os.path.join(bad, name), "--missing=fail"],
                   must_contain=[rule, loc])
    # ③ 缺件必 rc=2：目标路径不存在 / schema 不存在 / 扫描域为空
    ok &= case("缺件 · 目标路径不存在", 2, ["--target", os.path.join(tmp, "no-such-dir")],
               must_contain=["不给结论"])
    ok &= case("缺件 · schema 不存在", 2,
               ["--schema", os.path.join(tmp, "no-such-schema.json"), "--target", good],
               must_contain=["不给结论"])
    ok &= case("缺件 · 扫描域为空", 2, ["--target", empty], must_contain=["不给结论"])
    # ④ 不可判定 ⇒ rc=2（不是红）
    for name, rule, _text in BLOCKED_DOCS:
        ok &= case("不可判定 %s" % name, 2,
                   ["--target", os.path.join(blk, name), "--missing=fail"],
                   must_contain=[rule])
    # ⑤ 未回填：默认告警不失败 / --missing=fail 升红（同一件、两种口径 → 结论必须不同）
    ok &= case("未回填默认为告警（rc=0）", 0, ["--target", nofm],
               must_contain=["告警 2", "M1"])
    ok &= case("未回填 --missing=fail 为红（rc=1）", 1, ["--target", nofm, "--missing=fail"],
               must_contain=["不合规 2"])
    # ⑥ 技能契约件：剔除、不算不合规、不计入 rc
    ok &= case("技能契约件被剔除", 0, ["--target", skill, "--missing=fail"],
               must_contain=["技能契约件"], must_not_contain=["不合规 1"])
    # ⑦ 目录默认值不一致 = 告警（rc=0）；该判据的 rel 以仓根为基准 ⇒ 用 dirdef 当仓根
    ok &= case("目录默认值不一致只告警", 0,
               ["--target", os.path.join(dirdef, "docs", "skills", "warn-dir-default.md"),
                "--repo-root", dirdef],
               must_contain=["M16", "告警 1"])
    # ⑧ schema 用了未实现关键字 ⇒ rc=2（防 schema 加了规则、门照绿的假覆盖）
    drift = os.path.join(tmp, "schema-drift.json")
    _write(drift, '{"type":"object","required":["title"],"dependencies":{"a":["b"]}}')
    ok &= case("schema 未实现关键字 ⇒ 不给结论", 2,
               ["--schema", drift, "--target", good, "--missing=fail"],
               must_contain=["未实现的关键字"])
    # ⑨ 只报告不改：跑真目标前后整棵夹具树逐字节不变（第 ⑨ 条也算一条用例）
    import hashlib

    def tree_sha(d):
        h = hashlib.sha256()
        for dp, dns, fns in os.walk(d):
            dns.sort()
            for f in sorted(fns):
                p = os.path.join(dp, f)
                h.update(os.path.relpath(p, d).encode())
                with open(p, "rb") as fh:
                    h.update(fh.read())
        return h.hexdigest()

    before = tree_sha(tmp)
    _sub(script_path, *(base + ["--target", bad, "--missing=fail"]))
    after = tree_sha(tmp)
    same = (before == after)
    lines.append("%s 只报告不改（夹具树逐字节不变：%s）"
                 % ("✓" if same else "✗", "一致" if same else "被改动"))
    ok &= same

    expected_cases = 1 + len(BAD_DOCS) + 3 + len(BLOCKED_DOCS) + 2 + 1 + 1 + 1 + 1
    got = sum(1 for ln in lines if ln[:1] in ("✓", "✗"))
    lines.append("自检用例：期望 %d 条 · 实跑 %d 条" % (expected_cases, got))
    if got != expected_cases:
        lines.append("✗ 用例条数不符（有静默截断）")
        ok = False
    lines.append("自检临时目录：%s（未删除）" % tmp)
    return ok, lines


def list_rules():
    print("check-doc-meta.py 规则表（值域与字段一律读 docs/site/frontmatter-schema.json）")
    for rid, sev, what, src in RULES:
        print("  %-4s [%-5s] %s" % (rid, sev, what))
        print("        出处：%s" % src)
    print("  退码：0 全绿 · 1 有不合规项 · 2 不给结论（优先级 2>1>0）；冻结区不计入退码。")


def main(argv):
    ap = argparse.ArgumentParser(add_help=True, description="文档 frontmatter 元数据门（只报告不改）")
    ap.add_argument("targets", nargs="*", help="扫描目标（文件或目录；默认 = 仓根）")
    ap.add_argument("--target", action="append", default=[], help="同上（可多次）")
    ap.add_argument("--repo-root", default=REPO_ROOT_DEFAULT, help="仓根（默认 = 本脚本的上一级）")
    ap.add_argument("--schema", default=None, help="schema 路径（默认 <仓根>/docs/site/frontmatter-schema.json）")
    ap.add_argument("--scope", choices=["repo", "docs", "formal"], default="docs",
                    help="repo=全仓文档面 · docs=仅 docs/ · formal=正式面候选第一波（§8.4③）")
    ap.add_argument("--missing", choices=["warn", "fail"], default="warn",
                    help="无 frontmatter 块的档位（默认 warn；正式面第一波用 fail）")
    ap.add_argument("--count-frozen", action="store_true", help="把冻结区（项目文档/）计入退码；"
                    "`docs/issues/` 已于 2026-09-19 分家至 `Zerg-内部文档/` ⇒ 不再有该区")
    ap.add_argument("--max-examples", type=int, default=5)
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--no-self-test", action="store_true", help="内部子进程用（防递归）")
    args = ap.parse_args(argv)

    if args.list_rules:
        list_rules()
        return 0
    if args.self_test:
        ok, lines = self_test(os.path.abspath(__file__))
        print("check-doc-meta 自检（成对负控：好件必绿 / 坏件必红 / 缺件必 rc=2）")
        for ln in lines:
            print("  " + ln)
        return 0 if ok else 2
    if not args.no_self_test:
        ok, lines = self_test(os.path.abspath(__file__))
        if not ok:
            print("自检未过 ⇒ 拒绝扫真目标（rc=2 不给结论）")
            for ln in lines:
                print("  " + ln)
            return 2

    root = os.path.abspath(args.repo_root)
    schema_path = os.path.abspath(args.schema) if args.schema else os.path.join(root, SCHEMA_DEFAULT)
    targets = [os.path.abspath(x) for x in (list(args.targets) + list(args.target))]
    if not targets:
        targets = [root if args.scope == "repo" else os.path.join(root, "docs")]
    for t in targets:
        if not os.path.exists(t):
            print("BLOCKED：目标不存在 ⇒ 不给结论：%s" % t)
            return 2
    if args.scope == "formal":
        keep = []
        for t in targets:
            for pre in FORMAL_PREFIX:
                p = os.path.join(root, pre)
                if os.path.isdir(p):
                    keep.append(p)
        targets = sorted(set(keep))
        if not targets:
            print("BLOCKED：正式面候选目录一个都不存在 ⇒ 不给结论")
            for pre in FORMAL_PREFIX:
                print("  缺件路径：%s（仓根下解析 = %s）" % (pre, os.path.join(root, pre)))
            print("  口径：公开树侧（无 docs/）本 scope 为 BLOCKED、不适用；私有树正常判（判据一字未改）。")
            return 2
    if not os.path.isdir(root):
        print("BLOCKED：仓根不是目录 ⇒ 不给结论：%s" % root)
        return 2
    try:
        schema = load_schema(root, schema_path)
    except Undecidable as e:
        print("BLOCKED：%s ⇒ 不给结论" % e)
        return 2
    except SchemaUnsupported as e:
        print("BLOCKED：%s ⇒ 不给结论" % e)
        return 2

    today = datetime.date.today()
    try:
        rep = run_scan(root, targets, schema, args.missing, args.count_frozen,
                       args.max_examples, today)
    except Undecidable as e:
        print("BLOCKED：%s ⇒ 不给结论" % e)
        return 2
    rc, verdict = decide(rep, args.count_frozen)
    if args.json:
        print(json.dumps({"rc": rc, "verdict": verdict, "scope": args.scope,
                          "missing_mode": args.missing, "files": rep["files"],
                          "tally": rep["tally"], "frozen": rep["frozen"],
                          "skipped": rep["skipped_cls"],
                          "per_rule": {"%s/%s" % k: v for k, v in sorted(rep["per_rule"].items())}},
                         ensure_ascii=False, indent=2))
        return rc
    print("check-doc-meta —— schema=%s（scope=%s · missing=%s）"
          % (os.path.relpath(schema_path, root) if schema_path.startswith(root) else schema_path,
             args.scope, args.missing))
    print_report(rep, args)
    print(verdict)
    return rc


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
