import SwiftUI

// MARK: - 主视图
struct ContentView: View {
    @ObservedObject var api = FleetAPI.shared

    var body: some View {
        VStack(spacing: 0) {
            // ═══ 顶部：状态栏 + 主控操作按钮 ═══
            HeaderView()

            Divider()

            // ═══ B4 v2：TabView（总览/模型/告警/日志）═══
            TabView {
                // 总览：健康条 + 机器卡片 + 趋势
                OverviewTab()
                    .tabItem { Label("总览", systemImage: "gauge") }
                // 模型：加载/候选
                ModelsSection()
                    .tabItem { Label("模型", systemImage: "cpu") }
                // 告警：阈值列表
                AlertsSection()
                    .tabItem { Label("告警", systemImage: "exclamationmark.triangle") }
                // 日志：主控日志尾部
                LogsSection()
                    .tabItem { Label("日志", systemImage: "doc.text") }
            }
            .frame(minWidth: 460, minHeight: 520)
        }
        .frame(minWidth: 460, minHeight: 560)
    }
}

// MARK: - 总览 Tab（B4 v2：健康条 + 机器卡片 + SparkLine）
struct OverviewTab: View {
    @ObservedObject var api = FleetAPI.shared

    var body: some View {
        ScrollView {
            VStack(spacing: 0) {
                ExcludeLocalToggle()
                Divider()
                HealthBarView()
                Divider()
                MachinesSection()
                Divider()
                TrendSection()
            }
        }
    }
}

// MARK: - 排除本机开关（B4 v2：工作时路由跳过本机——任务全走远程）
struct ExcludeLocalToggle: View {
    @ObservedObject var api = FleetAPI.shared
    @State private var excludeLocal = false

    var body: some View {
        Toggle(isOn: $excludeLocal) {
            HStack(spacing: 6) {
                Image(systemName: excludeLocal ? "shield.fill" : "shield")
                    .foregroundColor(excludeLocal ? .orange : .secondary)
                VStack(alignment: .leading, spacing: 1) {
                    Text("排除本机（工作模式）")
                        .font(.system(.caption, weight: .semibold))
                    Text("路由跳过本机——任务全走远程，不薅本机资源")
                        .font(.system(size: 9))
                        .foregroundColor(.secondary)
                }
            }
        }
        .onAppear {
            // 启动时同步主控开关状态（App 显示与实际一致）
            api.fetchExcludeLocal { value in
                excludeLocal = value
            }
        }
        .toggleStyle(.switch)
        .controlSize(.small)
        .padding(.horizontal, 12)
        .padding(.vertical, 6)
        .background(excludeLocal ? Color.orange.opacity(0.1) : Color.clear)
        .onChange(of: excludeLocal) { _, newValue in
            api.setExcludeLocal(newValue)
        }
    }
}

// MARK: - 健康条（B4 v2：在线/请求/告警红点）
struct HealthBarView: View {
    @ObservedObject var api = FleetAPI.shared

    var body: some View {
        HStack(spacing: 8) {
            // 告警红点（claude-stats Events 式）
            if !api.alerts.isEmpty {
                Circle()
                    .fill(Color.red)
                    .frame(width: 8, height: 8)
                    .shadow(color: .red.opacity(0.6), radius: 2)
            }
            Text("健康 \(api.healthyCount)/\(api.totalMachines)")
                .font(.system(.caption, weight: .semibold))
                .foregroundColor(api.healthyCount == api.totalMachines ? .green : .red)
            Text("· 请求 \(api.activeRequests)")
                .font(.system(.caption))
                .foregroundColor(.secondary)
            Spacer()
            if !api.alerts.isEmpty {
                Text("\(api.alerts.count) 告警")
                    .font(.system(.caption, weight: .semibold))
                    .foregroundColor(.red)
            }
            Text(api.alerts.isEmpty ? "正常" : "异常")
                .font(.system(.caption))
                .foregroundColor(api.alerts.isEmpty ? .green : .red)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 6)
    }
}

