import { Call, Events } from "/wails/runtime.js";

// ---------------------------------------------------------------- utils
const isTray = new URLSearchParams(location.search).get("view") === "tray";
const root = document.getElementById("app");
const toastEl = document.getElementById("toast");
if (isTray) document.body.classList.add("tray");

const esc = (v) => String(v ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

async function api(method, ...args) {
  try {
    return await Call.ByName("main.App." + method, ...args);
  } catch (err) {
    toast(err?.message || String(err));
    throw err;
  }
}

let toastTimer;
function toast(msg, ok = false) {
  toastEl.textContent = msg;
  toastEl.className = "toast" + (ok ? " ok" : "");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toastEl.classList.add("hidden"), 4200);
}

function stratLabel(p) {
  let s = p.splitMode === "chunk" ? `chunk/${p.chunkSize}` : p.splitMode;
  if (p.disorder) s += "+disorder";
  if (p.fakeCount > 0) s += `+fake×${p.fakeCount}`;
  return s;
}
function dnsLabel(p) {
  if (p.dnsMode === "https") {
    const u = p.dohUrl || "";
    if (u.includes("1.1.1.1") || u.includes("cloudflare")) return "CF DoH";
    if (u.includes("8.8.8.8") || u.includes("dns.google")) return "Google DoH";
    if (u.includes("9.9.9.9") || u.includes("quad9")) return "Quad9 DoH";
    return "DoH";
  }
  if (p.dnsMode === "system") return "Sistem";
  if ((p.dnsAddr || "").startsWith("77.88.8.8")) return "Yandex";
  if ((p.dnsAddr || "").startsWith("1.1.1.1")) return "CF UDP";
  return "UDP " + (p.dnsAddr || "");
}
const profileLabel = (p) => `${stratLabel(p)} · ${dnsLabel(p)}`;

function since(t) {
  const s = Math.max(0, Math.floor((Date.now() - new Date(t).getTime()) / 1000));
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60);
  if (h) return `${h} sa ${m} dk`;
  if (m) return `${m} dk`;
  return `${s} sn`;
}

// ---------------------------------------------------------------- icons
const I = {
  panel: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><rect x="3" y="3" width="7" height="9" rx="1.5"/><rect x="14" y="3" width="7" height="5" rx="1.5"/><rect x="14" y="12" width="7" height="9" rx="1.5"/><rect x="3" y="16" width="7" height="5" rx="1.5"/></svg>',
  bolt: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M13 2 4 14h7l-1 8 9-12h-7z"/></svg>',
  globe: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c2.5 2.7 3.8 5.7 3.8 9s-1.3 6.3-3.8 9c-2.5-2.7-3.8-5.7-3.8-9S9.5 5.7 12 3z"/></svg>',
  list: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M4 6h16M4 12h10M4 18h13"/></svg>',
  gear: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M2 12h3M19 12h3M4.9 19.1 7 17M17 7l2.1-2.1"/></svg>',
};

// The "Kesik" mark. state: on | off | warn
function glyph(state, color, size) {
  const x = state === "off" ? [4.5, 4.5, 4.5] : [5, 8, 2];
  const op = state === "off" ? 0.55 : 1;
  const mid = state === "warn"
    ? `<rect x="8" y="9.5" width="6" height="5" rx="1.2" fill="${color}"/><rect x="17" y="9.5" width="6" height="5" rx="1.2" fill="${color}"/>`
    : `<rect x="${x[1]}" y="9.5" width="15" height="5" rx="1.2" fill="${color}" opacity="${op}"/>`;
  return `<svg class="glyph" width="${size}" height="${size}" viewBox="0 0 24 24" aria-hidden="true">
    <rect x="${x[0]}" y="3" width="15" height="5" rx="1.2" fill="${color}" opacity="${op}"/>${mid}
    <rect x="${x[2]}" y="16" width="15" height="5" rx="1.2" fill="${color}" opacity="${op}"/></svg>`;
}

// ---------------------------------------------------------------- state
const ui = {
  s: null,
  page: "panel",
  busy: false,
  region: "Tümü",
  hosts: [],
  logs: [],
  logSeq: 0,
  scan: null, // { stage, done, total, message, cells: Map, rows: [], cols: [] }
};

function freshScan() {
  return { stage: "baseline", done: 0, total: 0, message: "Başlatılıyor…", cells: new Map(), rows: [], cols: [] };
}

