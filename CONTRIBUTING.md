# 贡献指南（Contributing）

感谢你愿意参与虫族（Zerg Swarm）。

## 开发者原产地证书（DCO）

本项目采用 **DCO**（而非 CLA）——每次提交用 `-s` 签名即表示你确认：

```bash
git commit -s -m "fix: ..."
```

签名会附上 `Signed-off-by: Your Name <you@example.com>`，其含义见
<https://developercertificate.org/>。要点：你有权提交该代码，且同意以本项目的 Apache-2.0 许可发布。

## 提交前自检

```bash
# 主控 + 子端
cd core && go build ./... && go test ./... -count=1
cd ../agent && go build ./... && go test ./...
# 桌面 UI
cd ../ui && cargo test -p zerg-ui
```

四项都过再提 PR。改动涉及工具行为时，请同步更新对应 `tools/<name>.md` 履历与工具简介。

## 提交信息

`<类型>: <一句话>` —— 类型用 `feat` / `fix` / `docs` / `refactor` / `chore` / `test`。
