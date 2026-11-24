import Combine
import Dikeccore
import Foundation
import NetworkExtension

/// Persists the selected profile, the auto-bypass winner and the last scan.
final class Store {
    private let d = Shared.defaults

    func presets() -> [Profile] {
        parseJSONArray(MobilePresets()).compactMap { $0 as? [String: Any] }.map(Profile.init).filter { !$0.needsRoot }
    }

    var autoProfile: Profile? {
        get { Profile(string: d.string(forKey: "auto")) }
        set { d.set(newValue?.jsonString, forKey: "auto") }
    }

    var lastScan: [String: Any]? {
        get { d.string(forKey: "lastScan").flatMap { $0.data(using: .utf8) }
                .flatMap { try? JSONSerialization.jsonObject(with: $0) as? [String: Any] } }
        set { d.set(newValue.flatMap { try? JSONSerialization.data(withJSONObject: $0) }
                .map { String(decoding: $0, as: UTF8.self) }, forKey: "lastScan") }
    }

    var selectedID: String {
        get { d.string(forKey: "selected") ?? "tr" }
        set { d.set(newValue, forKey: "selected") }
    }

    var targets: [String] {
        if let s = d.string(forKey: "targets") { return s.split(separator: ",").map(String.init).filter { !$0.isEmpty } }
        return parseJSONArray(MobileDefaultTargets()).compactMap { $0 as? String }
    }

    func all() -> [Profile] { (autoProfile.map { [$0] } ?? []) + presets() }

    func selected() -> Profile { all().first { $0.id == selectedID } ?? presets()[0] }
}

enum Phase { case off, starting, on, stopping }

/// Owns the VPN configuration and talks to the tunnel extension.
@MainActor
final class Tunnel: ObservableObject {
    @Published private(set) var phase: Phase = .off
    @Published private(set) var since: Date?
    @Published private(set) var logs: [String] = []
    @Published var error: String?

    private var manager: NETunnelProviderManager?
    private var observer: Any?
    private var poll: Timer?

    init() {
        observer = NotificationCenter.default.addObserver(forName: .NEVPNStatusDidChange, object: nil, queue: .main) { [weak self] _ in
            Task { @MainActor in self?.sync() }
        }
        Task { await load() }
    }

    private func load() async {
        let managers = (try? await NETunnelProviderManager.loadAllFromPreferences()) ?? []
        manager = managers.first
        sync()
    }

    /// Creates (once) the VPN configuration iOS shows under Settings › VPN.
    private func prepared() async throws -> NETunnelProviderManager {
        let m = manager ?? NETunnelProviderManager()
        let proto = (m.protocolConfiguration as? NETunnelProviderProtocol) ?? NETunnelProviderProtocol()
        proto.providerBundleIdentifier = Shared.tunnelBundleID
        proto.serverAddress = "Dikec DPI"
        m.protocolConfiguration = proto
        m.localizedDescription = "Dikec DPI"
        m.isEnabled = true
        try await m.saveToPreferences()
        try await m.loadFromPreferences() // required before the first start
        manager = m
        return m
    }

    func start(_ profile: Profile) {
        error = nil
        phase = .starting
        Shared.activeProfileJSON = profile.jsonString
        Task {
            do {
                let m = try await prepared()
                try m.connection.startVPNTunnel(options: ["profile": profile.jsonString as NSString])
            } catch {
                self.error = L("ios.installFailed") + "\n" + error.localizedDescription
                self.phase = .off
            }
        }
    }

    func stop() {
        phase = .stopping
        manager?.connection.stopVPNTunnel()
    }

    /// Restarts with another profile if protection is on.
    func switchTo(_ profile: Profile) {
        Shared.activeProfileJSON = profile.jsonString
        guard phase == .on else { return }
        stop()
        Task {
            for _ in 0..<40 where phase != .off { try? await Task.sleep(nanoseconds: 100_000_000) }
            start(profile)
        }
    }

    /// Stops protection and waits for it to be down (the scan must see the real network).
    func stopAndWait() async {
        guard phase != .off else { return }
        stop()
        for _ in 0..<50 where phase != .off { try? await Task.sleep(nanoseconds: 100_000_000) }
    }