// MARK: - 趋势 SparkLine（B4 v2：负载/内存 5s 采样）
struct TrendSection: View {
    @ObservedObject var api = FleetAPI.shared

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text("本机趋势（5分钟）")
                    .font(.system(.caption, weight: .semibold))
                    .foregroundColor(.secondary)
                    .textCase(.uppercase)
                Spacer()
            }
            .padding(.horizontal, 12)
            .padding(.top, 8)

            HStack(spacing: 16) {
                // 负载
                VStack(alignment: .leading, spacing: 2) {
                    Text("负载")
                        .font(.system(.caption2))
                        .foregroundColor(.secondary)
                    SparkLine(data: api.loadHistory, color: .orange)
                        .frame(height: 24)
                }
                // 内存
                VStack(alignment: .leading, spacing: 2) {
                    Text("内存可用 G")
                        .font(.system(.caption2))
                        .foregroundColor(.secondary)
                    SparkLine(data: api.memHistory, color: .blue)
                        .frame(height: 24)
                }
            }
            .padding(.horizontal, 12)
            .padding(.bottom, 8)
        }
    }
}

// MARK: - SparkLine 迷你曲线（klimax-ui 范式——无 Charts 依赖的简易折线）
struct SparkLine: View {
    let data: [Double]
    let color: Color

    var body: some View {
        GeometryReader { geo in
            if data.count >= 2 {
                let minV = data.min() ?? 0
                let maxV = data.max() ?? 1
                let range = max(maxV - minV, 0.001)
                let step = geo.size.width / CGFloat(data.count - 1)
                let points = data.enumerated().map { i, v in
                    CGPoint(x: CGFloat(i) * step,
                            y: geo.size.height - CGFloat((v - minV) / range) * geo.size.height)
                }
                Path { p in
                    p.move(to: points[0])
                    for pt in points.dropFirst() { p.addLine(to: pt) }
                }
                .stroke(color, lineWidth: 1.5)
            } else {
                Text("采样中...")
                    .font(.system(size: 8))
                    .foregroundColor(.secondary)
            }
        }
    }
}

// MARK: - 告警 Tab（B4 v2：阈值告警列表）
struct AlertsSection: View {
    @ObservedObject var api = FleetAPI.shared

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text("告警（阈值：不健康/离线/负载>4/内存<10G）")
                    .font(.system(.caption, weight: .semibold))
                    .foregroundColor(.secondary)
                    .textCase(.uppercase)
                Spacer()
            }
            .padding(.horizontal, 12)
            .padding(.top, 8)

            if api.alerts.isEmpty {
                HStack {
                    Image(systemName: "checkmark.circle.fill")
                        .foregroundColor(.green)
                    Text("全部正常")
                        .font(.system(.caption))
                        .foregroundColor(.secondary)
                }
                .padding(.horizontal, 12)
                .padding(.top, 4)
            } else {
                ScrollView {
                    LazyVStack(spacing: 4) {
                        ForEach(api.alerts, id: \.self) { alert in
                            HStack(spacing: 6) {
                                Image(systemName: "exclamationmark.triangle.fill")
                                    .foregroundColor(.red)
                                    .font(.system(size: 10))
                                Text(alert)
                                    .font(.system(.caption))
                                    .foregroundColor(.red)
                                Spacer()
                            }
                            .padding(.horizontal, 8)
                            .padding(.vertical, 4)
                            .background(Color.red.opacity(0.08))
                            .cornerRadius(4)
                        }
                    }
                    .padding(.horizontal, 12)
                    .padding(.bottom, 8)
                }
            }
            Spacer()
        }
    }
}

// MARK: - 顶部状态栏
struct HeaderView: View {
    @ObservedObject var api = FleetAPI.shared
    @State private var confirmStop = false
    @State private var controlMsg: String?