function addCombo(scan, combo) {
  const r = stratLabel(combo.profile), c = dnsLabel(combo.profile);
  if (!scan.rows.includes(r)) scan.rows.push(r);
  if (!scan.cols.includes(c)) scan.cols.push(c);
  scan.cells.set(r + "|" + c, combo);
}

function scanFromSummary(sum) {
  const scan = freshScan();
  scan.stage = "done";
  (sum.results || []).forEach((c) => addCombo(scan, c)); // already ranked
  return scan;
}

// ---------------------------------------------------------------- views: main
function render() {
  if (!ui.s) return;
  root.innerHTML = isTray ? trayView() : mainView();
  if (!isTray && ui.page === "logs") {
    const box = document.getElementById("logbox");
    if (box) box.scrollTop = box.scrollHeight;
  }
}

function mainView() {
  const s = ui.s;
  const nav = [
    ["panel", "Panel", I.panel],
    ["auto", "Otomatik Bypass", I.bolt],
    ["profiles", "Profiller", I.globe],
    ["logs", "Kayıt", I.list],
    ["settings", "Ayarlar", I.gear],
  ].map(([id, label, icon]) =>
    `<button data-act="page" data-arg="${id}" ${ui.page === id ? 'aria-current="page"' : ""}>${icon}${label}</button>`).join("");

  const page = { panel: panelPage, auto: autoPage, profiles: profilesPage, logs: logsPage, settings: settingsPage }[ui.page]();
  return `<div class="shell">
    <div class="titlebar">Dikec DPI</div>
    <div class="body">
      <nav class="nav" aria-label="Ana menü">
        <div class="brand">${glyph(s.status.running ? "on" : "off", "var(--accent)", 26)}<b>Dikec<span>DPI</span></b></div>
        ${nav}
        <div class="grow"></div>
        <div class="netcard">
          <span class="eyebrow">Proxy</span>
          <span style="font-size:13px;font-weight:500">${esc(s.options.listenAddr)}</span>
          <span class="mono muted" style="font-size:11px">${s.status.systemProxy ? "Sistem proxy’si ayarlı" : (s.status.running ? "Tarayıcıyı elle yönlendirin" : "Kapalı")}</span>
        </div>
      </nav>
      <main class="main">${page}</main>
    </div>
  </div>`;
}

