import { Browser, Call, Events } from "/wails/runtime.js";
import { LANGUAGES, detectLanguage, language, setLanguage, t, translateBackend } from "./i18n.js";

// ---------------------------------------------------------------- utils
const isTray = new URLSearchParams(location.search).get("view") === "tray";
const root = document.getElementById("app");
const toastEl = document.getElementById("toast");
if (isTray) document.body.classList.add("tray");
setLanguage(detectLanguage()); // until the saved choice arrives

const esc = (v) => String(v ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

async function api(method, ...args) {
  try {
    return await Call.ByName("main.App." + method, ...args);
  } catch (err) {
    toast(translateBackend(err?.message || String(err)));
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
  if (p.dnsMode === "system") return t("dns.system");
  if ((p.dnsAddr || "").startsWith("77.88.8.8")) return "Yandex";
  if ((p.dnsAddr || "").startsWith("1.1.1.1")) return "CF UDP";
  return "UDP " + (p.dnsAddr || "");
}
const profileLabel = (p) => `${stratLabel(p)} · ${dnsLabel(p)}`;

// Profile names/descriptions/regions come from Go in Turkish; translate them
// by profile id (descriptions fall back to the Go text for Turkish).
function pName(p) {
  const k = "p." + p.id;
  const v = t(k);
  return v === k ? p.name : v;
}
function pDesc(p) {
  if (p.id === "auto") {
    const best = ui.s?.lastScan?.results?.[0];
    if (best) return t("pd.auto", { p: best.passed, t: best.total });
  }
  if (language() === "tr" && p.id !== "auto" && p.id !== "custom") return p.description;
  const k = "pd." + p.id;
  const v = t(k);
  return v === k ? p.description : v;
}
function regionName(r) {
  const k = "region." + r;
  const v = t(k);
  return v === k ? r : v;
}

// Elapsed-time text is patched in place every second (see tickSince) so the
// rendered HTML stays identical and the DOM is not rebuilt each second —
// rebuilding it under the cursor swallowed clicks.
const SINCE_TOKEN = "\u0000since\u0000";
function withSince(text, ts) {
  const html = esc(text);
  // Left empty on purpose: tickSince fills it right after render, so the
  // HTML does not change from one second to the next.
  return html.replace(SINCE_TOKEN, `<span class="since" data-since="${esc(ts)}"></span>`);
}
function tickSince() {
  for (const el of root.querySelectorAll("[data-since]")) {
    const v = since(el.dataset.since);
    if (el.textContent !== v) el.textContent = v;
  }
}

function since(ts) {
  const s = Math.max(0, Math.floor((Date.now() - new Date(ts).getTime()) / 1000));
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60);
  if (h) return `${h} ${t("u.h")} ${m} ${t("u.m")}`;
  if (m) return `${m} ${t("u.m")}`;
  return `${s} ${t("u.s")}`;
}

// ---------------------------------------------------------------- icons
const I = {
  panel: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><rect x="3" y="3" width="7" height="9" rx="1.5"/><rect x="14" y="3" width="7" height="5" rx="1.5"/><rect x="14" y="12" width="7" height="9" rx="1.5"/><rect x="3" y="16" width="7" height="5" rx="1.5"/></svg>',
  bolt: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M13 2 4 14h7l-1 8 9-12h-7z"/></svg>',
  globe: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c2.5 2.7 3.8 5.7 3.8 9s-1.3 6.3-3.8 9c-2.5-2.7-3.8-5.7-3.8-9S9.5 5.7 12 3z"/></svg>',
  list: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M4 6h16M4 12h10M4 18h13"/></svg>',
  gear: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M2 12h3M19 12h3M4.9 19.1 7 17M17 7l2.1-2.1"/></svg>',
};
const spinner = '<span class="spin" aria-hidden="true"></span>';

// The "Kesik" mark. state: on | off | warn | busy (slices slide in turn)
function glyph(state, color, size) {
  if (state === "busy" || state === "settle") {
    return `<svg class="glyph busy ${state === "settle" ? "settle" : ""}" width="${size}" height="${size}" viewBox="0 0 24 24" aria-hidden="true">
      <rect class="s1" x="5" y="3" width="15" height="5" rx="1.2" fill="${color}"/>
      <rect class="s2" x="8" y="9.5" width="15" height="5" rx="1.2" fill="${color}"/>
      <rect class="s3" x="2" y="16" width="15" height="5" rx="1.2" fill="${color}"/></svg>`;
  }
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
  busy: null, // null | "starting" | "stopping"
  region: "*",
  hosts: [],
  logs: [],
  logSeq: 0,
  scan: null, // { stage, done, total, message, cells: Map, rows: [], cols: [] }
  seenCells: new Set(), // matrix cells already shown (so they animate once)
};

function freshScan() {
  ui.seenCells = new Set();
  return { stage: "baseline", done: 0, total: 0, message: t("sm.start"), cells: new Map(), rows: [], cols: [] };
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

// ---------------------------------------------------------------- render
// Scrollable areas whose position must survive a re-render.
const SCROLLERS = [".main", ".matrix-card", ".logs", ".plist"];
let lastHTML = "";
let lastPage = "";

let pointerDown = false;
let renderPending = false;
root.addEventListener("pointerdown", () => { pointerDown = true; });
const pointerUp = () => {
  if (!pointerDown) return;
  pointerDown = false;
  // Let the click event fire on the element that was pressed, then catch up.
  if (renderPending) setTimeout(() => { renderPending = false; render(); }, 0);
};
window.addEventListener("pointerup", pointerUp);
window.addEventListener("pointercancel", pointerUp);
window.addEventListener("blur", pointerUp);

function render() {
  if (!ui.s) return;
  if (pointerDown) { renderPending = true; return; }
  const html = isTray ? trayView() : mainView();
  // Nothing changed: leave the DOM alone (keeps scroll, selection, focus).
  if (html === lastHTML) return;

  const samePage = lastPage === ui.page;
  const saved = SCROLLERS.map((sel) => {
    const el = root.querySelector(sel);
    return el ? { sel, top: el.scrollTop, left: el.scrollLeft, atBottom: el.scrollTop + el.clientHeight >= el.scrollHeight - 8 } : null;
  });
  const logWasAtBottom = saved[2]?.atBottom ?? true;

  root.innerHTML = html;
  lastHTML = html;
  lastPage = ui.page;
  tickSince();

  if (samePage) {
    for (const s of saved) {
      if (!s) continue;
      const el = root.querySelector(s.sel);
      if (el) { el.scrollTop = s.top; el.scrollLeft = s.left; }
    }
  }
  if (!isTray && ui.page === "logs") {
    const box = document.getElementById("logbox");
    // Follow new lines only when the user was already at the bottom.
    if (box && (!samePage || logWasAtBottom)) box.scrollTop = box.scrollHeight;
  }
}

// ---------------------------------------------------------------- views: main
function mainView() {
  const s = ui.s;
  const nav = [
    ["panel", t("nav.panel"), I.panel],
    ["auto", t("nav.auto"), I.bolt],
    ["profiles", t("nav.profiles"), I.globe],
    ["logs", t("nav.logs"), I.list],
    ["settings", t("nav.settings"), I.gear],
  ].map(([id, label, icon]) =>
    `<button data-act="page" data-arg="${id}" ${ui.page === id ? 'aria-current="page"' : ""}>${icon}${label}</button>`).join("");

  const tun = s.options.appMode === "tun";
  const page = { panel: panelPage, auto: autoPage, profiles: profilesPage, logs: logsPage, settings: settingsPage }[ui.page]();
  return `<div class="shell">
    <div class="titlebar">Dikec DPI</div>
    <div class="body">
      <nav class="nav" aria-label="${esc(t("nav.aria"))}">
        <div class="brand">${glyph(ui.busy ? "busy" : s.status.running ? "on" : "off", "var(--accent)", 26)}<b>Dikec<span>DPI</span></b></div>
        ${nav}
        <div class="grow"></div>
        <div class="netcard">
          <span class="eyebrow">${tun ? "TUN" : "Proxy"}</span>
          <span style="font-size:13px;font-weight:500">${tun ? esc(t("net.allSystem")) : esc(s.options.listenAddr)}</span>
          <span class="mono muted" style="font-size:11px">${esc(tun ? (s.status.running ? t("net.utun") : t("common.off")) : s.status.systemProxy ? t("net.sysProxy") : (s.status.running ? t("net.manual") : t("common.off")))}</span>
        </div>
      </nav>
      <main class="main">${page}</main>
    </div>
  </div>`;
}

function panelPage() {
  const s = ui.s, st = s.status, on = st.running, tun = s.options.appMode === "tun";
  const accent = "var(--accent)";
  const busy = ui.busy;
  const ringDash = on ? "46 9.3" : "553 0";
  const ringColor = on ? accent : "#3A3F4B";
  const ring = busy
    ? `<circle class="ring-arc" cx="98" cy="98" r="88" fill="none" stroke="${busy === "stopping" ? "#6E7582" : accent}" stroke-width="10"/>`
    : `<circle class="ring-fg" cx="98" cy="98" r="88" fill="none" stroke="${ringColor}" stroke-width="10" stroke-dasharray="${ringDash}" transform="rotate(-90 98 98)"/>`;
  const best = s.lastScan?.results?.[0];
  const error = translateBackend(s.error);

  let banner = updateBanner(s);
  if (banner) {
    if (error) banner += `<div class="banner err">${glyph("warn", "#FF8A7A", 22)}<p>${esc(error)}</p></div>`;
  } else if (error && !busy) {
    banner = `<div class="banner err">${glyph("warn", "#FF8A7A", 22)}<p>${esc(error)}</p><button class="btn sm" data-act="page" data-arg="settings">${esc(t("nav.settings"))}</button></div>`;
  } else if (s.firstRun && !s.scanning) {
    banner = `<div class="banner">${glyph("on", accent, 22)}<p><b>${esc(t("fr.title"))}</b> ${esc(t("fr.body"))}</p>
      <button class="btn sm primary" data-act="scan">${esc(t("btn.scanNow"))}</button><button class="btn sm" data-act="dismiss">${esc(t("btn.skip"))}</button></div>`;
  } else if (s.scanning) {
    banner = `<div class="banner">${glyph("busy", accent, 22)}<p>${esc(t("banner.scanning"))}</p><button class="btn sm" data-act="page" data-arg="auto">${esc(t("btn.watch"))}</button></div>`;
  }

  const hosts = ui.hosts.length
    ? (() => {
        const max = Math.max(...ui.hosts.map((h) => h.requests));
        return ui.hosts.map((h) => {
          const n = Math.max(1, Math.round((h.requests / max) * 12));
          const segs = Array.from({ length: n }, (_, i) => `<span style="flex:1;background:${i % 2 ? "#3A3F4B" : accent}"></span>`).join("");
          return `<div class="host"><span class="h" title="${esc(h.host)}">${esc(h.host)}</span><span class="bar" aria-hidden="true">${segs}</span><span class="n">${esc(t("panel.requests", { n: h.requests }))}</span></div>`;
        }).join("");
      })()
    : `<div class="empty">${esc(on && tun ? t("panel.tunHint") : on ? t("panel.noTraffic") : t("panel.off"))}</div>`;

  let tag, title, body;
  if (busy) {
    tag = busy === "starting" ? t("busy.startTag") : t("busy.stopTag");
    title = busy === "starting" ? t("busy.starting") : t("busy.stopping");
    body = busy === "starting" ? (tun ? t("busy.tun") : t("busy.proxy")) : t("busy.off");
  } else {
    tag = on ? t("status.active", { t: SINCE_TOKEN }) : t("status.off");
    title = on ? t("hero.onTitle") : t("hero.offTitle");
    body = tun ? (on ? t("hero.tunOn") : t("hero.tunOff")) : on ? t("hero.proxyOn", { addr: st.addr }) : t("hero.proxyOff");
  }

  return `${banner}
  <div class="hero">
    <button class="power ${busy ? "busy " + busy : ""}" data-act="toggle" ${busy ? "disabled" : ""} aria-pressed="${on}" aria-busy="${!!busy}" aria-label="${esc(on ? t("power.off") : t("power.on"))}">
      <svg class="ring" width="196" height="196" viewBox="0 0 196 196" aria-hidden="true">
        <circle cx="98" cy="98" r="88" fill="none" stroke="#1E2129" stroke-width="10"/>
        ${ring}
        <circle cx="98" cy="98" r="70" fill="#14161B" stroke="#262A33"/>
      </svg>
      <span style="position:relative">${glyph(busy === "stopping" ? "settle" : busy ? "busy" : on ? "on" : "off", busy === "stopping" ? "#9AA1AE" : busy || on ? accent : "#6E7582", 64)}</span>
    </button>
    <div class="hero-text">
      <span class="tag" style="color:${busy === "stopping" ? "var(--muted2)" : busy || on ? accent : "var(--muted)"}">${on && !busy ? withSince(tag, st.since) : esc(tag)}</span>
      <h1>${esc(title)}</h1>
      <p>${esc(body)}</p>
      <div class="row" style="margin-top:6px">
        <button class="btn ${busy ? "busy " + busy : on ? "dark" : "primary"}" data-act="toggle" ${busy ? "disabled" : ""}>${busy ? spinner + esc(title) : esc(on ? t("btn.stop") : t("btn.start"))}</button>
        <button class="btn" data-act="scan" ${s.scanning || busy ? "disabled" : ""}>${I.bolt}${esc(s.lastScan ? t("btn.rescan") : t("btn.autoScan"))}</button>
      </div>
    </div>
  </div>
  <div class="grid3">
    <div class="card stat"><span class="k">${esc(t("stat.profile"))}</span><span class="v">${esc(pName(s.selected))}</span><span class="s" style="color:var(--accent)">${esc(profileLabel(s.selected))}</span></div>
    <div class="card stat"><span class="k">${esc(t("stat.unblocked"))}</span><span class="v">${best ? `${best.passed} / ${best.total}` : "—"}</span><span class="s">${esc(best ? t("stat.lastScan", { d: s.lastScan.duration }) : t("stat.noScan"))}</span></div>
    <div class="card stat"><span class="k">${esc(on ? t("stat.requests") : t("stat.latency"))}</span><span class="v">${on ? st.requests : best ? best.avgMillis + " ms" : "—"}</span><span class="s">${esc(on ? (s.unblockedNow ? t("stat.usedBlocked", { n: s.unblockedNow }) : t("stat.allSites")) : t("stat.tls"))}</span></div>
  </div>
  <section class="card" aria-label="${esc(t("live.title"))}" style="display:flex;flex-direction:column;gap:10px">
    <div class="between"><span style="font-size:13px;font-weight:600">${esc(t("live.title"))}</span>${ui.hosts.length ? `<button class="btn sm" data-act="resetstats">${esc(t("btn.reset"))}</button>` : ""}</div>
    <div class="hosts">${hosts}</div>
  </section>`;
}

function updateBanner(s) {
  const u = s.update;
  if (!u || !u.available) return "";
  if (u.installing) {
    return `<div class="banner">${glyph("busy", "var(--accent)", 22)}<p><b>${esc(t("upd.installing", { v: u.latest }))}</b> ${esc(t("upd.installingBody"))}</p></div>`;
  }
  return `<div class="banner">${glyph("on", "var(--accent)", 22)}<p><b>${esc(t("upd.new", { v: u.latest }))}</b> · ${esc(t("upd.current", { v: u.current }))} ${esc(u.viaBrew ? t("upd.brew") : t("upd.page"))}</p>
    <button class="btn sm primary" data-act="installupdate">${esc(u.viaBrew ? t("btn.update") : t("btn.download"))}</button><button class="btn sm" data-act="releasenotes">${esc(t("btn.notes"))}</button></div>`;
}

const STAGES = [["baseline", "stage.baseline"], ["dns", "stage.dns"], ["strategy", "stage.strategy"], ["dns-mix", "stage.mix"]];

function autoPage() {
  const s = ui.s;
  const scan = ui.scan || (s.lastScan ? scanFromSummary(s.lastScan) : null);
  const curIdx = scan ? (scan.stage === "done" ? 99 : STAGES.findIndex(([k]) => k === scan.stage)) : -1;
  const stages = STAGES.map(([k, l], i) => {
    const cls = i < curIdx ? "done" : i === curIdx ? "now" : "";
    return `<li class="${cls}"><i>${i < curIdx ? "✓" : i + 1}</i>${esc(t(l))}</li>`;
  }).join("");

  let head, body;
  if (s.scanning && ui.scan) {
    const pct = ui.scan.total ? Math.round((ui.scan.done / ui.scan.total) * 100) : 4;
    head = `<span class="tag" style="color:var(--accent)">${esc(t("auto.scanningTag"))}</span><h1>${esc(t("auto.scanningTitle"))}</h1>`;
    body = `<div class="card" style="display:flex;flex-direction:column;gap:10px">
      <div class="between"><span class="mono" style="font-size:12px">${esc(ui.scan.message)}</span><button class="btn sm" data-act="cancelscan">${esc(t("btn.cancel"))}</button></div>
      <div class="progress"><span style="width:${pct}%"></span></div></div>
      ${matrixView(ui.scan, null)}`;
  } else if (scan && s.lastScan) {
    const sum = s.lastScan, best = sum.best;
    head = `<span class="tag" style="color:var(--accent)">${esc(t("auto.doneTag", { d: sum.duration }))}</span>
      <h1>${esc(best ? t("auto.found") : t("auto.notFound"))}</h1>`;
    const winner = best ? `<div class="winner">
        <span class="eyebrow" style="color:var(--bg)">${esc(t("auto.winner"))}</span>
        <span class="big">${esc(stratLabel(best))}</span>
        <span class="mono" style="font-size:12px">${esc(dnsLabel(best))} · ${sum.results[0].passed}/${sum.results[0].total} · ${sum.results[0].avgMillis} ms</span>
        <button class="btn" data-act="applyauto">${esc(s.selectedId === "auto" && s.status.running ? t("btn.applied") : t("btn.apply"))}</button>
      </div>` : `<div class="card"><p class="muted">${esc(t("auto.noneWorked"))}</p></div>`;
    const maxMs = Math.max(1, ...(sum.dns || []).filter((d) => d.ok).map((d) => d.avgMillis));
    const dns = (sum.dns || []).map((d) => `<div class="dnsrow"><span class="nm">${esc(d.name === "Sistem DNS" ? t("st.dnsSystem") : d.name)}</span><span class="tr" aria-hidden="true"><span style="width:${d.ok ? Math.max(4, (d.avgMillis / maxMs) * 100) : 0}%"></span></span><span class="ms">${d.ok ? d.avgMillis + " ms" : esc(t("auto.na"))}</span></div>`).join("");
    body = `<div class="scan-layout">
      <div class="card matrix-card">${matrixView(scan, best)}</div>
      <aside class="aside">${winner}
        <div class="card" style="display:flex;flex-direction:column;gap:8px"><span style="font-size:12px;font-weight:600">${esc(t("auto.dnsTitle"))}</span>${dns}</div>
        ${targetsCard(sum)}
      </aside></div>`;
  } else {
    head = `<span class="tag muted">${esc(t("auto.introTag"))}</span><h1>${esc(t("auto.introTitle"))}</h1>`;
    body = `<div class="card" style="display:flex;flex-direction:column;gap:14px;max-width:620px">
      <p class="muted">${esc(t("auto.introBody"))}</p>
      <div class="row"><button class="btn primary" data-act="scan">${I.bolt}${esc(t("btn.startScan"))}</button><span class="muted" style="font-size:12px">${esc(t("auto.duration"))}</span></div>
    </div>${targetsCard(null)}`;
  }
  return `<div class="page-head"><div>${head}</div><ol class="stages" aria-label="${esc(t("auto.stagesAria"))}">${stages}</ol></div>${body}`;
}

function targetsCard(sum) {
  const blocked = sum ? `<span class="muted" style="font-size:12px;line-height:1.5">${esc(sum.blocked?.length ? t("tg.blocked", { list: sum.blocked.join(", ") }) : t("tg.noneBlocked"))}</span>` : "";
  return `<div class="card" style="display:flex;flex-direction:column;gap:8px">
    <label for="targets" style="font-size:12px;font-weight:600">${esc(t("tg.title"))}</label>
    <textarea id="targets" class="targets" spellcheck="false" dir="ltr">${esc(ui.s.targets.join("\n"))}</textarea>
    <div class="between">${blocked}<button class="btn sm" data-act="savetargets">${esc(t("btn.save"))}</button></div>
  </div>`;
}

function matrixView(scan, best) {
  if (!scan.rows.length) return `<div class="empty">${esc(t("mx.empty"))}</div>`;
  const cols = scan.cols;
  const bestKey = best ? stratLabel(best) : null;
  const head = `<div class="mrow" style="--cols:${cols.length}"><span class="hd" style="text-align:start;font-weight:600">${esc(t("mx.strategy"))}</span>${cols.map((c) => `<span class="hd">${esc(c)}</span>`).join("")}</div>`;
  const rows = scan.rows.map((r) => {
    const cells = cols.map((c) => {
      const x = scan.cells.get(r + "|" + c);
      if (!x) return `<span class="cell"></span>`;
      const cls = x.passed === x.total ? "p3" : x.passed > 0 ? "p2" : "p0";
      const txt = x.passed === x.total ? `✓ ${x.avgMillis} ms` : x.passed > 0 ? `${x.passed}/${x.total}` : "×";
      const tip = [t("mx.tip", { r, c, p: x.passed, t: x.total }) + (x.passed ? t("mx.tipAvg", { ms: x.avgMillis }) : ""),
        ...(x.targets || []).map((tg) => `${tg.target}: ${tg.ok ? t("mx.open") : tg.error || t("mx.blocked")}`)].join("\n");
      const key = r + "|" + c + "|" + x.passed;
      const fresh = scan.stage !== "done" && !ui.seenCells.has(key);
      ui.seenCells.add(key);
      return `<span class="cell ${cls}${fresh ? " fresh" : ""}" title="${esc(tip)}" dir="ltr">${txt}</span>`;
    }).join("");
    const isBest = r === bestKey;
    return `<div class="mrow" style="--cols:${cols.length}"><span class="name ${isBest ? "best" : ""}" dir="ltr">${isBest ? "★ " : ""}${esc(r)}</span>${cells}</div>`;
  }).join("");
  return `<div class="matrix">${head}${rows}
    <div class="legend"><span><b style="background:var(--accent)"></b>${esc(t("lg.all"))}</span><span><b style="background:var(--accent-mid)"></b>${esc(t("lg.partial"))}</span><span><b style="background:var(--line);border:1px solid var(--line3)"></b>${esc(t("lg.blocked"))}</span><span>${esc(t("lg.untested"))}</span></div></div>`;
}

function profilesPage() {
  const s = ui.s;
  const regions = ["*", ...new Set(s.profiles.map((p) => p.region))];
  const tabs = regions.map((r) => `<button role="tab" aria-selected="${r === ui.region}" data-act="region" data-arg="${esc(r)}">${esc(r === "*" ? t("pf.all") : regionName(r))}</button>`).join("");
  const cards = s.profiles.filter((p) => ui.region === "*" || p.region === ui.region).map((p) => {
    const b = p.needsRoot ? ["r", t("badge.root")] : p.verified ? ["v", t("badge.verified")] : ["s", t("badge.starter")];
    return `<button class="pcard" data-act="select" data-arg="${esc(p.id)}" aria-pressed="${p.id === s.selectedId}">
      <span class="between"><span class="muted" style="font-size:11px">${esc(regionName(p.region))}</span><span class="badge ${b[0]}">${esc(b[1])}</span></span>
      <span class="nm">${esc(pName(p))}</span>
      <span class="lb" dir="ltr">${esc(profileLabel(p))}</span>
      <span class="ds">${esc(pDesc(p))}</span>
    </button>`;
  }).join("");
  return `<div class="page-head">
      <div><span class="tag muted">${esc(t("pf.tag", { n: s.profiles.length }))}</span><h1>${esc(t("pf.title"))}</h1></div>
      <button class="btn primary" data-act="scan" ${s.scanning ? "disabled" : ""}>${I.bolt}${esc(t("btn.testMine"))}</button>
    </div>
    <div class="tabs" role="tablist" aria-label="${esc(t("pf.regionAria"))}">${tabs}</div>
    <div class="pgrid">${cards}</div>
    <div class="card row" style="flex-wrap:wrap;font-size:12px;color:var(--muted2)">
      <span class="badge v">${esc(t("badge.verified"))}</span>${esc(t("pf.legendVerified"))}
      <span class="badge s">${esc(t("badge.starter"))}</span>${esc(t("pf.legendStarter"))}
      <span class="badge r">${esc(t("badge.root"))}</span>${esc(t("pf.legendRoot"))}
    </div>`;
}

function logsPage() {
  const rows = ui.logs.slice(-600).map((l) => {
    const tm = new Date(l.time).toLocaleTimeString(language());
    const f = Object.entries(l.fields || {}).filter(([k]) => k !== "step").map(([k, v]) => `${k}=${v}`).join(" ");
    return `<div class="log"><span class="t">${tm}</span><span class="l ${esc(l.level)}">${esc(l.level)}</span><span class="sc">[${esc(l.scope || "app")}]</span><span>${esc(l.msg)} <span class="f">${esc(f)}</span></span></div>`;
  }).join("");
  return `<div class="page-head"><div><span class="tag muted">${esc(t("log.tag"))}</span><h1>${esc(t("log.title"))}</h1></div><span class="muted" style="font-size:12px">${esc(t("log.hint"))}</span></div>
    <div class="logs" id="logbox" dir="ltr">${rows || `<div class="empty">${esc(t("log.empty"))}</div>`}</div>`;
}

function settingsPage() {
  const s = ui.s, o = s.options, p = s.selected;
  const sw = (id, on, label) => `<button class="switch" role="switch" aria-checked="${on}" aria-label="${esc(label)}" data-act="sw" data-arg="${id}"><span></span></button>`;
  const opt = (v, cur, label) => `<option value="${v}" ${v === cur ? "selected" : ""}>${esc(label)}</option>`;
  const langs = LANGUAGES.map((l) => opt(l.code, language(), l.name)).join("");
  return `<div style="display:flex;flex-direction:column;gap:6px"><span class="tag muted">${esc(t("st.tag"))}</span><h1>${esc(t("st.title"))}</h1></div>

  <section class="card" style="display:flex;flex-direction:column;gap:14px">
    <h2>${esc(t("st.general"))}</h2>
    <div class="form">
      <label class="k" for="o-lang">${esc(t("st.language"))}</label><select id="o-lang">${langs}</select>
      <span class="hint">${esc(t("st.languageHint"))}</span>
      <label class="k">${esc(t("st.startOnLaunch"))}</label><div>${sw("startOnLaunch", s.startOnLaunch, t("st.startOnLaunch"))}</div>
      <label class="k">${esc(t("st.trayCount"))}</label><div>${sw("showTrayCount", s.showTrayCount, t("st.trayCount"))}</div>
      <span class="hint">${esc(t("st.trayCountHint"))}</span>
    </div>
  </section>

  <section class="card" style="display:flex;flex-direction:column;gap:14px">
    <h2>${esc(t("st.connection"))}</h2>
    <div class="form">
      <label class="k" for="o-listen">${esc(t("st.listen"))}</label><input type="text" id="o-listen" dir="ltr" value="${esc(o.listenAddr)}" spellcheck="false">
      <label class="k" for="o-mode">${esc(t("st.mode"))}</label><select id="o-mode">${opt("http", o.appMode || "http", t("st.modeHttp"))}${opt("socks5", o.appMode, t("st.modeSocks"))}${opt("tun", o.appMode, t("st.modeTun"))}</select>
      <span class="hint">${esc(t("st.tunHint"))}</span>
      <label class="k">${esc(t("st.sysProxy"))}</label><div>${sw("systemProxy", o.systemProxy, t("st.sysProxy"))}</div>
      <span class="hint">${esc(t("st.sysProxyHint", { addr: o.listenAddr }))}</span>
      <label class="k">${esc(t("st.ipv6"))}</label><div>${sw("ipv6", o.ipv6, t("st.ipv6"))}</div>
      <label class="k">${esc(t("st.dnsCache"))}</label><div>${sw("dnsCache", o.dnsCache, t("st.dnsCache"))}</div>
    </div>
    <div><button class="btn primary" data-act="saveoptions">${esc(t("btn.save"))}</button></div>
  </section>

  <section class="card" style="display:flex;flex-direction:column;gap:14px">
    <div class="between"><h2>${esc(t("st.custom"))}</h2><span class="muted" style="font-size:12px">${esc(t("st.current", { p: pName(p) + " · " + profileLabel(p) }))}</span></div>
    <div class="form">
      <label class="k" for="c-split">${esc(t("st.split"))}</label><select id="c-split">${["tlsrec", "chunk", "sni", "first-byte", "random", "none"].map((m) => opt(m, p.splitMode, m)).join("")}</select>
      <label class="k" for="c-chunk">${esc(t("st.chunk"))}</label><input type="number" id="c-chunk" min="1" max="255" value="${p.chunkSize || 5}">
      <label class="k">${esc(t("st.disorder"))}</label><div>${sw("c-disorder", p.disorder, t("st.disorder"))}</div>
      <label class="k" for="c-dns">${esc(t("st.dns"))}</label><select id="c-dns">${opt("udp", p.dnsMode || "udp", "UDP")}${opt("https", p.dnsMode, "DNS over HTTPS")}${opt("system", p.dnsMode, t("st.dnsSystem"))}</select>
      <label class="k" for="c-addr">${esc(t("st.udpAddr"))}</label><input type="text" id="c-addr" dir="ltr" value="${esc(p.dnsAddr || "77.88.8.8:1253")}" spellcheck="false">
      <label class="k" for="c-doh">${esc(t("st.dohAddr"))}</label><input type="text" id="c-doh" dir="ltr" value="${esc(p.dohUrl || "https://1.1.1.1/dns-query")}" spellcheck="false">
    </div>
    <div><button class="btn" data-act="savecustom">${esc(t("btn.saveCustom"))}</button></div>
  </section>

  <section class="card credits">
    <h2 style="margin-bottom:8px">${esc(t("st.about"))}</h2>
    <div class="row" style="flex-wrap:wrap;margin-bottom:6px">Dikec DPI ${esc(s.version)} · Apache 2.0
      <button class="btn sm" data-act="checkupdate">${esc(t("btn.checkUpdate"))}</button>
      <span class="muted" style="font-size:12px">${esc(updateStatus(s.update))}</span></div>
    ${t("st.credits", { core: "<b>xvzc/SpoofDPI</b>", tr: "<b>mertikavi/SpoofDPI-Turkiye</b>" })}<br>
    ${esc(t("st.restore"))}
  </section>`;
}

function updateStatus(u) {
  if (!u || !u.checkedAt || u.checkedAt.startsWith("0001")) return "";
  if (u.error) return translateBackend(u.error);
  if (u.available) return t("us.new", { v: u.latest });
  return t("us.current", { t: new Date(u.checkedAt).toLocaleTimeString(language(), { hour: "2-digit", minute: "2-digit" }) });
}

// ---------------------------------------------------------------- views: tray
function trayView() {
  const s = ui.s, on = s.status.running, busy = ui.busy;
  const featured = s.profiles.filter((p) => p.id === "auto" || p.id === "custom" || p.region === "Türkiye").slice(0, 5);
  if (!featured.some((p) => p.id === s.selectedId)) featured.unshift(s.selected);
  const items = featured.map((p) => `<button class="pitem" data-act="select" data-arg="${esc(p.id)}" aria-pressed="${p.id === s.selectedId}">
      <span class="dot" aria-hidden="true"></span><span class="nm">${esc(pName(p))}</span><span class="lb" dir="ltr">${esc(stratLabel(p))}</span></button>`).join("");
  const scanLine = s.scanning
    ? t("tray.scanning") + (ui.scan?.total ? ` · ${ui.scan.done}/${ui.scan.total}` : "…")
    : s.lastScan ? t("tray.lastScan", { n: s.lastScan.combos, d: s.lastScan.duration }) : t("tray.noScan");
  const title = busy ? (busy === "starting" ? t("busy.starting") : t("busy.stopping")) : on ? t("tray.on") : t("tray.off");
  const sub = busy ? (busy === "starting" && s.options.appMode === "tun" ? t("busy.tun") : busy === "starting" ? t("busy.startTag") : t("busy.stopTag"))
    : (on ? t("tray.active", { t: SINCE_TOKEN }) : t("tray.inactive")) + (s.options.appMode === "tun" ? " · TUN" : "");
  const error = translateBackend(s.error);
  return `<div class="pop">
    <div class="head">
      ${glyph(busy ? "busy" : error && on ? "warn" : on ? "on" : "off", busy || on ? "var(--accent)" : "#6E7582", 34)}
      <div style="flex:1;display:flex;flex-direction:column;gap:2px;min-width:0">
        <span class="ttl">${esc(title)}</span>
        <span class="mono" style="font-size:11px;color:${busy || on ? "var(--accent)" : "var(--muted)"}">${on && !busy ? withSince(sub, s.status.since) : esc(sub)}</span>
      </div>
      <button class="switch ${busy ? "busy" : ""}" role="switch" aria-checked="${busy ? busy === "starting" : on}" aria-busy="${!!busy}" ${busy ? "disabled" : ""} aria-label="${esc(t("tray.toggleAria"))}" data-act="toggle"><span></span></button>
    </div>
    ${error && !busy ? `<div class="mono" style="font-size:11px;color:var(--danger);line-height:1.4">${esc(error)}</div>` : ""}
    <div class="sep"></div>
    <span class="eyebrow">${esc(t("tray.profile"))}</span>
    <div class="plist">${items}</div>
    <div class="sep"></div>
    <div class="scan">
      ${I.bolt.replace('stroke="currentColor"', 'stroke="#C8F25A"')}
      <div style="flex:1;display:flex;flex-direction:column;gap:2px;min-width:0"><span style="font-size:13px;font-weight:500">${esc(t("nav.auto"))}</span><span class="muted" style="font-size:11px">${esc(scanLine)}</span></div>
      <button class="btn sm primary" data-act="${s.scanning ? "openmain" : "scan"}">${esc(s.scanning ? t("btn.watch") : t("btn.scan"))}</button>
    </div>
    ${s.update?.available ? `<button class="btn sm primary" data-act="${s.update.installing ? "openmain" : "installupdate"}" style="width:100%">${esc(s.update.installing ? t("tray.updating") : t("tray.newVersion", { v: s.update.latest }))}</button>` : ""}
    <div class="links">
      <button data-act="openmain">${esc(t("tray.open"))} <kbd>⌘O</kbd></button>
      <button data-act="quit">${esc(t("tray.quit"))} <kbd>⌘Q</kbd></button>
    </div>
  </div>`;
}

// ---------------------------------------------------------------- actions
async function act(name, arg, el) {
  switch (name) {
    case "page":
      ui.page = arg;
      render(); // switch instantly; data fills in afterwards
      if (arg === "logs" && (await pollLogs())) render();
      else if (arg === "panel" && ui.s.status.running) refresh(true);
      break;
    case "region":
      ui.region = arg;
      render();
      break;
    case "toggle":
      if (ui.busy) return;
      ui.busy = ui.s.status.running ? "stopping" : "starting";
      render();
      try {
        // Keep the transition visible for a moment even when it is instant.
        await Promise.all([api("Toggle"), new Promise((r) => setTimeout(r, 900))]);
      } finally {
        ui.busy = null;
        await refresh(true);
      }
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
      ui.busy = "starting"; render();
      try { await api("ApplyAuto"); toast(t("t.autoApplied"), true); } finally { ui.busy = null; await refresh(true); }
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
      toast(t("t.targetsSaved", { n: clean.length }), true);
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
      const mode = document.getElementById("o-mode").value;
      await api("SaveOptions", {
        listenAddr: document.getElementById("o-listen").value.trim(),
        appMode: mode,
        systemProxy: sw("systemProxy"),
        ipv6: sw("ipv6"),
        dnsCache: sw("dnsCache"),
      });
      ui.logs = [];
      ui.logSeq = 0;
      toast(mode === "tun" ? t("t.tunSelected") : t("t.saved"), true);
      await refresh(true);
      break;
    }
    case "setlang":
      setLanguage(arg);
      await api("SetLanguage", language());
      lastHTML = "";
      await refresh(true);
      break;
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
      toast(t("t.customSaved"), true);
      await refresh(true);
      break;
    }
    case "openmain":
      await api("OpenMain");
      break;
    case "checkupdate": {
      el.disabled = true;
      const u = await api("CheckUpdate");
      toast(u.error ? translateBackend(u.error) : u.available ? t("us.new", { v: u.latest }) : t("t.upToDate"), !u.error);
      await refresh(true);
      break;
    }
    case "installupdate": {
      const how = await api("InstallUpdate");
      toast(how === "brew" ? t("t.updateStarted") : t("t.pageOpened"), true);
      await refresh(true);
      break;
    }
    case "releasenotes":
      await Browser.OpenURL(ui.s.update.url);
      break;
    case "quit":
      await api("Quit");
      break;
  }
}

