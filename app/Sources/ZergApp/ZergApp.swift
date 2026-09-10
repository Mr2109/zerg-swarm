import SwiftUI

/// 虫族集群状态菜单栏应用入口
/// 纯只读界面壳：读取 zerg-core 8580 API，显示集群状态
/// 关闭界面不影响 zerg-core / agent 运行
@main
struct ZergApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        // 空 Settings 场景：保持 SwiftUI App 生命周期，但不创建可见窗口
        Settings {
            EmptyView()
        }
    }
}
