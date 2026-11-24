import Foundation

/// Values shared by the app and the packet tunnel extension.
enum Shared {
    static let appGroup = "group.com.dikec.dpi"
    static let tunnelBundleID = "com.dikec.dpi.tunnel"
    static let defaults = UserDefaults(suiteName: appGroup) ?? .standard

    /// Profile JSON the tunnel uses when iOS starts it without options
    /// (on-demand, Settings app, reboot).
    static var activeProfileJSON: String? {
        get { defaults.string(forKey: "activeProfile") }
        set { defaults.set(newValue, forKey: "activeProfile") }
    }

    static var logFile: URL? {
        FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: appGroup)?
            .appendingPathComponent("tunnel.log")
    }
}

/// Localized string from Localizable.strings (generated from i18n.js).
func L(_ key: String, _ args: CVarArg...) -> String {
    let format = NSLocalizedString(key, comment: "")
    return args.isEmpty ? format : String(format: format, arguments: args)
}

/// One bypass setting, in the same JSON shape as the Go engine.Profile.
struct Profile: Identifiable, Equatable {
    let json: [String: Any]

    init(_ json: [String: Any]) { self.json = json }

    init?(string: String?) {
        guard let s = string, let data = s.data(using: .utf8),
              let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return nil }
        self.json = obj
    }

    var id: String { json["id"] as? String ?? "" }
    var name: String { json["name"] as? String ?? id }
    var region: String { json["region"] as? String ?? "" }
    var description: String { json["description"] as? String ?? "" }
    var verified: Bool { json["verified"] as? Bool ?? false }
    var fakeCount: Int { (json["fakeCount"] as? NSNumber)?.intValue ?? 0 }
    /// Fake packets need raw sockets, which iOS apps do not have.
    var needsRoot: Bool { fakeCount > 0 }

    var jsonString: String {
        guard let data = try? JSONSerialization.data(withJSONObject: json, options: [.sortedKeys]) else { return "{}" }
        return String(decoding: data, as: UTF8.self)
    }

    var strategy: String {
        var s = json["splitMode"] as? String ?? ""
        if s == "chunk" { s += "/\((json["chunkSize"] as? NSNumber)?.intValue ?? 0)" }
        if json["disorder"] as? Bool == true { s += "+disorder" }
        if fakeCount > 0 { s += "+fake×\(fakeCount)" }
        return s
    }

    func dns(_ systemName: String) -> String {
        let doh = json["dohUrl"] as? String ?? ""
        let addr = json["dnsAddr"] as? String ?? ""
        switch json["dnsMode"] as? String ?? "" {
        case "https":
            if doh.contains("1.1.1.1") || doh.contains("cloudflare") { return "CF DoH" }
            if doh.contains("8.8.8.8") || doh.contains("dns.google") { return "Google DoH" }
            if doh.contains("9.9.9.9") || doh.contains("quad9") { return "Quad9 DoH" }
            return "DoH"
        case "system":
            return systemName
        default:
            if addr.hasPrefix("77.88.8.8") { return "Yandex" }
            if addr.hasPrefix("1.1.1.1") { return "CF UDP" }
            return "UDP \(addr)"
        }
    }

    func label(_ systemName: String) -> String { "\(strategy) · \(dns(systemName))" }

    static func == (a: Profile, b: Profile) -> Bool { a.jsonString == b.jsonString }
}

func parseJSONArray(_ s: String?) -> [Any] {
    guard let s, let data = s.data(using: .utf8),
          let arr = try? JSONSerialization.jsonObject(with: data) as? [Any] else { return [] }
    return arr
}