    var body: some View {
        HStack(spacing: 10) {
            // 状态指示灯
            Circle()
                .fill(statusColor)
                .frame(width: 10, height: 10)
                .shadow(color: statusColor.opacity(0.5), radius: 2)

            VStack(alignment: .leading, spacing: 1) {
                Text("虫族集群")
                    .font(.system(.subheadline, weight: .semibold))
                    .lineLimit(1)

                Text(statusMessage)
                    .font(.system(size: 10))
                    .foregroundColor(.secondary)
                    .lineLimit(1)
            }

            Spacer()

            // 主控操作按钮
            if let msg = controlMsg {
                Text(msg)
                    .font(.system(size: 9))
                    .foregroundColor(.orange)
                    .lineLimit(1)
                    .help(msg)
            }

            // 退出主控
            Button(action: { confirmStop = true }) {
                Image(systemName: "power")
                    .font(.caption)
            }
            .buttonStyle(.plain)
            .help("退出主控 (zerg-core)")
            .alert("确认退出主控？", isPresented: $confirmStop) {
                Button("退出", role: .destructive) {
                    controlMsg = "正在退出主控..."
                    api.stopCore { result in
                        DispatchQueue.main.async {
                            switch result {
                            case .success: controlMsg = "主控已退出"
                            case .failure(let e): controlMsg = "退出失败: \(e)"
                            }
                        }
                    }
                }
                Button("取消", role: .cancel) {}
            } message: {
                Text("zerg-core 将优雅退出，需要时点击启动按钮重新拉起。")
            }

            // 刷新按钮
            Button("🔄") {
                api.fetchAll()
            }
            .buttonStyle(.plain)
            .font(.caption)
            .help("立即刷新")
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(Color(.windowBackgroundColor))
    }

    private var statusColor: Color {
        switch api.fetchState {
        case .idle, .ready: return .green
        case .loading:      return .orange
        case .error:        return .red
        }
    }

    private var statusMessage: String {
        switch api.fetchState {
        case .idle:       return "等待加载..."
        case .loading:    return "数据加载中..."
        case .ready(let s, let m, let t):
            let online = s.machines.values.filter { $0.isOnline }.count
            let total = s.machines.count
            return "\(online)/\(total) 在线 · \(m.count) 模型 · \(t.count) 任务"
        case .error(let msg):
            return "状态加载失败: \(msg)"
        }
    }
}

// MARK: - 主控模块
struct CoreSection: View {
    @ObservedObject var api = FleetAPI.shared
    @State private var coreStatus: CoreStatus?
    @State private var coreLogs: [String] = []
    @State private var confirmStop = false
    @State private var controlMsg: String?
    @State private var showLogs = false
    @State private var lastRefresh = Date()

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            // 标题
            HStack {
                Text("主控")
                    .font(.system(.caption, weight: .semibold))
                    .foregroundColor(.secondary)
                    .textCase(.uppercase)
                Spacer()
                if let msg = controlMsg {
                    Text(msg)
                        .font(.system(size: 8))
                        .foregroundColor(.orange)
                }
            }
            .padding(.horizontal, 12)
            .padding(.top, 8)

            // 状态行
            HStack(spacing: 8) {
                Circle()
                    .fill(coreStatus != nil ? .green : .red)
                    .frame(width: 8, height: 8)

                Text(coreStatus != nil ? "运行中" : "已停止")
                    .font(.system(.caption, weight: .medium))

                if let status = coreStatus {
                    if let pid = status.pid {
                        Text("PID \(pid)")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(.secondary)
                    }
                    if let ver = status.version {
                        Text(ver)
                            .font(.system(size: 9))
                            .foregroundColor(.secondary)
                    }
                    if let lb = status.local_backend {
                        Text("本机后端 \(lb)")
                            .font(.system(size: 9))
                            .foregroundColor(.secondary)
                    }
                } else {
                    Text("主控不可达 — 可尝试本地启动")
                        .font(.system(size: 9))
                        .foregroundColor(.red)
                }

                Spacer()

                // 启动主控（本地拉起，主控不在线时才可用）
                Button("启动") {
                    launchCoreLocal()
                }
                .buttonStyle(.bordered)
                .controlSize(.mini)
                .disabled(coreStatus != nil)

                // 重启：先 stop 再本地拉起
                Button("重启") {
                    confirmStop = true
                }
                .buttonStyle(.bordered)
                .controlSize(.mini)
                .disabled(coreStatus == nil)
                .alert("确认重启主控？", isPresented: $confirmStop) {
                    Button("重启", role: .destructive) {
                        restartCore()
                    }
                    Button("取消", role: .cancel) {}
                } message: {
                    Text("zerg-core 将退出并由界面重新拉起，短暂中断。")
                }

                // 退出主控
                Button("退出") {
                    api.stopCore { result in
                        DispatchQueue.main.async {
                            switch result {
                            case .success: controlMsg = "主控已退出"
                            case .failure(let e): controlMsg = "退出失败: \(e)"
                            }
                            refreshCore()
                        }
                    }
                }
                .buttonStyle(.bordered)
                .controlSize(.mini)
                .disabled(coreStatus == nil)

                // 日志开关
                Button(showLogs ? "隐藏日志" : "显示日志") {
                    withAnimation { showLogs.toggle() }
                    if showLogs { refreshCore() }
                }
                .buttonStyle(.bordered)
                .controlSize(.mini)
            }
            .padding(.horizontal, 12)

