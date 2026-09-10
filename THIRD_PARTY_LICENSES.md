# 第三方组件与许可（Third-Party Licenses）

本项目**自身**以 Apache-2.0 发布（见 `LICENSE`）。以下组件**不随仓库分发**，
由使用者按需自行安装；此处登记是为了合规与可追溯。

| 组件 | 许可 | 用途 | 是否随仓库分发 |
|---|---|---|---|
| [llama.cpp](https://github.com/ggml-org/llama.cpp) | MIT | 本地模型推理后端（`llama-server`） | 否（用户自装；或其 fork，如 K2-Horizon 变体） |
| [ds4-server](https://github.com/ggml-org/llama.cpp) | MIT | 另一推理后端 | 否 |
| [egui / eframe](https://github.com/emilk/egui) | MIT 或 Apache-2.0 | 桌面 UI | 是（作为 crates 依赖） |
| [rtk](https://github.com/rtk-ai/rtk) | Apache-2.0 | 可选的命令输出压缩包装（`ZERG_RTK=0` 可关） | **否**（`brew install rtk` 或跳过——缺装时自动降级） |
| [searxng](https://github.com/searxng/searxng) | **AGPL-3.0** | 联网搜索（`web_search`） | **否**（用户自建；见 `scripts/install-searxng.sh`） |
| [SQLite](https://sqlite.org)（`modernc.org/sqlite` 纯 Go 驱动） | Public Domain / BSD-style | 对话与知识库存储 | 是（作为 Go module 依赖） |
| RapidOCR / ONNX Runtime | Apache-2.0 | 截图 OCR 可选路径 | 否 |
| LLMLingua-2 (ONNX 权重) | MIT | 可选压缩模型 | 否（`ZERG_COMPRESS_MODELS` 指向自备权重） |

> **关于 searxng（AGPL-3.0）**：本项目**不**把它并入仓库、也不与其静态链接——仅通过子进程调用用户自建实例，
> 因此不构成衍生作品。若你要对外提供网络服务，请自行评估 AGPL 义务。