function panelPage() {
  const s = ui.s, st = s.status, on = st.running;
  const accent = "var(--accent)";
  const ringDash = on ? "46 9.3" : "553 0";
  const ringColor = on ? accent : "#3A3F4B";
  const best = s.lastScan?.results?.[0];

  let banner = "";
  if (s.error) {
    banner = `<div class="banner err">${glyph("warn", "#FF8A7A", 22)}<p>${esc(s.error)}</p><button class="btn sm" data-act="page" data-arg="settings">Ayarlar</button></div>`;
  } else if (s.firstRun && !s.scanning) {
    banner = `<div class="banner">${glyph("on", accent, 22)}<p><b>İlk kez mi?</b> Otomatik Bypass ağınızda ~10 saniyede onlarca kombinasyonu dener ve en iyisini seçer.</p>
      <button class="btn sm primary" data-act="scan">Şimdi tara</button><button class="btn sm" data-act="dismiss">Geç</button></div>`;
  } else if (s.scanning) {
    banner = `<div class="banner">${glyph("on", accent, 22)}<p>Otomatik Bypass çalışıyor…</p><button class="btn sm" data-act="page" data-arg="auto">İzle</button></div>`;
  }

  const hosts = ui.hosts.length
    ? (() => {
        const max = Math.max(...ui.hosts.map((h) => h.requests));
        return ui.hosts.map((h) => {
          const n = Math.max(1, Math.round((h.requests / max) * 12));
          const segs = Array.from({ length: n }, (_, i) => `<span style="flex:1;background:${i % 2 ? "#3A3F4B" : accent}"></span>`).join("");
          return `<div class="host"><span class="h" title="${esc(h.host)}">${esc(h.host)}</span><span class="bar" aria-hidden="true">${segs}</span><span class="n">${h.requests} istek</span></div>`;
        }).join("");
      })()
    : `<div class="empty">${on ? "Henüz trafik yok. Tarayıcıda bir site açtığınızda burada görünecek." : "Koruma kapalı."}</div>`;

  return `${banner}
  <div class="hero">
    <button class="power ${ui.busy ? "busy" : ""}" data-act="toggle" aria-pressed="${on}" aria-label="${on ? "Korumayı kapat" : "Korumayı aç"}">
      <svg class="ring" width="196" height="196" viewBox="0 0 196 196" aria-hidden="true">
        <circle cx="98" cy="98" r="88" fill="none" stroke="#1E2129" stroke-width="10"/>
        <circle class="ring-fg" cx="98" cy="98" r="88" fill="none" stroke="${ringColor}" stroke-width="10" stroke-dasharray="${ringDash}" transform="rotate(-90 98 98)"/>
        <circle cx="98" cy="98" r="70" fill="#14161B" stroke="#262A33"/>
      </svg>
      <span style="position:relative">${glyph(on ? "on" : "off", on ? accent : "#6E7582", 64)}</span>
    </button>
    <div class="hero-text">
      <span class="tag" style="color:${on ? accent : "var(--muted)"}">${on ? "● Aktif · " + since(st.since) : "○ Kapalı"}</span>
      <h1>${on ? "Bağlantı serbest." : "Koruma kapalı."}</h1>
      <p>${on
        ? `Proxy ${esc(st.addr)} adresinde. Engelli siteler parçalanmış ClientHello ile açılıyor; diğer trafik olduğu gibi geçiyor.`
        : "Açtığınızda sistem proxy’si ayarlanır, kapattığınızda eski ayarlarınız geri yüklenir."}</p>
      <div class="row" style="margin-top:6px">
        <button class="btn ${on ? "dark" : "primary"}" data-act="toggle">${on ? "Durdur" : "Başlat"}</button>
        <button class="btn" data-act="scan" ${s.scanning ? "disabled" : ""}>${I.bolt}${s.lastScan ? "Yeniden tara" : "Otomatik tara"}</button>
      </div>
    </div>
  </div>
  <div class="grid3">
    <div class="card stat"><span class="k">Aktif profil</span><span class="v">${esc(s.selected.name)}</span><span class="s" style="color:var(--accent)">${esc(profileLabel(s.selected))}</span></div>
    <div class="card stat"><span class="k">Engelli site açıldı</span><span class="v">${best ? `${best.passed} / ${best.total}` : "—"}</span><span class="s">${best ? "son taramada · " + esc(s.lastScan.duration) : "henüz tarama yok"}</span></div>
    <div class="card stat"><span class="k">${on ? "Bu oturumda istek" : "Ortalama gecikme"}</span><span class="v">${on ? st.requests : best ? best.avgMillis + " ms" : "—"}</span><span class="s">${on ? (s.unblockedNow ? s.unblockedNow + " engelli site kullanıldı" : "tüm siteler") : "TLS el sıkışma dahil"}</span></div>
  </div>
  <section class="card" aria-label="Canlı bağlantılar" style="display:flex;flex-direction:column;gap:10px">
    <div class="between"><span style="font-size:13px;font-weight:600">Canlı bağlantılar</span>${ui.hosts.length ? '<button class="btn sm" data-act="resetstats">Sıfırla</button>' : ""}</div>
    <div class="hosts">${hosts}</div>
  </section>`;
}

const STAGES = [["baseline", "Bağlantı"], ["dns", "DNS"], ["strategy", "Strateji"], ["dns-mix", "Çapraz"]];

