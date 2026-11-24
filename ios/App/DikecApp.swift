import Dikeccore
import SwiftUI

// "Kesik" palette, shared with the desktop and Android apps.
extension Color {
    static let bg = Color(red: 0x0B / 255, green: 0x0C / 255, blue: 0x0F / 255)
    static let panel = Color(red: 0x14 / 255, green: 0x16 / 255, blue: 0x1B / 255)
    static let panel2 = Color(red: 0x1B / 255, green: 0x1E / 255, blue: 0x25 / 255)
    static let line = Color(red: 0x26 / 255, green: 0x2A / 255, blue: 0x33 / 255)
    static let textMain = Color(red: 0xF2 / 255, green: 0xF3 / 255, blue: 0xF5 / 255)
    static let muted = Color(red: 0x9A / 255, green: 0xA1 / 255, blue: 0xAE / 255)
    static let accent = Color(red: 0xC8 / 255, green: 0xF2 / 255, blue: 0x5A / 255)
    static let danger = Color(red: 0xFF / 255, green: 0x8A / 255, blue: 0x7A / 255)
}

@main
struct DikecApp: App {
    @StateObject private var tunnel = Tunnel()
    @StateObject private var scanner: Scanner
    private let store: Store

    init() {
        let s = Store()
        store = s
        _scanner = StateObject(wrappedValue: Scanner(store: s))
        let tab = UITabBarAppearance()
        tab.configureWithOpaqueBackground()
        tab.backgroundColor = UIColor(Color.panel)
        UITabBar.appearance().standardAppearance = tab
        UITabBar.appearance().scrollEdgeAppearance = tab
    }

    var body: some Scene {
        WindowGroup {
            RootView(store: store)
                .environmentObject(tunnel)
                .environmentObject(scanner)
                .preferredColorScheme(.dark)
                .tint(.accent)
        }
    }
}

struct RootView: View {
    let store: Store
    @State private var tab = 0
    @State private var refresh = 0 // bump to re-read the store

    var body: some View {
        TabView(selection: $tab) {
            PanelView(store: store, refresh: refresh, goAuto: { tab = 1 })
                .tabItem { Label(L("nav.panel"), systemImage: "power") }.tag(0)
            AutoView(store: store, onApplied: { refresh += 1; tab = 0 })
                .tabItem { Label(L("nav.auto"), systemImage: "wand.and.stars") }.tag(1)
            ProfilesView(store: store, refresh: $refresh)
                .tabItem { Label(L("nav.profiles"), systemImage: "slider.horizontal.3") }.tag(2)
            AboutView()
                .tabItem { Label(L("st.about"), systemImage: "info.circle") }.tag(3)
        }
    }
}

struct Card<Content: View>: View {
    @ViewBuilder var content: Content
    var body: some View {
        VStack(alignment: .leading, spacing: 8) { content }
            .padding(16)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Color.panel, in: RoundedRectangle(cornerRadius: 16))
            .overlay(RoundedRectangle(cornerRadius: 16).stroke(Color.line))
    }
}

struct Screen<Content: View>: View {
    @ViewBuilder var content: Content
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) { content }
                .padding(16)
        }
        .background(Color.bg.ignoresSafeArea())
        .foregroundStyle(Color.textMain)
    }
}

func elapsed(_ since: Date?, now: Date) -> String {
    guard let since else { return "" }
    let s = max(0, Int(now.timeIntervalSince(since)))
    if s >= 3600 { return "\(s / 3600) \(L("u.h")) \(s % 3600 / 60) \(L("u.m"))" }
    if s >= 60 { return "\(s / 60) \(L("u.m")) \(s % 60) \(L("u.s"))" }
    return "\(s) \(L("u.s"))"
}

// MARK: Dashboard

struct PanelView: View {
    let store: Store
    let refresh: Int
    let goAuto: () -> Void
    @EnvironmentObject var tunnel: Tunnel
    @EnvironmentObject var scanner: Scanner

