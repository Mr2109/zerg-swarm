import Foundation
import Combine
import AppKit

// MARK: - API 配置
// 2026-09-11 开源清理：端点与 token 不再写死
//   端点：环境变量 ZERG_API_BASE（默认本机 8580）
//   token：环境变量 ZERG_TOKEN → UserDefaults "zergToken" → 空
private let baseURL = ProcessInfo.processInfo.environment["ZERG_API_BASE"] ?? "http://127.0.0.1:8580"
private let authToken = ProcessInfo.processInfo.environment["ZERG_TOKEN"]
    ?? UserDefaults.standard.string(forKey: "zergToken") ?? ""

// MARK: - 数据模型
struct FleetStatus: Codable {
    let machines: [String: MachineInfo]
}

struct MachineInfo: Codable, Identifiable {
    let id: String?
    let machine: String?
    let online: Bool?
    let model: String?
    let backend_state: String?
    let mem_available_gb: Double?
    let mem_total_gb: Double?
    let load: Double?
    let models: [String]?
    let uptime: Double?
    let gpu_used_gb: Double?
    let gpu_temp_c: Double?
    let backend_rss_gb: Double?
    let active_requests: Int?
    let healthy: Bool?
    let last_seen: String?
    let cpu_pct: Double?   // B4 v2：CPU 使用率 %
    let gpu_pct: Double?   // B4 v2：GPU 使用率 %

    // Identifiable 由存储属性 id: String? 满足（主控 JSON 无 id 字段时为 nil）

    /// 在线判定：最近心跳 15 秒内视为在线（主控 JSON 无 online 字段）
    var isOnline: Bool {
        if let online = online { return online }
        guard let lastSeen = last_seen,
              let date = ISO8601DateFormatter().date(from: lastSeen) else {
            return false
        }
        return Date().timeIntervalSince(date) < 15
    }
}

struct FleetModel: Codable, Identifiable {
    let id: String
    let host: String?
    let backend: String?
    let mem_gb: Double?
}

struct FleetModels: Codable {
    let models: [FleetModel]
    let count: Int?
}

struct FleetTasks: Codable {
    let tasks: [FleetTask]
}

struct FleetTask: Codable, Identifiable {
    let id: String
    let machine: String?
    let model: String?
    let status: String?
    let started_at: String?
    let created_at: String?
}

// MARK: - 数据加载状态
enum FleetFetchState {
    case idle
    case loading
    case ready(status: FleetStatus, models: [FleetModel], tasks: [FleetTask])
    case error(String)
}

// MARK: - API 客户端
/// 虫族集群状态 API 客户端
/// 定时刷新 zerg-core 8680 的 /api/fleet/status + /api/fleet/models + /api/fleet/tasks
class FleetAPI: ObservableObject {
    static let shared = FleetAPI()

    @Published private(set) var fetchState: FleetFetchState = .idle

    // B4 v2：负载/内存采样历史（SparkLine 数据源——最多 60 点 = 5s×60 = 5分钟）
    @Published private(set) var loadHistory: [Double] = []
    @Published private(set) var memHistory: [Double] = []
    private let historyLimit = 60

    // B4 v2：健康汇总（状态栏 tooltip / 标题栏）
    var healthyCount: Int {
        guard case .ready(let status, _, _) = fetchState else { return 0 }
        return status.machines.values.filter { $0.healthy == true }.count
    }
    var totalMachines: Int {
        guard case .ready(let status, _, _) = fetchState else { return 0 }
        return status.machines.count
    }
    var activeRequests: Int {
        guard case .ready(let status, _, _) = fetchState else { return 0 }
        return status.machines.values.reduce(0) { $0 + ($1.active_requests ?? 0) }
    }
    // 告警列表（不健康/离线/高负载/低内存）
    var alerts: [String] {
        guard case .ready(let status, _, _) = fetchState else { return [] }
        var result: [String] = []
        for (name, m) in status.machines {
            if m.healthy == false { result.append("\(name): 不健康") }
            if !m.isOnline { result.append("\(name): 离线") }
            if let load = m.load, load > 4 { result.append("\(name): 高负载 \(String(format: "%.1f", load))") }
            if let mem = m.mem_available_gb, mem < 10 { result.append("\(name): 低内存 \(Int(mem))G") }
        }
        return result
    }

    // B4 v2：排除本机模式切换（调主控端点——路由跳过 local）
    func setExcludeLocal(_ exclude: Bool) {
        guard let url = URL(string: baseURL + "/api/fleet/exclude-local") else { return }
        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.setValue(authToken, forHTTPHeaderField: "X-Auth-Token")
        req.httpBody = try? JSONSerialization.data(withJSONObject: ["exclude": exclude])
        URLSession.shared.dataTask(with: req) { _, _, _ in }.resume()
    }