            // 日志区
            if showLogs {
                VStack(alignment: .leading, spacing: 2) {
                    HStack {
                        Text("主控日志（\(coreLogs.count) 行）")
                            .font(.system(size: 8))
                            .foregroundColor(.secondary)
                        Spacer()
                        Button("🔄") { refreshCore() }
                            .buttonStyle(.plain)
                            .font(.caption2)
                    }
                    .padding(.horizontal, 12)

                    ScrollView {
                        VStack(alignment: .leading, spacing: 1) {
                            ForEach(coreLogs, id: \.self) { line in
                                Text(line)
                                    .font(.system(size: 8, design: .monospaced))
                                    .foregroundColor(logColor(line))
                                    .lineLimit(2)
                                    .frame(maxWidth: .infinity, alignment: .leading)
                            }
                        }
                        .padding(.horizontal, 12)
                        .padding(.bottom, 8)
                    }
                    .frame(maxHeight: 160)
                    .background(Color.black.opacity(0.05))
                }
            }
        }
        .onAppear { refreshCore() }
        .onReceive(Timer.publish(every: 5, on: .main, in: .common).autoconnect()) { _ in
            // 每 5 秒刷新（与机器状态同步）
            if coreStatus != nil { refreshCore() }
        }
    }

    private func refreshCore() {
        api.fetchCoreStatus { status in
            DispatchQueue.main.async {
                coreStatus = status
                if showLogs {
                    api.fetchCoreLogs { logs in
                        DispatchQueue.main.async { coreLogs = logs }
                    }
                }
            }
        }
    }

    /// 本地拉起主控（主控不在线时用）
    private func launchCoreLocal() {
        controlMsg = "正在启动主控..."
        let task = Process()
        task.executableURL = URL(fileURLWithPath: "/bin/zsh")
        task.arguments = ["-c", "cd '<repo>/core' && nohup go run ./cmd/zerg-core > /tmp/zerg-core.log 2>&1 &"]
        do {
            try task.run()
            DispatchQueue.main.asyncAfter(deadline: .now() + 3) {
                controlMsg = "已请求启动，等待就绪..."
                refreshCore()
            }
        } catch {
            controlMsg = "启动失败: \(error.localizedDescription)"
        }
    }

    /// 重启主控：stop 后本地拉起
    private func restartCore() {
        controlMsg = "正在重启..."
        api.stopCore { _ in
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
                launchCoreLocal()
            }
        }
    }

    private func logColor(_ line: String) -> Color {
        let lower = line.lowercased()
        if lower.contains("error") || lower.contains("fail") || lower.contains("panic") {
            return .red
        }
        if lower.contains("warn") {
            return .orange
        }
        if lower.contains("心跳") || lower.contains("heartbeat") {
            return .secondary
        }
        return .primary
    }
}

// MARK: - 机器卡片列表
struct MachinesSection: View {
    @ObservedObject var api = FleetAPI.shared

    var body: some View {
        Group {
            switch api.fetchState {
            case .idle:
                Text("加载集群状态中...")
                    .font(.caption2)
                    .foregroundColor(.secondary)
                    .frame(maxWidth: .infinity, alignment: .center)
                    .padding()

            case .loading:
                Text("数据加载中...")
                    .font(.caption2)
                    .foregroundColor(.secondary)
                    .frame(maxWidth: .infinity, alignment: .center)
                    .padding()

            case .ready(let status, _, _):
                MachinesList(status: status)

            case .error:
                Text("状态加载失败，请检查 zerg-core 是否运行")
                    .font(.caption2)
                    .foregroundColor(.red)
                    .frame(maxWidth: .infinity, alignment: .center)
                    .padding()
            }
        }
    }
}