    var body: some View {
        let profile = store.selected()
        Screen {
            HStack {
                Text("Dikec DPI").font(.system(size: 22, weight: .bold))
                Spacer()
                TimelineView(.periodic(from: .now, by: 1)) { ctx in
                    Text(status(now: ctx.date)).font(.system(size: 13, design: .monospaced))
                        .foregroundStyle(tunnel.phase == .on ? Color.accent : Color.muted)
                }
            }
            VStack(spacing: 18) {
                PowerButton(phase: tunnel.phase) {
                    switch tunnel.phase {
                    case .off: tunnel.start(store.selected())
                    case .on, .starting: tunnel.stop()
                    case .stopping: break
                    }
                }
                Text(title).font(.system(size: 24, weight: .bold)).multilineTextAlignment(.center)
                Text(profile.label(L("dns.system")))
                    .font(.system(size: 14, design: .monospaced)).foregroundStyle(Color.muted)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 18)
            .id(refresh)

            if let e = tunnel.error { Text(e).foregroundStyle(Color.danger).font(.footnote) }

            Card {
                Text(L("stat.profile")).font(.caption).foregroundStyle(Color.muted)
                Text(profile.name).font(.headline)
                Text(profile.description).font(.footnote).foregroundStyle(Color.muted)
            }
            Card {
                Text(L("stat.unblocked")).font(.caption).foregroundStyle(Color.muted)
                if scanner.finished, !scanner.blocked.isEmpty {
                    Text("\(scanner.best != nil ? scanner.blocked.count : 0)/\(scanner.blocked.count)").font(.title2.bold())
                } else {
                    Button(L("stat.noScan"), action: goAuto).font(.footnote)
                }
            }
            if tunnel.phase == .on || !tunnel.logs.isEmpty {
                Card {
                    Text(L("log.title")).font(.caption).foregroundStyle(Color.muted)
                    if tunnel.logs.isEmpty {
                        Text(L("log.empty")).font(.footnote).foregroundStyle(Color.muted)
                    }
                    ForEach(Array(tunnel.logs.suffix(40).reversed().enumerated()), id: \.offset) { _, line in
                        Text(line).font(.system(size: 11, design: .monospaced)).foregroundStyle(Color.muted)
                            .lineLimit(2)
                    }
                }
            }
            Text(L("ios.vpnNote")).font(.caption).foregroundStyle(Color.muted)
        }
    }

    private var title: String {
        switch tunnel.phase {
        case .on: return L("hero.onTitle")
        case .starting: return L("busy.starting")
        case .stopping: return L("busy.stopping")
        case .off: return L("hero.offTitle")
        }
    }

    private func status(now: Date) -> String {
        switch tunnel.phase {
        case .on: return L("status.active", elapsed(tunnel.since, now: now))
        case .starting: return L("busy.startTag")
        case .stopping: return L("busy.stopTag")
        case .off: return L("status.off")
        }
    }
}

/// The power button: a ring that "cuts" open while connecting, like the desktop app.
struct PowerButton: View {
    let phase: Phase
    let action: () -> Void
    @State private var spin = false

    var body: some View {
        let busy = phase == .starting || phase == .stopping
        Button(action: action) {
            ZStack {
                Circle().fill(phase == .on ? Color.accent.opacity(0.14) : Color.panel2)
                Circle().stroke(Color.line, lineWidth: 2)
                Circle()
                    .trim(from: 0, to: busy ? 0.28 : (phase == .on ? 1 : 0))
                    .stroke(Color.accent, style: StrokeStyle(lineWidth: 4, lineCap: .round))
                    .rotationEffect(.degrees(spin ? 270 : -90))
                    .animation(busy ? .linear(duration: 0.9).repeatForever(autoreverses: false) : .easeOut(duration: 0.4), value: spin)
                    .animation(.easeInOut(duration: 0.4), value: phase)
                Image(systemName: "power")
                    .font(.system(size: 54, weight: .semibold))
                    .foregroundStyle(phase == .on ? Color.accent : Color.muted)
            }
            .frame(width: 170, height: 170)
        }
        .buttonStyle(.plain)
        .accessibilityLabel(phase == .on ? L("power.off") : L("power.on"))
        .disabled(phase == .stopping)
        .onChange(of: busy) { b in spin = b }
        .onAppear { spin = busy }
    }
}

// MARK: Auto Bypass

struct AutoView: View {
    let store: Store
    let onApplied: () -> Void
    @EnvironmentObject var tunnel: Tunnel
    @EnvironmentObject var scanner: Scanner

    var body: some View {
        Screen {
            if scanner.running {
                Text(L("auto.scanningTag")).font(.caption).foregroundStyle(Color.muted)
                Text(L("auto.scanningTitle")).font(.title2.bold())
                ProgressView(value: Double(scanner.done), total: Double(max(scanner.total, 1)))
                Text(scanner.message).font(.system(size: 12, design: .monospaced)).foregroundStyle(Color.muted)
                Button(L("btn.cancel")) { scanner.cancel() }.buttonStyle(.bordered)
                comboList
            } else if scanner.finished {
                Text(scanner.best != nil ? L("auto.found") : L("auto.notFound")).font(.title2.bold())
                if let e = scanner.error { Text(e).foregroundStyle(Color.danger).font(.footnote) }
                if !scanner.blocked.isEmpty {
                    Text(L("tg.blocked", scanner.blocked.joined(separator: ", "))).font(.footnote).foregroundStyle(Color.muted)
                }
                if let best = scanner.best {
                    Card {
                        Text(L("auto.winner")).font(.caption).foregroundStyle(Color.muted)
                        Text(best.label(L("dns.system"))).font(.system(.headline, design: .monospaced)).foregroundStyle(Color.accent)
                        Button(L("btn.apply")) {
                            store.autoProfile = best
                            store.selectedID = best.id
                            tunnel.phase == .off ? tunnel.start(best) : tunnel.switchTo(best)
                            onApplied()
                        }
                        .buttonStyle(.borderedProminent).foregroundStyle(Color.bg)
                    }
                }
                Button(L("btn.rescan")) { scanner.start(tunnel: tunnel) }.buttonStyle(.bordered)
                comboList
            } else {
                Text(L("auto.introTag")).font(.caption).foregroundStyle(Color.muted)
                Text(L("auto.introTitle")).font(.title2.bold())
                Text(L("auto.introBody")).font(.callout).foregroundStyle(Color.muted)
                Button(L("btn.startScan")) { scanner.start(tunnel: tunnel) }.buttonStyle(.borderedProminent).foregroundStyle(Color.bg)
                Text(L("auto.duration")).font(.caption).foregroundStyle(Color.muted)
            }
        }
    }

