// swift-tools-version: 5.9
// 虫族集群状态菜单栏应用 (ZergApp)
// 纯只读界面壳——只读 zerg-core 8680 API，不参与核心逻辑

import PackageDescription

let package = Package(
    name: "ZergApp",
    platforms: [.macOS(.v14)],
    products: [
        .executable(name: "ZergApp", targets: ["ZergApp"])
    ],
    targets: [
        .executableTarget(
            name: "ZergApp",
            path: "Sources/ZergApp"
        )
    ]
)