// MARK: - 机器卡片
struct MachinesList: View {
    let status: FleetStatus

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            // 分区标题
            HStack {
                Text("机器节点")
                    .font(.system(.caption, weight: .semibold))
                    .foregroundColor(.secondary)
                    .textCase(.uppercase)
                Spacer()
                Text("\(status.machines.count) 台")
                    .font(.system(.caption2))
                    .foregroundColor(.secondary)
            }
            .padding(.horizontal, 12)
            .padding(.top, 8)

            // 机器卡片网格
            LazyVGrid(columns: [GridItem(.flexible(), spacing: 8)], spacing: 8) {
                ForEach(machineIDs, id: \.self) { key in
                    if let machine = status.machines[key] {
                        MachineCard(info: machine, key: key)
                            // B4 告警高亮：不健康/离线机器红色边框突出
                            .padding(4)
                            .background(
                                RoundedRectangle(cornerRadius: 6)
                                    .stroke(
                                        (machine.healthy == false || !machine.isOnline) ? Color.red.opacity(0.6) : Color.clear,
                                        lineWidth: 1.5
                                    )
                            )
                    }
                }
            }
            .padding(.horizontal, 12)
            .padding(.bottom, 8)
        }
    }

    private var machineIDs: [String] {
        // 自定义排序：X3 第一 → local → mini1 → mini2（其余按字母序）
        let priority = ["x3", "local", "mini1", "mini2"]
        return Array(status.machines.keys).sorted { a, b in
            let ia = priority.firstIndex(of: a) ?? Int.max
            let ib = priority.firstIndex(of: b) ?? Int.max
            if ia != ib { return ia < ib }
            return a < b
        }
    }
}

// MARK: - 单台机器卡片（增强：内存/显存/温度/负载/模型列表 + 控制按钮）
struct MachineCard: View {
    @ObservedObject var api = FleetAPI.shared
    let info: MachineInfo
    let key: String
    @State private var selectedModel: String?
    @State private var controlMsg: String?
    @State private var confirmUnload = false