    /// 启动时读主控排除本机状态（App 开关与主控同步——防止显示开但实际关）
    func fetchExcludeLocal(completion: @escaping (Bool) -> Void = { _ in }) {
        guard let url = URL(string: baseURL + "/api/fleet/exclude-local") else { return }
        var req = URLRequest(url: url)
        req.setValue(authToken, forHTTPHeaderField: "X-Auth-Token")
        URLSession.shared.dataTask(with: req) { data, _, _ in
            guard let data = data,
                  let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                  let value = json["exclude_local"] as? Bool else { return }
            DispatchQueue.main.async {
                completion(value)
            }
        }.resume()
    }

    private var timer: Timer?
    private var cancellables = Set<AnyCancellable>()

    private let decoder: JSONDecoder = {
        let d = JSONDecoder()
        // 注意：模型字段与 JSON 键同名（snake_case），不需要 convertFromSnakeCase——
        // 该策略会把 JSON 的 mem_available_gb 映射到 memAvailableGb，导致字段丢失
        return d
    }()

    // MARK: - 初始化
    func setup() {
        // 首次拉取
        fetchAll()
        // 定时刷新：每 5 秒
        timer = Timer.scheduledTimer(withTimeInterval: 5.0, repeats: true) { [weak self] _ in
            self?.fetchAll()
        }
    }

    // MARK: - 拉取全部数据
    func fetchAll() {
        // 首次才显示 loading（后续刷新保留旧数据——避免界面每5s闪动）
        if case .idle = fetchState {
            fetchState = .loading
        }

        let group = DispatchGroup()
        var statusResult: Result<FleetStatus, Error>?
        var modelsResult: Result<FleetModels, Error>?
        var tasksResult: Result<FleetTasks, Error>?

        // 并发拉取三个端点
        group.enter(); fetchStatus(group: group) { result in statusResult = result }
        group.enter(); fetchModels(group: group) { result in modelsResult = result }
        group.enter(); fetchTasks(group: group) { result in tasksResult = result }

        group.notify(queue: .main) { [weak self] in
            guard let self = self else { return }

            if let status = statusResult, case .success(let s) = status,
               let models = modelsResult, case .success(let m) = models,
               let tasks = tasksResult, case .success(let t) = tasks {
                self.fetchState = .ready(status: s, models: m.models, tasks: t.tasks)
                // B4 v2：采样历史（SparkLine——取本机负载/内存）
                if let local = s.machines["local"] {
                    if let load = local.load { self.loadHistory.append(load) }
                    if let mem = local.mem_available_gb { self.memHistory.append(mem) }
                    if self.loadHistory.count > self.historyLimit { self.loadHistory.removeFirst(self.loadHistory.count - self.historyLimit) }
                    if self.memHistory.count > self.historyLimit { self.memHistory.removeFirst(self.memHistory.count - self.historyLimit) }
                }
            } else {
                // 构造错误信息
                var errors: [String] = []
                if let status = statusResult, case .failure(let e) = status { errors.append("status: \(e.localizedDescription)") }
                if let models = modelsResult, case .failure(let e) = models { errors.append("models: \(e.localizedDescription)") }
                if let tasks = tasksResult, case .failure(let e) = tasks { errors.append("tasks: \(e.localizedDescription)") }
                self.fetchState = .error(errors.joined(separator: " | "))
            }
        }
    }

    // MARK: - 单个端点拉取
    private func fetchStatus(group: DispatchGroup, completion: @escaping (Result<FleetStatus, Error>) -> Void) {
        fetchJSON(urlPath: "/api/fleet/status", group: group) { data in
            do {
                let status = try self.decoder.decode(FleetStatus.self, from: data)
                completion(.success(status))
            } catch {
                completion(.failure(error))
            }
        }
    }

    private func fetchModels(group: DispatchGroup, completion: @escaping (Result<FleetModels, Error>) -> Void) {
        fetchJSON(urlPath: "/api/fleet/models", group: group) { data in
            do {
                let models = try self.decoder.decode(FleetModels.self, from: data)
                completion(.success(models))
            } catch {
                completion(.failure(error))
            }
        }
    }

    private func fetchTasks(group: DispatchGroup, completion: @escaping (Result<FleetTasks, Error>) -> Void) {
        fetchJSON(urlPath: "/api/fleet/tasks", group: group) { data in
            do {
                let tasks = try self.decoder.decode(FleetTasks.self, from: data)
                completion(.success(tasks))
            } catch {
                completion(.failure(error))
            }
        }
    }