function autoPage() {
  const s = ui.s;
  const scan = ui.scan || (s.lastScan ? scanFromSummary(s.lastScan) : null);
  const curIdx = scan ? (scan.stage === "done" ? 99 : STAGES.findIndex(([k]) => k === scan.stage)) : -1;
  const stages = STAGES.map(([k, l], i) => {
    const cls = i < curIdx ? "done" : i === curIdx ? "now" : "";
    return `<li class="${cls}"><i>${i < curIdx ? "✓" : i + 1}</i>${l}</li>`;
  }).join("");

  let head, body;
  if (s.scanning && ui.scan) {
    const pct = ui.scan.total ? Math.round((ui.scan.done / ui.scan.total) * 100) : 4;
    head = `<span class="tag" style="color:var(--accent)">Otomatik Bypass · taranıyor</span><h1>Ağınız test ediliyor…</h1>`;
    body = `<div class="card" style="display:flex;flex-direction:column;gap:10px">
      <div class="between"><span class="mono" style="font-size:12px">${esc(ui.scan.message)}</span><button class="btn sm" data-act="cancelscan">İptal</button></div>
      <div class="progress"><span style="width:${pct}%"></span></div></div>
      ${matrixView(ui.scan, null)}`;
  } else if (scan && s.lastScan) {
    const sum = s.lastScan, best = sum.best;
    head = `<span class="tag" style="color:var(--accent)">Otomatik Bypass · tamamlandı · ${esc(sum.duration)}</span>
      <h1>${best ? "Bu ağ için en iyi yol bulundu." : "Çalışan bir kombinasyon bulunamadı."}</h1>`;
    const winner = best ? `<div class="winner">
        <span class="eyebrow" style="color:var(--bg)">Kazanan</span>
        <span class="big">${esc(stratLabel(best))}</span>
        <span class="mono" style="font-size:12px">${esc(dnsLabel(best))} · ${sum.results[0].passed}/${sum.results[0].total} · ${sum.results[0].avgMillis} ms</span>
        <button class="btn" data-act="applyauto">${s.selectedId === "auto" && s.status.running ? "Uygulandı ✓" : "Uygula ve başlat"}</button>
      </div>` : `<div class="card"><p class="muted">Hedef sitelerin hiçbiri açılamadı. Siteler IP düzeyinde engelli olabilir; bu durumda VPN gerekir.</p></div>`;
    const maxMs = Math.max(1, ...(sum.dns || []).filter((d) => d.ok).map((d) => d.avgMillis));
    const dns = (sum.dns || []).map((d) => `<div class="dnsrow"><span class="nm">${esc(d.name)}</span><span class="tr" aria-hidden="true"><span style="width:${d.ok ? Math.max(4, (d.avgMillis / maxMs) * 100) : 0}%"></span></span><span class="ms">${d.ok ? d.avgMillis + " ms" : "yok"}</span></div>`).join("");
    body = `<div class="scan-layout">
      <div class="card matrix-card" style="flex:1;min-width:0">${matrixView(scan, best)}</div>
      <aside class="aside">${winner}
        <div class="card" style="display:flex;flex-direction:column;gap:8px"><span style="font-size:12px;font-weight:600">DNS yanıt süresi</span>${dns}</div>
        ${targetsCard(sum)}
      </aside></div>`;
  } else {
    head = `<span class="tag muted">Otomatik Bypass</span><h1>Ağınıza en uygun ayarı bulalım.</h1>`;
    body = `<div class="card" style="display:flex;flex-direction:column;gap:14px;max-width:620px">
      <p class="muted">Dikec DPI önce internet bağlantınızı ve hangi sitelerin engelli olduğunu kontrol eder, ardından 6 DNS sunucusunu ve 17+ parçalama stratejisini gerçek bağlantılarla dener. Sistem ayarlarınıza dokunmaz.</p>
      <div class="row"><button class="btn primary" data-act="scan">${I.bolt}Taramayı başlat</button><span class="muted" style="font-size:12px">genellikle 10–20 saniye</span></div>
    </div>${targetsCard(null)}`;
  }
  return `<div class="between" style="align-items:flex-end"><div style="display:flex;flex-direction:column;gap:6px">${head}</div><ol class="stages" aria-label="Tarama aşamaları">${stages}</ol></div>${body}`;
}

function targetsCard(sum) {
  const blocked = sum ? `<span class="muted" style="font-size:12px;line-height:1.5">${sum.blocked?.length ? `Doğrudan bağlantıda engelli: ${esc(sum.blocked.join(", "))}.` : "Hedef sitelerin hiçbiri engelli görünmüyor."}</span>` : "";
  return `<div class="card" style="display:flex;flex-direction:column;gap:8px">
    <label for="targets" style="font-size:12px;font-weight:600">Test edilecek siteler</label>
    <textarea id="targets" class="targets" spellcheck="false">${esc(ui.s.targets.join("\n"))}</textarea>
    <div class="between">${blocked}<button class="btn sm" data-act="savetargets">Kaydet</button></div>
  </div>`;
}