root.addEventListener("change", (e) => {
  if (e.target.id === "o-mode") act("saveoptions").catch(() => {});
  if (e.target.id === "o-lang") act("setlang", e.target.value).catch(() => {});
});

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
  // The settings page holds unsaved form state (selects, switches); only
  // explicit actions re-render it, never background refreshes.
  if (!isTray && ui.page === "settings") return true;
  const a = document.activeElement;
  return a && root.contains(a) && /^(INPUT|TEXTAREA|SELECT)$/.test(a.tagName);
}

// Apply the saved language (or save the detected one on first run), so
// every window and the native menu agree.
let langSynced = false;
function syncLanguage() {
  const saved = ui.s?.language;
  if (saved && saved !== language()) {
    setLanguage(saved);
    lastHTML = "";
  }
  if (!saved && !langSynced) {
    langSynced = true;
    Call.ByName("main.App.SetLanguage", language()).catch(() => {});
  }
}

async function refresh(force = false) {
  try {
    ui.s = await Call.ByName("main.App.GetState");
  } catch { return; }
  syncLanguage();
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
      if (more[0].seq <= ui.logSeq) ui.logs = []; // source restarted
      ui.logs.push(...more);
      ui.logSeq = more[more.length - 1].seq;
      if (ui.logs.length > 1500) ui.logs = ui.logs.slice(-1500);
      return true;
    }
  } catch {}
  return false;
}