    private func sync() {
        switch manager?.connection.status ?? .invalid {
        case .connected:
            phase = .on
            since = manager?.connection.connectedDate
        case .connecting, .reasserting: phase = .starting
        case .disconnecting: phase = .stopping
        default:
            phase = .off
            since = nil
        }
        if phase == .on && poll == nil {
            poll = Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in
                Task { @MainActor in self?.fetchLogs() }
            }
            fetchLogs()
        } else if phase == .off {
            poll?.invalidate()
            poll = nil
        }
    }

    private func fetchLogs() {
        guard let session = manager?.connection as? NETunnelProviderSession else { return }
        try? session.sendProviderMessage(Data("status".utf8)) { data in
            guard let data, let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return }
            let lines = obj["logs"] as? [String] ?? []
            Task { @MainActor in self.logs = lines }
        }
    }
}

struct Combo: Identifiable {
    let id = UUID()
    let profile: Profile
    let passed: Int
    let total: Int
    let avgMillis: Int
}

/// Auto Bypass, run by the Go core inside the app (not the tunnel, which has
/// a 50 MB memory limit).
@MainActor
final class Scanner: ObservableObject {
    @Published private(set) var running = false
    @Published private(set) var stage = ""
    @Published private(set) var done = 0
    @Published private(set) var total = 0
    @Published private(set) var message = ""
    @Published private(set) var combos: [Combo] = []
    @Published private(set) var best: Profile?
    @Published private(set) var blocked: [String] = []
    @Published private(set) var finished = false
    @Published var error: String?

    private let store: Store
    private var listener: Listener?

    init(store: Store) {
        self.store = store
        if let last = store.lastScan { apply(report: last) }
    }

    func start(tunnel: Tunnel) {
        guard !running else { return }
        running = true
        finished = false
        stage = "baseline"; done = 0; total = 0; message = ""; combos = []; best = nil; blocked = []; error = nil
        Task {
            await tunnel.stopAndWait()
            let targets = (try? JSONSerialization.data(withJSONObject: store.targets)).map { String(decoding: $0, as: UTF8.self) } ?? "[]"
            let l = Listener(owner: self)
            listener = l
            MobileScan(targets, l)
        }
    }

    func cancel() { MobileCancelScan() }

    fileprivate func progress(_ p: [String: Any]) {
        stage = p["stage"] as? String ?? stage
        done = (p["done"] as? NSNumber)?.intValue ?? done
        total = (p["total"] as? NSNumber)?.intValue ?? total
        if let c = (p["combo"] as? [String: Any]).flatMap(Self.combo) {
            combos.append(c)
            message = c.profile.label(L("dns.system"))
        } else {
            message = p["message"] as? String ?? ""
        }
    }

    fileprivate func finish(_ out: [String: Any]) {
        running = false
        listener = nil
        let err = (out["error"] as? String).flatMap { $0.isEmpty ? nil : $0 }
        if let report = out["report"] as? [String: Any] {
            store.lastScan = report
            if let b = report["best"] as? [String: Any] { store.autoProfile = Profile(b) }
            apply(report: report)
        }
        error = err
    }

    private func apply(report r: [String: Any]) {
        combos = (r["results"] as? [[String: Any]] ?? []).compactMap(Self.combo)
        best = (r["best"] as? [String: Any]).map(Profile.init)
        blocked = r["blocked"] as? [String] ?? []
        stage = "done"
        finished = true
    }

    private static func combo(_ c: [String: Any]) -> Combo? {
        guard let p = c["profile"] as? [String: Any] else { return nil }
        return Combo(profile: Profile(p), passed: (c["passed"] as? NSNumber)?.intValue ?? 0,
                     total: (c["total"] as? NSNumber)?.intValue ?? 0, avgMillis: (c["avgMillis"] as? NSNumber)?.intValue ?? 0)
    }

    private final class Listener: NSObject, MobileScanListenerProtocol {
        weak var owner: Scanner?
        init(owner: Scanner) { self.owner = owner }

        private static func obj(_ s: String?) -> [String: Any] {
            s.flatMap { $0.data(using: .utf8) }.flatMap { try? JSONSerialization.jsonObject(with: $0) as? [String: Any] } ?? [:]
        }

        func progress(_ json: String?) {
            let p = Self.obj(json)
            Task { @MainActor in self.owner?.progress(p) }
        }

        func done(_ json: String?) {
            let o = Self.obj(json)
            Task { @MainActor in self.owner?.finish(o) }
        }
    }
}