function matrixView(scan, best) {
  if (!scan.rows.length) return `<div class="empty">Sonuçlar burada belirecek…</div>`;
  const cols = scan.cols;
  const bestKey = best ? stratLabel(best) : null;
  const head = `<div class="mrow" style="--cols:${cols.length}"><span class="hd" style="text-align:left;font-weight:600">Strateji</span>${cols.map((c) => `<span class="hd">${esc(c)}</span>`).join("")}</div>`;
  const rows = scan.rows.map((r) => {
    const cells = cols.map((c) => {
      const x = scan.cells.get(r + "|" + c);
      if (!x) return `<span class="cell"></span>`;
      const cls = x.passed === x.total ? "p3" : x.passed > 0 ? "p2" : "p0";
      const txt = x.passed === x.total ? `${x.passed}/${x.total} · ${x.avgMillis}` : x.passed > 0 ? `${x.passed}/${x.total}` : "×";
      const tip = (x.targets || []).map((t) => `${t.target}: ${t.ok ? "açık" : t.error || "engelli"}`).join("\n");
      return `<span class="cell ${cls}" title="${esc(tip)}">${txt}</span>`;
    }).join("");
    const isBest = r === bestKey;
    return `<div class="mrow" style="--cols:${cols.length}"><span class="name ${isBest ? "best" : ""}">${isBest ? "★ " : ""}${esc(r)}</span>${cells}</div>`;
  }).join("");
  return `<div class="matrix">${head}${rows}
    <div class="legend"><span><b style="background:var(--accent)"></b>tümü açıldı</span><span><b style="background:var(--accent-mid)"></b>kısmi</span><span><b style="background:var(--line);border:1px solid var(--line3)"></b>engellendi</span><span>boş = denenmedi</span></div></div>`;
}

function profilesPage() {
  const s = ui.s;
  const regions = ["Tümü", ...new Set(s.profiles.map((p) => p.region))];
  const tabs = regions.map((r) => `<button role="tab" aria-selected="${r === ui.region}" data-act="region" data-arg="${esc(r)}">${esc(r)}</button>`).join("");
  const cards = s.profiles.filter((p) => ui.region === "Tümü" || p.region === ui.region).map((p) => {
    const b = p.needsRoot ? ["r", "root"] : p.verified ? ["v", "doğrulandı"] : ["s", "başlangıç"];
    return `<button class="pcard" data-act="select" data-arg="${esc(p.id)}" aria-pressed="${p.id === s.selectedId}">
      <span class="between"><span class="muted" style="font-size:11px">${esc(p.region)}</span><span class="badge ${b[0]}">${b[1]}</span></span>
      <span class="nm">${esc(p.name)}</span>
      <span class="lb">${esc(profileLabel(p))}</span>
      <span class="ds">${esc(p.description)}</span>
    </button>`;
  }).join("");
  return `<div class="between" style="align-items:flex-end">
      <div style="display:flex;flex-direction:column;gap:6px"><span class="tag muted">Profiller · ${s.profiles.length} ayar</span><h1>Ülkene göre başla, ağına göre ince ayar yap.</h1></div>
      <button class="btn primary" data-act="scan" ${s.scanning ? "disabled" : ""}>${I.bolt}Benim ağımda test et</button>
    </div>
    <div class="tabs" role="tablist" aria-label="Bölge">${tabs}</div>
    <div class="pgrid">${cards}</div>
    <div class="card row" style="flex-wrap:wrap;font-size:12px;color:var(--muted2)">
      <span class="badge v">doğrulandı</span>gerçek ağda test edildi
      <span class="badge s">başlangıç</span>bilinen DPI davranışına göre; Otomatik Bypass ile doğrulayın
      <span class="badge r">root</span>sahte paket, yönetici yetkisi ister
    </div>`;
}

function logsPage() {
  const rows = ui.logs.slice(-600).map((l) => {
    const t = new Date(l.time).toLocaleTimeString("tr-TR");
    const f = Object.entries(l.fields || {}).filter(([k]) => k !== "step").map(([k, v]) => `${k}=${v}`).join(" ");
    return `<div class="log"><span class="t">${t}</span><span class="l ${esc(l.level)}">${esc(l.level)}</span><span class="sc">[${esc(l.scope || "app")}]</span><span>${esc(l.msg)} <span class="f">${esc(f)}</span></span></div>`;
  }).join("");
  return `<div class="between"><div style="display:flex;flex-direction:column;gap:6px"><span class="tag muted">Kayıt</span><h1>Ne oluyor?</h1></div><span class="muted" style="font-size:12px">Bağlantı başına ayrıntılar Panel’deki istatistiklerde.</span></div>
    <div class="logs" id="logbox">${rows || '<div class="empty">Henüz kayıt yok. Korumayı başlatın.</div>'}</div>`;
}

