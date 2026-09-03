import SwiftUI
import Combine

// MARK: - AppDelegate（AppKit 控制菜单栏，绕开 MenuBarExtra 兼容问题）
final class AppDelegate: NSObject, NSApplicationDelegate {
    static let shared = AppDelegate()
    private var cancellable: AnyCancellable?

    func applicationDidFinishLaunching(_ notification: Notification) {
        // 纯菜单栏应用：不占 Dock
        NSApp.setActivationPolicy(.accessory)

        // 初始化 API 客户端并启动定时刷新
        FleetAPI.shared.setup()

        // 菜单栏：NSStatusBar + NSPopover 方案（与 controller 同款，可靠）
        StatusBarManager.shared.setup()
    }
}