    // 机器可加载的模型（来自 fleet.yaml，通过主控 models 端点过滤 host）
    private var availableModels: [String] {
        guard case .ready(_, let models, _) = api.fetchState else { return [] }
        return models
            .filter { $0.host == key || (key == "local" && $0.host == "local") }
            .map { $0.id }
            .uniqueSorted()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            // 机器名 + 在线状态 + 健康
            HStack(spacing: 4) {
                Image(systemName: info.isOnline ? "circle.fill" : "circle")
                    .foregroundColor(info.isOnline ? .green : .gray)
                    .font(.system(size: 8))

                Text(key)
                    .font(.system(.caption, weight: .medium))
                    .lineLimit(1)
                    .truncationMode(.middle)

                Spacer()

                if let healthy = info.healthy {
                    Image(systemName: healthy ? "checkmark.circle.fill" : "xmark.circle.fill")
                        .foregroundColor(healthy ? .green : .red)
                        .font(.system(size: 10))
                        .help(healthy ? "健康" : "不健康")
                }
            }
            // 模型信息（始终显示：有模型显示模型名，无模型显示"未加载"）
            HStack(spacing: 4) {
                Text("📦")
                    .font(.system(size: 8))
                if let model = info.model, !model.isEmpty {
                    Text(model)
                        .font(.system(size: 9, design: .monospaced))
                        .foregroundColor(.primary)
                        .lineLimit(1)
                        .truncationMode(.middle)
                } else {
                    Text("未加载模型")
                        .font(.system(size: 9))
                        .foregroundColor(.secondary)
                }
            }

            // 后端状态
            if let state = info.backend_state, !state.isEmpty {
                Text("⚙️ \(state)")
                    .font(.system(size: 9))
                    .foregroundColor(.secondary)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }

            // 资源行：内存 / 显存 / 温度 / 负载
            HStack(spacing: 8) {
                // 内存（显示可用/总量，明确标注避免误解为已用）
                HStack(spacing: 2) {
                    Text("💾")
                        .font(.system(size: 8))
                    if let mem = info.mem_available_gb {
                        Text("可用 \(mem, specifier: "%.0f")/\(info.mem_total_gb ?? 0, specifier: "%.0f")G")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(.secondary)
                    } else {
                        Text("--")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(.secondary)
                    }
                }

                // 显存（显示已用，与内存的"可用"区分）
                HStack(spacing: 2) {
                    Text("🖥")
                        .font(.system(size: 8))
                    if let gpu = info.gpu_used_gb, gpu > 0 {
                        Text("已用 \(gpu, specifier: "%.1f")G")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(.secondary)
                    } else {
                        Text("已用 0G")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(.secondary)
                    }
                }

                // 温度
                HStack(spacing: 2) {
                    Text("🌡")
                        .font(.system(size: 8))
                    if let temp = info.gpu_temp_c, temp > 0 {
                        Text("\(temp, specifier: "%.0f")°C")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(temp > 80 ? .red : .secondary)
                    } else {
                        Text("--")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(.secondary)
                    }
                }

                Spacer()

                // B4 v2：CPU 使用率 %
                HStack(spacing: 2) {
                    Text("⚡")
                        .font(.system(size: 8))
                    if let cpu = info.cpu_pct, cpu >= 0 {
                        Text("CPU \(cpu, specifier: "%.0f")%")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(cpu > 80 ? .red : .secondary)
                    } else {
                        Text("CPU --")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(.secondary)
                    }
                }

                // B4 v2：GPU 使用率 %
                HStack(spacing: 2) {
                    Text("🖥")
                        .font(.system(size: 8))
                    if let gpuPct = info.gpu_pct, gpuPct >= 0 {
                        Text("GPU \(gpuPct, specifier: "%.0f")%")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(gpuPct > 80 ? .red : .secondary)
                    } else {
                        Text("GPU --")
                            .font(.system(size: 9, design: .monospaced))
                            .foregroundColor(.secondary)
                    }
                }

                // 负载
                if let load = info.load, load > 0 {
                    Text("负载 \(load, specifier: "%.1f")")
                        .font(.system(size: 9, design: .monospaced))
                        .foregroundColor(.secondary)
                }
            }

            // 加载/卸载控制行
            HStack(spacing: 6) {
                // 模型选择
                if !availableModels.isEmpty {
                    Picker("", selection: $selectedModel) {
                        Text("选择模型").tag(String?.none)
                        ForEach(availableModels, id: \.self) { name in
                            Text(name).tag(String?.some(name))
                        }
                    }
                    .labelsHidden()
                    .font(.system(size: 9))
                    .frame(width: 120)
                }

                // 加载按钮
                Button("加载") {
                    guard let model = selectedModel else { return }
                    controlMsg = "加载中..."
                    api.loadModel(machine: key, model: model) { result in
                        DispatchQueue.main.async {
                            switch result {
                            case .success: controlMsg = "已加载 \(model)"
                            case .failure(let e): controlMsg = "加载失败: \(e)"
                            }
                            DispatchQueue.main.asyncAfter(deadline: .now() + 3) {
                                controlMsg = nil
                            }
                        }
                    }
                }
                .buttonStyle(.bordered)
                .controlSize(.mini)
                .disabled(selectedModel == nil)

                // 卸载按钮
                Button("卸载") {
                    confirmUnload = true
                }
                .buttonStyle(.bordered)
                .controlSize(.mini)
                .disabled(!(info.model != nil && !(info.model?.isEmpty ?? true)))
                .alert("确认卸载 \(key) 的模型？", isPresented: $confirmUnload) {
                    Button("卸载", role: .destructive) {
                        controlMsg = "卸载中..."
                        api.unloadModel(machine: key) { result in
                            DispatchQueue.main.async {
                                switch result {
                                case .success: controlMsg = "已卸载"
                                case .failure(let e): controlMsg = "卸载失败: \(e)"
                                }
                                DispatchQueue.main.asyncAfter(deadline: .now() + 3) {
                                    controlMsg = nil
                                }
                            }
                        }
                    }
                    Button("取消", role: .cancel) {}
                } message: {
                    Text("将停止该机器上的后端进程并释放内存。")
                }
            }

            // 操作反馈
            if let msg = controlMsg {
                Text(msg)
                    .font(.system(size: 8))
                    .foregroundColor(.orange)
                    .lineLimit(2)
            }
        }
        .padding(8)
        .frame(minWidth: 250, maxWidth: .infinity, alignment: .leading)
        .background(onlineColor.opacity(0.08))
        .cornerRadius(6)
    }

    private var onlineColor: Color {
        info.online == true ? .green : .gray
    }
}