    private var comboList: some View {
        VStack(spacing: 6) {
            ForEach(scanner.combos.sorted { ($0.passed, -$0.avgMillis) > ($1.passed, -$1.avgMillis) }) { c in
                HStack {
                    Text(c.profile.label(L("dns.system"))).font(.system(size: 12, design: .monospaced))
                    Spacer()
                    Text("\(c.passed)/\(c.total)").font(.system(size: 12, design: .monospaced))
                        .foregroundStyle(c.total > 0 && c.passed == c.total ? Color.accent : Color.muted)
                    Text(c.avgMillis > 0 ? "\(c.avgMillis) ms" : "–").font(.system(size: 12, design: .monospaced))
                        .foregroundStyle(Color.muted).frame(width: 64, alignment: .trailing)
                }
                .padding(.vertical, 6).padding(.horizontal, 10)
                .background(Color.panel, in: RoundedRectangle(cornerRadius: 8))
            }
        }
    }
}

// MARK: Profiles

struct ProfilesView: View {
    let store: Store
    @Binding var refresh: Int
    @EnvironmentObject var tunnel: Tunnel
    @State private var region = ""

    var body: some View {
        let all = store.all()
        let regions = [""] + Array(Set(all.map(\.region).filter { !$0.isEmpty })).sorted()
        let selected = store.selectedID
        Screen {
            Text(L("pf.title")).font(.title3.bold())
            ScrollView(.horizontal, showsIndicators: false) {
                HStack {
                    ForEach(regions, id: \.self) { r in
                        Button(r.isEmpty ? L("pf.all") : r) { region = r }
                            .font(.footnote.weight(.medium))
                            .padding(.horizontal, 12).padding(.vertical, 6)
                            .background(region == r ? Color.accent : Color.panel2, in: Capsule())
                            .foregroundStyle(region == r ? Color.bg : Color.textMain)
                    }
                }
            }
            ForEach(all.filter { region.isEmpty || $0.region == region }) { p in
                Button {
                    store.selectedID = p.id
                    refresh += 1
                    tunnel.switchTo(p)
                } label: {
                    VStack(alignment: .leading, spacing: 4) {
                        HStack {
                            Text(p.name).font(.headline)
                            if p.verified { Text(L("badge.verified")).font(.caption2).foregroundStyle(Color.accent) }
                            Spacer()
                            if p.id == selected { Image(systemName: "checkmark.circle.fill").foregroundStyle(Color.accent) }
                        }
                        Text(p.label(L("dns.system"))).font(.system(size: 12, design: .monospaced)).foregroundStyle(Color.muted)
                        if !p.description.isEmpty { Text(p.description).font(.footnote).foregroundStyle(Color.muted) }
                    }
                    .padding(14)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Color.panel, in: RoundedRectangle(cornerRadius: 14))
                    .overlay(RoundedRectangle(cornerRadius: 14).stroke(p.id == selected ? Color.accent : Color.line))
                }
                .buttonStyle(.plain)
            }
            .id(refresh)
        }
    }
}

// MARK: About

struct AboutView: View {
    var body: some View {
        Screen {
            Text("Dikec DPI").font(.title.bold())
            Text("v\(MobileVersion())").font(.system(size: 13, design: .monospaced)).foregroundStyle(Color.muted)
            Text(L("ios.vpnNote")).font(.callout).foregroundStyle(Color.muted)
            Text(L("ios.language")).font(.callout).foregroundStyle(Color.muted)
            Text(L("st.credits", "xvzc/SpoofDPI", "mertikavi/SpoofDPI-Turkiye")).font(.footnote).foregroundStyle(Color.muted)
            Link("github.com/dikeckaan/DikecDPI", destination: URL(string: "https://github.com/dikeckaan/DikecDPI")!)
                .font(.footnote)
        }
    }
}
