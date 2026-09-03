import SwiftUI
import Combine

// MARK: - 菜单栏管理
class StatusBarManager: NSObject, NSPopoverDelegate {
    static let shared = StatusBarManager()

    private let statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private let popover = NSPopover()
    private var cancellable: AnyCancellable?

    private override init() {
        super.init()
        // transient：点击外部自动关闭，且不要求 App 激活（accessory 模式必须）
        popover.behavior = .transient
        popover.delegate = self
    }

    func setup() {
        updateIcon(state: .idle)

        let hosting = NSHostingController(rootView: ContentView())
        popover.contentViewController = hosting

        let button = statusItem.button
        button?.action = #selector(handleClick)
        button?.target = self
        button?.sendAction(on: [NSEvent.EventTypeMask.leftMouseUp, NSEvent.EventTypeMask.rightMouseUp])

        // 监听 API 客户端状态变化，更新图标颜色
        cancellable = FleetAPI.shared.$fetchState
            .receive(on: DispatchQueue.main)
            .sink { [weak self] state in
                self?.updateIcon(state: state)
            }
    }

    private func updateIcon(state: FleetFetchState) {
        let color: NSColor
        switch state {
        case .idle:    color = .systemGreen
        case .loading: color = .systemOrange
        case .error:   color = .systemRed
        case .ready:   color = .systemGreen
        }
        guard let button = statusItem.button else { return }
        button.image = renderIcon(statusColor: color)
        button.imagePosition = .imageLeft
        // 文字"虫族"（留一格空格分隔图标与文字）
        button.title = " 虫族"
        // 文字颜色跟随状态
        button.attributedTitle = NSAttributedString(
            string: " 虫族",
            attributes: [.foregroundColor: color, .font: NSFont.systemFont(ofSize: 12, weight: .medium)]
        )
        button.toolTip = "虫族集群"
    }

    private func renderIcon(statusColor: NSColor) -> NSImage {
        let img = NSImage(size: NSSize(width: 18, height: 18))
        img.lockFocus()

        // 绘制 🦠 字符作为图标
        let font = NSFont.systemFont(ofSize: 14, weight: .medium)
        let emoji = "🦠"
        emoji.draw(at: NSPoint(x: 1, y: 0), withAttributes: [.font: font])

        // 状态指示灯（右上角小圆点）
        statusColor.setFill()
        NSBezierPath(ovalIn: NSRect(x: 12, y: 10, width: 6, height: 6)).fill()

        img.unlockFocus()
        return img
    }

    @objc private func handleClick() {
        guard let event = NSApp.currentEvent else {
            togglePopover()
            return
        }
        if event.type == .rightMouseUp {
            showMenu()
        } else {
            togglePopover()
        }
    }

    private func togglePopover() {
        if popover.isShown {
            popover.performClose(nil)
        } else {
            guard let button = statusItem.button else { return }
            popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
            popover.contentViewController?.view.window?.makeKey()
        }
    }

    private func showMenu() {
        let menu = NSMenu()
        menu.addItem(NSMenuItem(title: "刷新数据", action: #selector(refreshData), keyEquivalent: "r"))
        menu.addItem(NSMenuItem.separator())
        menu.addItem(NSMenuItem(title: "退出", action: #selector(quitApp), keyEquivalent: "q"))
        statusItem.menu = menu
        statusItem.button?.performClick(nil)
        statusItem.menu = nil
    }

    @objc private func refreshData() {
        FleetAPI.shared.fetchAll()
    }

    @objc private func quitApp() {
        NSApplication.shared.terminate(nil)
    }
}