// MARK: - 数组辅助扩展
extension Array where Element == String {
    /// 去重并排序
    func uniqueSorted() -> [String] {
        var seen = Set<String>()
        return filter { seen.insert($0).inserted }.sorted()
    }
}

// MARK: - 模型列表
struct ModelsSection: View {
    @ObservedObject var api = FleetAPI.shared

    var body: some View {
        Group {
            switch api.fetchState {
            case .idle, .loading:
                EmptyView()

            case .ready(_, let models, _):
                if models.isEmpty {
                    Text("暂无模型")
                        .font(.caption2)
                        .foregroundColor(.secondary)
                        .frame(maxWidth: .infinity, alignment: .center)
                        .padding()
                } else {
                    ScrollView {
                    VStack(alignment: .leading, spacing: 4) {
                        HStack {
                            Text("模型列表")
                                .font(.system(.caption, weight: .semibold))
                                .foregroundColor(.secondary)
                                .textCase(.uppercase)
                            Spacer()
                            Text("\(models.count) 个")
                                .font(.system(.caption2))
                                .foregroundColor(.secondary)
                    }
                        }
                        .padding(.horizontal, 12)
                        .padding(.top, 4)

                        ForEach(models) { model in
                            ModelRow(model: model)
                        }
                        .padding(.horizontal, 12)
                        .padding(.bottom, 4)
                    }
                }

            case .error:
                EmptyView()
            }
        }
    }
}

// MARK: - 模型行
struct ModelRow: View {
    let model: FleetModel

    var body: some View {
        HStack(spacing: 8) {
            // 后端图标
            Image(systemName: modelBackendIcon(backend: model.backend ?? ""))
                .foregroundColor(.accentColor)
                .font(.system(size: 12))
                .frame(width: 20)

            // 模型信息
            VStack(alignment: .leading, spacing: 1) {
                Text(model.id)
                    .font(.system(.caption, weight: .medium))
                    .lineLimit(1)
                    .truncationMode(.middle)

                if let host = model.host, !host.isEmpty {
                    Text(host)
                        .font(.system(size: 9, design: .monospaced))
                        .foregroundColor(.secondary)
                        .lineLimit(1)
                }
            }

            Spacer()

            // 显存
            if let mem = model.mem_gb {
                Text("\(mem, specifier: "%.1f")G")
                    .font(.system(size: 9, design: .monospaced))
                    .foregroundColor(.secondary)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 1)
                    .background(Color.blue.opacity(0.1))
                    .cornerRadius(3)
            }
        }
        .padding(.vertical, 3)
    }

    private func modelBackendIcon(backend: String) -> String {
        switch backend.lowercased() {
        case "llamacpp":    return "cpu"
        case "mlx":         return "memorychip"
        case "vulkan":      return "externaldrive"
        case "metal":       return "externaldrive"
        default:            return "questionmark.circle"
        }
    }
}

// MARK: - 任务列表
struct TasksSection: View {
    @ObservedObject var api = FleetAPI.shared

    var body: some View {
        Group {
            switch api.fetchState {
            case .idle, .loading:
                EmptyView()

            case .ready(_, _, let tasks):
                if tasks.isEmpty {
                    Text("暂无运行中的任务")
                        .font(.caption2)
                        .foregroundColor(.secondary)
                        .frame(maxWidth: .infinity, alignment: .center)
                        .padding()
                } else {
                    VStack(alignment: .leading, spacing: 4) {
                        HStack {
                            Text("当前任务")
                                .font(.system(.caption, weight: .semibold))
                                .foregroundColor(.secondary)
                                .textCase(.uppercase)
                            Spacer()
                            Text("\(tasks.count) 个")
                                .font(.system(.caption2))
                                .foregroundColor(.secondary)
                        }
                        .padding(.horizontal, 12)
                        .padding(.top, 4)

                        ForEach(tasks) { task in
                            TaskRow(task: task)
                        }
                        .padding(.horizontal, 12)
                        .padding(.bottom, 8)
                    }
                }

            case .error:
                EmptyView()
            }
        }
    }
}