function settingsPage() {
  const s = ui.s, o = s.options, p = s.selected;
  const sw = (id, on, label) => `<button class="switch" role="switch" aria-checked="${on}" aria-label="${esc(label)}" data-act="sw" data-arg="${id}"><span></span></button>`;
  const opt = (v, cur, label) => `<option value="${v}" ${v === cur ? "selected" : ""}>${label}</option>`;
  return `<div style="display:flex;flex-direction:column;gap:6px"><span class="tag muted">Ayarlar</span><h1>İnce ayar.</h1></div>

  <section class="card" style="display:flex;flex-direction:column;gap:14px">
    <h2>Genel</h2>
    <div class="form">
      <label class="k">Açılışta korumayı başlat</label><div>${sw("startOnLaunch", s.startOnLaunch, "Açılışta korumayı başlat")}</div>
      <label class="k">Menü çubuğunda sayaç</label><div>${sw("showTrayCount", s.showTrayCount, "Menü çubuğunda sayaç")}</div>
      <span class="hint">Bu oturumda açılan engelli site sayısını simgenin yanında gösterir.</span>
    </div>
  </section>

  <section class="card" style="display:flex;flex-direction:column;gap:14px">
    <h2>Bağlantı</h2>
    <div class="form">
      <label class="k" for="o-listen">Dinleme adresi</label><input type="text" id="o-listen" value="${esc(o.listenAddr)}" spellcheck="false">
      <label class="k" for="o-mode">Proxy türü</label><select id="o-mode">${opt("http", o.appMode || "http", "HTTP (önerilen)")}${opt("socks5", o.appMode, "SOCKS5 (deneysel)")}</select>
      <label class="k">Sistem proxy’sini ayarla</label><div>${sw("systemProxy", o.systemProxy, "Sistem proxy’sini ayarla")}</div>
      <span class="hint">macOS’ta tüm uygulamalar Dikec DPI’dan geçer. Kapalıysa tarayıcınızı elle ${esc(o.listenAddr)} adresine yönlendirin.</span>
      <label class="k">IPv6 adresleri</label><div>${sw("ipv6", o.ipv6, "IPv6 adresleri")}</div>
      <label class="k">DNS önbelleği</label><div>${sw("dnsCache", o.dnsCache, "DNS önbelleği")}</div>
    </div>
    <div><button class="btn primary" data-act="saveoptions">Kaydet</button></div>
  </section>

  <section class="card" style="display:flex;flex-direction:column;gap:14px">
    <div class="between"><h2>Özel profil</h2><span class="muted" style="font-size:12px">şu an: ${esc(p.name)} · ${esc(profileLabel(p))}</span></div>
    <div class="form">
      <label class="k" for="c-split">Parçalama</label><select id="c-split">${["tlsrec", "chunk", "sni", "first-byte", "random", "none"].map((m) => opt(m, p.splitMode, m)).join("")}</select>
      <label class="k" for="c-chunk">Parça boyutu (bayt)</label><input type="number" id="c-chunk" min="1" max="255" value="${p.chunkSize || 5}">
      <label class="k">Sıra karıştırma (disorder)</label><div>${sw("c-disorder", p.disorder, "Sıra karıştırma")}</div>
      <label class="k" for="c-dns">DNS</label><select id="c-dns">${opt("udp", p.dnsMode || "udp", "UDP")}${opt("https", p.dnsMode, "DNS over HTTPS")}${opt("system", p.dnsMode, "Sistem DNS")}</select>
      <label class="k" for="c-addr">UDP DNS adresi</label><input type="text" id="c-addr" value="${esc(p.dnsAddr || "77.88.8.8:1253")}" spellcheck="false">
      <label class="k" for="c-doh">DoH adresi</label><input type="text" id="c-doh" value="${esc(p.dohUrl || "https://1.1.1.1/dns-query")}" spellcheck="false">
    </div>
    <div><button class="btn" data-act="savecustom">Özel profil olarak kaydet ve seç</button></div>
  </section>

  <section class="card credits">
    <h2 style="margin-bottom:8px">Hakkında</h2>
    Dikec DPI ${esc(s.version)} · Apache 2.0<br>
    Çekirdek: <b>xvzc/SpoofDPI</b> · Türkiye uyarlaması: <b>renardev/SpoofDPI-Turkiye</b>. Emeği geçen herkese teşekkürler.<br>
    Çıkışta sistem proxy ayarlarınız otomatik olarak geri yüklenir.
  </section>`;
}

