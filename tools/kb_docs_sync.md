# kb_docs_sync —— 项目文档 → 知识库同步

**版本**：v1.0.0（2026-09-03——v2.5.8 文档体系 t6）
**入口**：tools/kb_docs_sync.py（python3——无三方依赖）

## 作用
版本定稿时把当前版关键文档同步进知识库（kb）——元数据注册表 + content_hash 变更检测——只处理新增/变更——候选条目+核验门控（不直连 knowledge.db——入库走 MCP kb_*）。

## 用法
```bash
python3 tools/kb_docs_sync.py               # dry-run: 检测 → 打印候选清单
# 核验者按候选清单逐条 kb_add（domain=虫族 file_path=来源路径——含版本号）
python3 tools/kb_docs_sync.py --mark 全部   # 全部入库后标记注册表（下次只检测真变更）
```

## 注册表
`<volume-path>`——{文件名: {hash, ts}}——记录已入库状态
候选输出：`<volume-path>`

## 行为
- 关键文档清单（G3）：00-架构/使用-虫族指南/变更-vX.Y.Z/01-主控/04-任务/06-子端/07-UI/本版设计
- 三操作对齐 particula 增量模式：增（新文档候选）/改（hash 变——覆盖入库）/删（版本归档——文件消失即不再扫）
- **同 file_path 覆盖制**：条目 file_path 含版本号（如 docs/项目文档/v2.5.8/00-架构…）——版本升级同步时新条目覆盖旧条目（v2.5.9 的 00-架构 覆盖 v2.5.8 的）——库内永无过期

## 铁律
- 不直连 knowledge.db（入库走 MCP kb_*——核验门控——防低质量条目污染）
- 版本定稿必跑（发布固定环节——code-quality skill 第 7 项联动：文档规整+知识库同步+INDEX 三件套）