// MARK: - 任务行
struct TaskRow: View {
    let task: FleetTask

    var body: some View {
        HStack(spacing: 8) {
            // 状态指示灯
            Circle()
                .fill(taskStatusColor)
                .frame(width: 6, height: 6)

            // 任务信息
            VStack(alignment: .leading, spacing: 1) {
                HStack(spacing: 4) {
                    if let model = task.model, !model.isEmpty {
                        Text(model)
                            .font(.system(.caption, weight: .medium))
                            .lineLimit(1)
                    }
                    if let status = task.status {
                        Text(status)
                            .font(.system(size: 9))
                            .foregroundColor(.secondary)
                    }
                }

                if let machine = task.machine, !machine.isEmpty {
                    Text("🖥 \(machine)")
                        .font(.system(size: 9, design: .monospaced))
                        .foregroundColor(.secondary)
                }
            }

            Spacer()

            // 时间
            if let started = task.started_at {
                Text(formatTimestamp(started))
                    .font(.system(size: 9, design: .monospaced))
                    .foregroundColor(.secondary)
            }
        }
        .padding(.vertical, 3)
    }

    private var taskStatusColor: Color {
        switch (task.status ?? "").lowercased() {
        case "running", "active":   return .green
        case "error", "failed":     return .red
        case "pending", "queued":   return .orange
        default:                    return .gray
        }
    }

    private func formatTimestamp(_ ts: String) -> String {
        // 简单格式：截取前 16 位 (HH:mm:ss)
        let trimmed = ts.trimmingCharacters(in: CharacterSet.whitespaces)
        if let colonIdx = trimmed.firstIndex(of: ":") {
            let before = trimmed[..<colonIdx]
            return String(before)
        }
        return ts
    }
}

// MARK: - 主控日志（独立面板，CoreSection 内也可展开）
struct LogsSection: View {
    @ObservedObject var api = FleetAPI.shared
    @State private var coreLogs: [String] = []
    @State private var expanded = true  // 默认展开（日志 Tab 进来就看到）

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text("主控日志")
                    .font(.system(.caption, weight: .semibold))
                    .foregroundColor(.secondary)
                    .textCase(.uppercase)
                Spacer()
                Text("\(coreLogs.count) 行")
                    .font(.system(.caption2))
                    .foregroundColor(.secondary)
                Button(expanded ? "收起" : "展开") {
                    withAnimation { expanded.toggle() }
                    if expanded { refresh() }
                }
                .buttonStyle(.bordered)
                .controlSize(.mini)
            }
            .padding(.horizontal, 12)
            .padding(.top, 8)

            if expanded {
                ScrollView {
                    VStack(alignment: .leading, spacing: 1) {
                        ForEach(coreLogs, id: \.self) { line in
                            Text(line)
                                .font(.system(size: 8, design: .monospaced))
                                .foregroundColor(logColor(line))
                                .lineLimit(2)
                                .frame(maxWidth: .infinity, alignment: .leading)
                        }
                    }
                    .padding(.horizontal, 12)
                    .padding(.bottom, 8)
                }
                // 日志高度 80%（占 Tab 内容区大部分——主控日志是重点）
                .frame(maxHeight: .infinity)
                .frame(maxHeight: 400)
                .background(Color.black.opacity(0.05))
            } else {
                Text("展开查看主控运行日志（心跳/请求/错误）")
                    .font(.system(size: 9))
                    .foregroundColor(.secondary)
                    .padding(.horizontal, 12)
                    .padding(.bottom, 8)
            }
        }
        .onAppear { if expanded { refresh() } }
        .onReceive(Timer.publish(every: 10, on: .main, in: .common).autoconnect()) { _ in
            if expanded { refresh() }
        }
    }

    private func refresh() {
        api.fetchCoreLogs { logs in
            DispatchQueue.main.async { coreLogs = logs }
        }
    }

    private func logColor(_ line: String) -> Color {
        let lower = line.lowercased()
        if lower.contains("error") || lower.contains("fail") || lower.contains("panic") {
            return .red
        }
        if lower.contains("warn") {
            return .orange
        }
        if lower.contains("心跳") || lower.contains("heartbeat") {
            return .secondary
        }
        return .primary
    }
}