// ---------------------------------------------------------------- views: tray
function trayView() {
  const s = ui.s, on = s.status.running;
  const featured = s.profiles.filter((p) => p.id === "auto" || p.id === "custom" || p.region === "Türkiye").slice(0, 5);
  if (!featured.some((p) => p.id === s.selectedId)) featured.unshift(s.selected);
  const items = featured.map((p) => `<button class="pitem" data-act="select" data-arg="${esc(p.id)}" aria-pressed="${p.id === s.selectedId}">
      <span class="dot" aria-hidden="true"></span><span class="nm">${esc(p.name)}</span><span class="lb">${esc(stratLabel(p))}</span></button>`).join("");
  const scanLine = s.scanning
    ? `Taranıyor${ui.scan?.total ? ` · ${ui.scan.done}/${ui.scan.total}` : "…"}`
    : s.lastScan ? `Son tarama: ${s.lastScan.combos} kombinasyon · ${s.lastScan.duration}` : "Henüz tarama yapılmadı";
  return `<div class="pop">
    <div class="head">
      ${glyph(s.error && on ? "warn" : on ? "on" : "off", on ? "var(--accent)" : "#6E7582", 34)}
      <div style="flex:1;display:flex;flex-direction:column;gap:2px;min-width:0">
        <span class="ttl">${on ? "Bağlantı serbest" : "Koruma kapalı"}</span>
        <span class="mono" style="font-size:11px;color:${on ? "var(--accent)" : "var(--muted)"}">${on ? "● aktif · " + since(s.status.since) : "○ kapalı"}</span>
      </div>
      <button class="switch" role="switch" aria-checked="${on}" aria-label="Korumayı aç veya kapat" data-act="toggle"><span></span></button>
    </div>
    ${s.error ? `<div class="mono" style="font-size:11px;color:var(--danger);line-height:1.4">${esc(s.error)}</div>` : ""}
    <div class="sep"></div>
    <span class="eyebrow">Profil</span>
    <div class="plist">${items}</div>
    <div class="sep"></div>
    <div class="scan">
      ${I.bolt.replace('stroke="currentColor"', 'stroke="#C8F25A"')}
      <div style="flex:1;display:flex;flex-direction:column;gap:2px;min-width:0"><span style="font-size:13px;font-weight:500">Otomatik Bypass</span><span class="muted" style="font-size:11px">${esc(scanLine)}</span></div>
      <button class="btn sm primary" data-act="${s.scanning ? "openmain" : "scan"}">${s.scanning ? "İzle" : "Tara"}</button>
    </div>
    <div class="links">
      <button data-act="openmain">Dikec DPI’ı aç <kbd>⌘O</kbd></button>
      <button data-act="quit">Çıkış ve proxy’yi geri al <kbd>⌘Q</kbd></button>
    </div>
  </div>`;
}

// ---------------------------------------------------------------- actions
const pending = { options: null };

