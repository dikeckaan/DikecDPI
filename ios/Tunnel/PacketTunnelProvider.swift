import Dikeccore
import NetworkExtension
import os

/// Runs the Dikec DPI Go core on the tunnel's utun interface. Every packet is
/// handled on the phone: TCP streams are reopened locally and only the first
/// TLS packet is reshaped; DNS is answered through the profile's resolver.
final class PacketTunnelProvider: NEPacketTunnelProvider {
    private let log = Logger(subsystem: "com.dikec.dpi", category: "tunnel")
    private let logger = TunnelLogger()

    override func startTunnel(options: [String: NSObject]?, completionHandler: @escaping (Error?) -> Void) {
        let json = (options?["profile"] as? String) ?? Shared.activeProfileJSON
            ?? (parseJSONArray(MobilePresets()).first.flatMap { try? JSONSerialization.data(withJSONObject: $0) })
                .map { String(decoding: $0, as: UTF8.self) }
            ?? "{}"
        Shared.activeProfileJSON = json
        logger.reset()

        let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "127.0.0.1")
        settings.mtu = 1500
        let v4 = NEIPv4Settings(addresses: ["10.111.222.1"], subnetMasks: ["255.255.255.255"])
        v4.includedRoutes = [NEIPv4Route.default()]
        settings.ipv4Settings = v4
        let v6 = NEIPv6Settings(addresses: ["fd66:6466:7069::1"], networkPrefixLengths: [128])
        v6.includedRoutes = [NEIPv6Route.default()]
        settings.ipv6Settings = v6
        // Queries go to this address through the tunnel; the core answers them
        // with the profile's resolver (Yandex 1253, DoH, ...).
        let dns = NEDNSSettings(servers: ["1.1.1.1"])
        dns.matchDomains = [""]
        settings.dnsSettings = dns

        setTunnelNetworkSettings(settings) { [weak self] error in
            guard let self else { return }
            if let error {
                self.log.error("settings: \(error.localizedDescription, privacy: .public)")
                completionHandler(error)
                return
            }
            let fd = dikec_find_utun_fd()
            guard fd >= 0 else {
                completionHandler(NSError(domain: "DikecDPI", code: 1,
                    userInfo: [NSLocalizedDescriptionKey: "utun file descriptor not found"]))
                return
            }
            var err: NSError?
            if !MobileStart(Int(fd), 1500, json, self.logger, &err) {
                self.log.error("start: \(err?.localizedDescription ?? "?", privacy: .public)")
                completionHandler(err ?? NSError(domain: "DikecDPI", code: 2))
                return
            }
            self.log.info("started fd=\(fd)")
            completionHandler(nil)
        }
    }

    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        MobileStop()
        completionHandler()
    }

    /// The app asks for recent log lines and the running state.
    override func handleAppMessage(_ messageData: Data, completionHandler: ((Data?) -> Void)?) {
        let reply: [String: Any] = [
            "running": MobileIsRunning(),
            "since": MobileRunningSince(),
            "profile": MobileCurrentProfile(),
            "logs": logger.lines(),
        ]
        completionHandler?(try? JSONSerialization.data(withJSONObject: reply))
    }

    override func sleep(completionHandler: @escaping () -> Void) { completionHandler() }
}

/// Keeps the last log lines in memory for the app (the extension has a tight
/// memory limit, so this is a small ring).
final class TunnelLogger: NSObject, MobileLoggerProtocol {
    private let lock = NSLock()
    private var buf: [String] = []
    private let fmt: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "HH:mm:ss"
        return f
    }()

    func log(_ level: String?, message: String?) {
        let line = "\(fmt.string(from: Date())) \(level ?? "") \(message ?? "")"
        lock.lock()
        buf.append(line)
        if buf.count > 300 { buf.removeFirst(buf.count - 300) }
        lock.unlock()
    }

    func reset() { lock.lock(); buf.removeAll(); lock.unlock() }

    func lines() -> [String] { lock.lock(); defer { lock.unlock() }; return buf }
}
