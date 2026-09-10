# ZergApp — 虫族集群状态菜单栏

macOS 菜单栏小面板，**只读**展示集群状态（各机器在线情况、当前模型、显存占用、负载）。

> **可选 / 实验性**：不参与核心逻辑，不影响主控与子端运行；仅支持 **macOS 14+**。

## 能力

- 菜单栏常驻，一眼看集群
- 展示每台机器：在线/离线、当前模型、显存占用、可用内存、负载
- 可请求本地拉起主控（主控不在线时）

## 构建

```bash
cd app
swift build -c release        # 需 macOS 14+ / Swift 5.9+
swift run ZergApp             # 直接运行
```

打包 `.app`：`swift build -c release` 后按需自行封装 bundle（`app/dist/` 为构建产物，不入库）。

## 配置（环境变量）

| 变量 | 默认 | 说明 |
|---|---|---|
| `ZERG_API_BASE` | `http://127.0.0.1:8580` | 主控 API 地址 |
| `ZERG_TOKEN` | UserDefaults `zergToken` → 空 | 主控共享令牌 |
| `ZERG_REPO_PATH` | `~/zerg-swarm` | 仓库路径（「本地拉起主控」用） |

也可用 `defaults write <bundle-id> zergToken '<token>'` 持久化令牌。

## 边界

- **只读**：不写主控数据、不调度任务
- 主控/子端完全不需要它；不装它一切照常

## 许可

Apache-2.0（见发布仓 `LICENSE`）