async function act(name, arg, el) {
  switch (name) {
    case "page":
      ui.page = arg;
      if (arg === "logs") await pollLogs();
      render();
      break;
    case "region":
      ui.region = arg;
      render();
      break;
    case "toggle":
      ui.busy = true; render();
      try { await api("Toggle"); } finally { ui.busy = false; await refresh(true); }
      break;
    case "scan":
      ui.scan = freshScan();
      await api("StartScan");
      if (!isTray) ui.page = "auto";
      await refresh(true);
      break;
    case "cancelscan":
      await api("CancelScan");
      break;
    case "applyauto":
      await api("ApplyAuto");
      toast("Otomatik profil uygulandı.", true);
      await refresh(true);
      break;
    case "dismiss":
      await api("DismissFirstRun");
      await refresh(true);
      break;
    case "select":
      await api("SelectProfile", arg);
      await refresh(true);
      break;
    case "resetstats":
      await api("ResetStats");
      ui.hosts = [];
      render();
      break;
    case "savetargets": {
      const list = document.getElementById("targets").value.split(/[\s,]+/);
      const clean = await api("SaveTargets", list);
      toast(`${clean.length} hedef site kaydedildi.`, true);
      await refresh(true);
      break;
    }
    case "sw": {
      const on = el.getAttribute("aria-checked") !== "true";
      el.setAttribute("aria-checked", String(on));
      if (arg === "startOnLaunch" || arg === "showTrayCount") {
        const s = ui.s;
        await api("SavePreferences", arg === "startOnLaunch" ? on : s.startOnLaunch, arg === "showTrayCount" ? on : s.showTrayCount);
        await refresh(true);
      }
      break;
    }
    case "saveoptions": {
      const sw = (id) => document.querySelector(`[data-arg="${id}"]`).getAttribute("aria-checked") === "true";
      await api("SaveOptions", {
        listenAddr: document.getElementById("o-listen").value.trim(),
        appMode: document.getElementById("o-mode").value,
        systemProxy: sw("systemProxy"),
        ipv6: sw("ipv6"),
        dnsCache: sw("dnsCache"),
      });
      toast("Ayarlar kaydedildi.", true);
      await refresh(true);
      break;
    }
    case "savecustom": {
      const disorder = document.querySelector('[data-arg="c-disorder"]').getAttribute("aria-checked") === "true";
      await api("SaveCustomProfile", {
        splitMode: document.getElementById("c-split").value,
        chunkSize: parseInt(document.getElementById("c-chunk").value, 10) || 5,
        disorder,
        dnsMode: document.getElementById("c-dns").value,
        dnsAddr: document.getElementById("c-addr").value.trim(),
        dohUrl: document.getElementById("c-doh").value.trim(),
      });
      toast("Özel profil kaydedildi ve seçildi.", true);
      await refresh(true);
      break;
    }
    case "openmain":
      await api("OpenMain");
      break;
    case "quit":
      await api("Quit");
      break;
  }
}

root.addEventListener("click", (e) => {
  const el = e.target.closest("[data-act]");
  if (!el || el.disabled) return;
  act(el.dataset.act, el.dataset.arg, el).catch(() => {});
});

document.addEventListener("keydown", (e) => {
  if (isTray && e.metaKey && e.key === "o") { e.preventDefault(); act("openmain"); }
  if (isTray && e.metaKey && e.key === "q") { e.preventDefault(); act("quit"); }
});

// ---------------------------------------------------------------- data flow
function editing() {
  const a = document.activeElement;
  return a && root.contains(a) && /^(INPUT|TEXTAREA|SELECT)$/.test(a.tagName);
}

async function refresh(force = false) {
  try {
    ui.s = await Call.ByName("main.App.GetState");
  } catch { return; }
  if (!isTray && ui.page === "panel" && ui.s.status.running) {
    try { ui.hosts = await Call.ByName("main.App.Hosts", 8); } catch {}
  } else if (!ui.s.status.running) {
    ui.hosts = [];
  }
  if (!ui.s.scanning && ui.scan && ui.scan.stage !== "done") ui.scan = null;
  if (force || !editing()) render();
}

async function pollLogs() {
  try {
    const more = await Call.ByName("main.App.Logs", ui.logSeq);
    if (more?.length) {
      ui.logs.push(...more);
      ui.logSeq = more[more.length - 1].seq;
      if (ui.logs.length > 1500) ui.logs = ui.logs.slice(-1500);
      return true;
    }
  } catch {}
  return false;
}

Events.On("scan:progress", (ev) => {
  const p = ev.data;
  if (!ui.scan) ui.scan = freshScan();
  ui.scan.stage = p.stage;
  ui.scan.done = p.done;
  ui.scan.total = p.total;
  ui.scan.message = p.message;
  if (p.combo) addCombo(ui.scan, p.combo);
  if (!editing()) render();
});

Events.On("scan:done", async (ev) => {
  const d = ev.data || {};
  ui.scan = null;
  if (d.error) toast(d.error);
  else if (d.summary?.best) toast("Tarama bitti: en iyi ayar bulundu.", true);
  await refresh(true);
});

Events.On("state:changed", (ev) => {
  ui.s = ev.data;
  if (!editing()) render();
});

await refresh(true);
setInterval(async () => {
  if (document.hidden) return;
  await refresh();
  if (!isTray && ui.page === "logs" && (await pollLogs()) && !editing()) render();
}, 1000);
