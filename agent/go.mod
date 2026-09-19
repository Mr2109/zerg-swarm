module github.com/Mr2109/zerg-swarm/agent

go 1.23

// G6：与 core/go.mod 同值（单一真源；scripts/gates/check-gotoolchain.py 断言）。
toolchain go1.25.5

require gopkg.in/yaml.v3 v3.0.1

// shared 是主控与子端共用的纯计算模块（资源裁决：《设计-资源管理器》§八）。
// 驻留/驱逐五档必须与主控用同一份实现（同一包、同一测试），否则两侧规则会各自漂移。
require github.com/Mr2109/zerg-swarm/shared v0.0.0

replace github.com/Mr2109/zerg-swarm/shared => ../shared