    // MARK: - 通用 JSON 请求
    private func fetchJSON(urlPath: String, group: DispatchGroup,
                           completion: @escaping (Data) -> Void) {
        let url = URL(string: "\(baseURL)\(urlPath)")!
        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue(authToken, forHTTPHeaderField: "X-Auth-Token")
        request.timeoutInterval = 3

        let task = URLSession.shared.dataTask(with: request) { data, response, error in
            defer { group.leave() }
            if let error = error {
                print("[ZergApp] API 请求失败 \(urlPath): \(error.localizedDescription)")
                completion(Data())
                return
            }
            guard let data = data else {
                print("[ZergApp] API 返回空数据 \(urlPath)")
                completion(Data())
                return
            }
            completion(data)
        }
        task.resume()
    }
}

// MARK: - 控制端点（加载/卸载/退出主控）

/// 控制操作错误（携带可读消息）
struct ControlError: Error, LocalizedError {
let message: String
var errorDescription: String? { message }
}

/// 主控状态
struct CoreStatus: Codable {
let ok: Bool?
let pid: Int?
let started_at: String?
let version: String?
let local_backend: String?
}

/// 主控日志
struct CoreLogs: Codable {
let logs: [String]?
let path: String?
}

extension FleetAPI {
    /// 加载模型到指定机器（machine: "local"/"x3"/"mini1"）
    func loadModel(machine: String, model: String, completion: @escaping (Result<Bool, ControlError>) -> Void) {
        postJSON(urlPath: "/api/control/load",
                 body: ["machine": machine, "model": model],
                 completion: completion)
    }

    /// 卸载指定机器的模型
    func unloadModel(machine: String, completion: @escaping (Result<Bool, ControlError>) -> Void) {
        postJSON(urlPath: "/api/control/unload",
                 body: ["machine": machine],
                 completion: completion)
    }

    /// 退出主控（zerg-core 优雅退出）
    func stopCore(completion: @escaping (Result<Bool, ControlError>) -> Void) {
        postJSON(urlPath: "/api/control/stop",
                 body: [:],
                 completion: completion)
    }

    /// 拉取主控状态
    func fetchCoreStatus(completion: @escaping (CoreStatus?) -> Void) {
        getJSON(urlPath: "/api/core/status") { data in
            guard let data = data else { completion(nil); return }
            let d = JSONDecoder()
            completion(try? d.decode(CoreStatus.self, from: data))
        }
    }

    /// 拉取主控日志
    func fetchCoreLogs(completion: @escaping ([String]) -> Void) {
        getJSON(urlPath: "/api/core/logs") { data in
            guard let data = data else { completion([]); return }
            let d = JSONDecoder()
            if let logs = try? d.decode(CoreLogs.self, from: data) {
                completion(logs.logs ?? [])
            } else {
                completion([])
            }
        }
    }

    // MARK: - 通用 POST
    private func postJSON(urlPath: String, body: [String: Any],
                          completion: @escaping (Result<Bool, ControlError>) -> Void) {
        let url = URL(string: "\(baseURL)\(urlPath)")!
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue(authToken, forHTTPHeaderField: "X-Auth-Token")
        request.timeoutInterval = 10

        do {
            request.httpBody = try JSONSerialization.data(withJSONObject: body)
        } catch {
            completion(.failure(ControlError(message: "请求体序列化失败: \(error.localizedDescription)")))
            return
        }

        let task = URLSession.shared.dataTask(with: request) { data, response, error in
            if let error = error {
                completion(.failure(ControlError(message: "请求失败: \(error.localizedDescription)")))
                return
            }
            guard let http = response as? HTTPURLResponse else {
                completion(.failure(ControlError(message: "无响应")))
                return
            }
            guard (200..<300).contains(http.statusCode) else {
                let msg = data.flatMap { String(data: $0, encoding: .utf8) } ?? ""
                completion(.failure(ControlError(message: "HTTP \(http.statusCode): \(msg)")))
                return
            }
            completion(.success(true))
        }
        task.resume()
    }

    // MARK: - 通用 GET（返回 Data，失败返回 nil）
    private func getJSON(urlPath: String, completion: @escaping (Data?) -> Void) {
        let url = URL(string: "\(baseURL)\(urlPath)")!
        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue(authToken, forHTTPHeaderField: "X-Auth-Token")
        request.timeoutInterval = 5

        let task = URLSession.shared.dataTask(with: request) { data, response, error in
            if let error = error {
                print("[ZergApp] GET 失败 \(urlPath): \(error.localizedDescription)")
                completion(nil)
                return
            }
            guard let data = data else {
                completion(nil)
                return
            }
            completion(data)
        }
        task.resume()
    }
}
