#!/usr/bin/env python3
"""searx_search.py — 虫族 Agent web_search 的 searxng 桥（v1.0.2——2026-09-02 升级）
直接调 searx 引擎（Hermes 同款思路——不走 HTTP 服务）
用法: python3 searx_search.py "查询词" [max_results] [lang] [time_range]
  lang: all/zh/zh-CN/en/...（ISO 639-1 或 babel locale）
  time_range: day/week/month/year（空=不限）
输出: JSON（标题/URL/摘要/评分 列表）
v1.0.1 变更: ①lang/time_range 参数（SearchQuery 原生支持——中文优先/最新信息）
②批量搜索（searxng 原生引擎容错——坏引擎内部跳过——不逐个串行防慢）
③结果带 score（Go 层可按质量过滤）
v1.0.2 变更（Mr2109）: 每次调用国外优先（settings 代理 7892——Clash 开时 google/bing 等通）——
不通（7892 不可达/引擎全 ConnectError）自动降级国内（去代理临时 settings——百度/360/搜狗直连）——
每次调用都动态判断——不是固定配代理或固定国内
"""
import json
import os
import re
import socket
import sys

# 关键：清除 PYTHONPATH（防 Hermes venv 污染）
os.environ.pop("PYTHONPATH", None)

DEFAULT_SETTINGS = "<repo>/vendor/searxng/searx/settings.yml"
NOPROXY_SETTINGS = "/tmp/searx_noproxy_settings.yml"
PROXY_ADDR = ("127.0.0.1", 7892)

sys.path.insert(0, "<repo>/vendor/searxng")

from flask import Flask  # noqa: E402

app = Flask(__name__)


def proxy_alive():
    """Clash 代理可达？（0.5s 探测——通=国外可走代理）"""
    try:
        s = socket.create_connection(PROXY_ADDR, 0.5)
        s.close()
        return True
    except OSError:
        return False


def make_noproxy_settings():
    """生成去代理临时 settings（国内引擎直连用——去掉 outgoing.proxies 段）"""
    s = open(DEFAULT_SETTINGS, encoding="utf-8").read()
    # 删 proxies 块（proxies: 到 - http://127.0.0.1:7892 之间——含 all:// 子键）
    s2 = re.sub(r"\n  proxies:.*?- http://127\.0\.0\.1:7892", "\n", s, flags=re.S)
    if s2 != s:
        with open(NOPROXY_SETTINGS, "w", encoding="utf-8") as f:
            f.write(s2)
    return NOPROXY_SETTINGS


def run_search(query, max_results, lang, time_range, settings_path):
    """按指定 settings 执行一次搜索——返回结果列表（空=引擎全失败/无结果）"""
    os.environ["SEARXNG_SETTINGS_PATH"] = settings_path
    # 重新初始化（settings 可能变了——代理/去代理切换）
    from searx.search import initialize, Search  # noqa: F401
    from searx.search.models import SearchQuery, EngineRef  # noqa: F401
    from searx.engines import categories as engine_categories  # noqa: F401
    initialize()

    engineref_list = []
    for engine in engine_categories.get("general", []):
        engineref_list.append(EngineRef(engine.name, "general"))
    if not engineref_list:
        return []

    sq = SearchQuery(query, engineref_list, lang=lang, safesearch=0, pageno=1,
                     time_range=time_range if time_range else None)
    try:
        with app.test_request_context("/search"):
            rc = Search(sq).search()
        return rc.get_ordered_results()
    except Exception:
        return []


def to_json(results, max_results):
    """结果结构化 + 去重"""
    seen = set()
    out = []
    for r in results:
        if isinstance(r, dict):
            url = r.get("url", "")
            title = r.get("title", "")
            content = r.get("content", "")
            score = r.get("score", 0)
        else:
            url = getattr(r, "url", "")
            title = getattr(r, "title", "")
            content = getattr(r, "content", "")
            score = getattr(r, "score", 0)
        if not url or url in seen:
            continue
        seen.add(url)
        out.append({
            "title": str(title)[:150],
            "url": url,
            "content": str(content)[:300],
            "score": round(float(score or 0), 3),
        })
        if len(out) >= max(1, max_results):
            break
    return out


def main():
    if len(sys.argv) < 2:
        print(json.dumps({"error": "缺少查询词"}, ensure_ascii=False))
        return 1
    query = sys.argv[1]
    max_results = int(sys.argv[2]) if len(sys.argv) > 2 else 5
    lang = sys.argv[3] if len(sys.argv) > 3 else "all"
    time_range = sys.argv[4] if len(sys.argv) > 4 else ""

    # v1.0.2 降级链: 国外优先（代理在→默认 settings）→ 不通自动降级国内（去代理 settings）
    attempts = []
    if proxy_alive():
        attempts.append(("国外(代理)", DEFAULT_SETTINGS))
    # 无论代理是否在——国内兜底都要试（代理在但国外引擎全失败也降级）
    attempts.append(("国内(直连)", make_noproxy_settings()))

    last_results = []
    used_engine = None
    for label, sp in attempts:
        results = run_search(query, max_results, lang, time_range, sp)
        if results:
            last_results = results
            used_engine = label
            break
        # 无结果——继续降级链

    if not last_results:
        print(json.dumps({"results": []}, ensure_ascii=False))
        return 0
    print(json.dumps({"results": to_json(last_results, max_results), "engine": used_engine},
                     ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