function scanMessage(p) {
  if (p.combo) return profileLabel(p.combo.profile);
  if (p.stage === "baseline") return t("sm.baseline");
  if (p.stage === "dns") return t("sm.dns", { i: Math.min(p.done + 1, p.total), n: p.total });
  return p.message;
}

Events.On("scan:progress", (ev) => {
  const p = ev.data;
  if (!ui.scan) ui.scan = freshScan();
  ui.scan.stage = p.stage;
  ui.scan.done = p.done;
  ui.scan.total = p.total;
  ui.scan.message = scanMessage(p);
  if (p.combo) addCombo(ui.scan, p.combo);
  if (!editing()) render();
});

Events.On("scan:done", async (ev) => {
  const d = ev.data || {};
  ui.scan = null;
  if (d.error) toast(translateBackend(d.error));
  else if (d.summary?.best) toast(t("t.scanDone"), true);
  await refresh(true);
});

Events.On("update:available", () => refresh(true));

Events.On("state:changed", (ev) => {
  ui.s = ev.data;
  syncLanguage();
  if (!editing()) render();
});

// A window that was hidden (closed to the menu bar, or the popover) shows a
// stale frame until the next poll; refresh the moment it becomes visible.
document.addEventListener("visibilitychange", () => { if (!document.hidden) refresh(true); });
window.addEventListener("focus", () => refresh(true));

await refresh(true);
setInterval(tickSince, 1000);
setInterval(async () => {
  if (document.hidden) return;
  await refresh();
  if (!isTray && ui.page === "logs" && (await pollLogs()) && !editing()) render();
}, 1000);
