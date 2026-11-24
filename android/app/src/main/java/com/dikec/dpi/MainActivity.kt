package com.dikec.dpi

import android.Manifest
import android.content.Intent
import android.net.Uri
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.dikec.dpi.core.mobile.Mobile
import kotlinx.coroutines.delay

// "Kesik" palette, shared with the desktop app.
private val Bg = Color(0xFF0B0C0F)
private val Panel = Color(0xFF14161B)
private val Panel2 = Color(0xFF1B1E25)
private val Line = Color(0xFF262A33)
private val TextMain = Color(0xFFF2F3F5)
private val Muted = Color(0xFF9AA1AE)
private val Accent = Color(0xFFC8F25A)
private val AccentMid = Color(0xFF6B7D33)
private val Danger = Color(0xFFFF8A7A)

class MainActivity : ComponentActivity() {

    private val consent = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { r ->
        if (r.resultCode == RESULT_OK) DikecVpnService.start(this)
        else Toast.makeText(this, R.string.android_consentdenied, Toast.LENGTH_LONG).show()
    }

    private val notifPermission = registerForActivityResult(ActivityResultContracts.RequestPermission()) {}

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        if (Build.VERSION.SDK_INT >= 33) notifPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
        Scanner.load(this)
        setContent { App(onToggle = ::toggle, onOpenReleases = ::openReleases) }
        // Debug builds only: `adb shell am start -n com.dikec.dpi/.MainActivity --ez autostart true`
        // starts protection (used by the emulator tests).
        debugCommands(intent)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        debugCommands(intent)
    }

    /** Debug builds only: hooks used by the emulator tests over adb. */
    private fun debugCommands(intent: Intent) {
        if (!BuildConfig.DEBUG) return
        if (intent.getBooleanExtra("autostart", false)) toggle()
        if (intent.getBooleanExtra("scan", false)) Scanner.start(this)
    }

    private fun toggle() {
        when (DikecVpnService.state.value.phase) {
            Phase.On, Phase.Starting -> DikecVpnService.stop(this)
            else -> {
                val ask = VpnService.prepare(this)
                if (ask != null) consent.launch(ask) else DikecVpnService.start(this)
            }
        }
    }

    private fun openReleases() {
        startActivity(Intent(Intent.ACTION_VIEW, Uri.parse("https://github.com/dikeckaan/DikecDPI/releases/latest")))
    }
}

private enum class Tab { Panel, Auto, Profiles, About }

@Composable
private fun App(onToggle: () -> Unit, onOpenReleases: () -> Unit) {
    var tab by remember { mutableStateOf(Tab.Panel) }
    var refresh by remember { mutableIntStateOf(0) } // bump to re-read the store
    Column(Modifier.fillMaxSize().background(Bg).safeDrawingPadding()) {
        Box(Modifier.weight(1f).fillMaxWidth()) {
            when (tab) {
                Tab.Panel -> PanelScreen(onToggle, refresh) { tab = Tab.Auto }
                Tab.Auto -> AutoScreen(onApplied = { refresh++; tab = Tab.Panel })
                Tab.Profiles -> ProfilesScreen(refresh) { refresh++ }
                Tab.About -> AboutScreen(onOpenReleases)
            }
        }
        BottomBar(tab) { tab = it }
    }
}

@Composable
private fun BottomBar(current: Tab, onSelect: (Tab) -> Unit) {
    val labels = mapOf(
        Tab.Panel to stringResource(R.string.nav_panel),
        Tab.Auto to stringResource(R.string.nav_auto),
        Tab.Profiles to stringResource(R.string.nav_profiles),
        Tab.About to stringResource(R.string.st_about),
    )
    Row(
        Modifier.fillMaxWidth().background(Panel).padding(horizontal = 8.dp, vertical = 6.dp),
        horizontalArrangement = Arrangement.SpaceAround,
    ) {
        Tab.entries.forEach { t ->
            val sel = t == current
            Box(
                Modifier.clip(RoundedCornerShape(10.dp))
                    .background(if (sel) Panel2 else Color.Transparent)
                    .clickable { onSelect(t) }
                    .padding(horizontal = 12.dp, vertical = 12.dp),
            ) {
                Text(labels.getValue(t), color = if (sel) TextMain else Muted, fontSize = 13.sp, fontWeight = FontWeight.Medium)
            }
        }
    }
}

// ---------------------------------------------------------------- panel

@Composable
private fun PanelScreen(onToggle: () -> Unit, refresh: Int, onScan: () -> Unit) {
    val ctx = LocalContext.current
    val vpn by DikecVpnService.state.collectAsState()
    val scan by Scanner.state.collectAsState()
    val profile = remember(refresh, vpn.phase) { Store(ctx).selected() }
    val on = vpn.phase == Phase.On
    val busy = vpn.phase == Phase.Starting || vpn.phase == Phase.Stopping
    var now by remember { mutableLongStateOf(System.currentTimeMillis()) }
    LaunchedEffect(on) { while (on) { now = System.currentTimeMillis(); delay(1000) } }

    LazyColumn(Modifier.fillMaxSize().padding(horizontal = 20.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
        item { Spacer(Modifier.height(12.dp)) }
        item {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Mark(Accent, 26.dp, on || busy)
                Spacer(Modifier.width(10.dp))
                Text("Dikec", color = TextMain, fontSize = 20.sp, fontWeight = FontWeight.Bold)
                Text("DPI", color = Accent, fontSize = 20.sp, fontWeight = FontWeight.Bold)
            }
        }
        item {
            Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                PowerButton(on, vpn.phase, onToggle)
            }
        }
        item {
            val tag = when (vpn.phase) {
                Phase.Starting -> stringResource(R.string.busy_starttag)
                Phase.Stopping -> stringResource(R.string.busy_stoptag)
                Phase.On -> stringResource(R.string.status_active, elapsed(ctx, now - vpn.since))
                Phase.Off -> stringResource(R.string.status_off)
            }
            val title = when (vpn.phase) {
                Phase.Starting -> stringResource(R.string.busy_starting)
                Phase.Stopping -> stringResource(R.string.busy_stopping)
                Phase.On -> stringResource(R.string.hero_ontitle)
                Phase.Off -> stringResource(R.string.hero_offtitle)
            }
            Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.fillMaxWidth()) {
                Text(tag.uppercase(), color = if (on || busy) Accent else Muted, fontFamily = FontFamily.Monospace, fontSize = 12.sp)
                Spacer(Modifier.height(6.dp))
                Text(title, color = TextMain, fontSize = 30.sp, fontWeight = FontWeight.SemiBold)
                Spacer(Modifier.height(8.dp))
                Text(stringResource(R.string.android_vpnnote), color = Muted, fontSize = 13.sp, lineHeight = 19.sp)
                vpn.error?.let {
                    Spacer(Modifier.height(10.dp))
                    Text(it, color = Danger, fontSize = 13.sp)
                }
            }
        }
        item {
            PrimaryButton(
                text = if (on || vpn.phase == Phase.Starting) stringResource(R.string.btn_stop) else stringResource(R.string.btn_start),
                filled = !on,
                enabled = !busy,
                onClick = onToggle,
            )
        }
        item {
            Card {
                Text(stringResource(R.string.stat_profile), color = Muted, fontSize = 12.sp)
                Text(displayName(ctx, profile), color = TextMain, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
                Text(profile.label(stringResource(R.string.dns_system)), color = Accent, fontFamily = FontFamily.Monospace, fontSize = 12.sp)
            }
        }
        item {
            Card(Modifier.clickable(onClick = onScan)) {
                Text(stringResource(R.string.stat_unblocked), color = Muted, fontSize = 12.sp)
                val best = scan.combos.firstOrNull()
                Text(if (best != null) "${best.passed} / ${best.total}" else "—", color = TextMain, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
                Text(
                    if (best != null) "${best.strategy} · ${best.dns} · ${best.avgMillis} ms" else stringResource(R.string.stat_noscan),
                    color = Muted, fontFamily = FontFamily.Monospace, fontSize = 12.sp,
                )
            }
        }
        item { Spacer(Modifier.height(8.dp)) }
    }
}

private fun elapsed(ctx: android.content.Context, ms: Long): String {
    val s = (ms / 1000).coerceAtLeast(0)
    val h = s / 3600
    val m = (s % 3600) / 60
    return when {
        h > 0 -> "$h ${ctx.getString(R.string.u_h)} $m ${ctx.getString(R.string.u_m)}"
        m > 0 -> "$m ${ctx.getString(R.string.u_m)}"
        else -> "$s ${ctx.getString(R.string.u_s)}"
    }
}

/** The big ring: segmented when on, a turning arc while (dis)connecting. */
@Composable
private fun PowerButton(on: Boolean, phase: Phase, onClick: () -> Unit) {
    val busy = phase == Phase.Starting || phase == Phase.Stopping
    val spin = rememberInfiniteTransition(label = "spin")
    val angle by spin.animateFloat(0f, 360f, infiniteRepeatable(tween(1600, easing = LinearEasing)), label = "angle")
    val sweep by spin.animateFloat(20f, 220f, infiniteRepeatable(tween(800), RepeatMode.Reverse), label = "sweep")
    val label = stringResource(if (on) R.string.power_off else R.string.power_on)
    Box(
        Modifier.size(210.dp).clip(CircleShape).clickable(enabled = !busy, onClick = onClick)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) {
        Canvas(Modifier.fillMaxSize()) {
            val w = 11.dp.toPx()
            val inset = w / 2 + 6.dp.toPx()
            val box = Size(size.width - inset * 2, size.height - inset * 2)
            val tl = Offset(inset, inset)
            drawArc(Line, 0f, 360f, false, tl, box, style = Stroke(w))
            when {
                busy -> drawArc(
                    if (phase == Phase.Stopping) Muted else Accent,
                    if (phase == Phase.Stopping) -angle else angle, sweep, false, tl, box,
                    style = Stroke(w, cap = StrokeCap.Round),
                )
                on -> for (i in 0 until 10) drawArc(Accent, -90f + i * 36f + 2f, 32f, false, tl, box, style = Stroke(w))
            }
            drawCircle(Panel, radius = size.minDimension / 2 - 30.dp.toPx())
        }
        Mark(if (on || busy) Accent else Color(0xFF6E7582), 68.dp, on || busy)
    }
}

/** The "Kesik" mark: aligned slices when off, offset when active. */
@Composable
private fun Mark(color: Color, size: androidx.compose.ui.unit.Dp, active: Boolean) {
    val offs = listOf(if (active) 5f else 4.5f, if (active) 8f else 4.5f, if (active) 2f else 4.5f)
    val a1 by animateFloatAsState(offs[0], label = "s1")
    val a2 by animateFloatAsState(offs[1], label = "s2")
    val a3 by animateFloatAsState(offs[2], label = "s3")
    Canvas(Modifier.size(size)) {
        val u = this.size.width / 24f
        val alpha = if (active) 1f else 0.55f
        slice(color.copy(alpha = alpha), a1 * u, 3 * u, u)
        slice(color.copy(alpha = alpha), a2 * u, 9.5f * u, u)
        slice(color.copy(alpha = alpha), a3 * u, 16 * u, u)
    }
}

private fun DrawScope.slice(color: Color, x: Float, y: Float, u: Float) =
    drawRoundRect(color, Offset(x, y), Size(15 * u, 5 * u), CornerRadius(1.2f * u))

// ---------------------------------------------------------------- auto bypass

@Composable
private fun AutoScreen(onApplied: () -> Unit) {
    val ctx = LocalContext.current
    val scan by Scanner.state.collectAsState()
    LazyColumn(Modifier.fillMaxSize().padding(horizontal = 20.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        item { Spacer(Modifier.height(12.dp)) }
        item {
            val title = when {
                scan.running -> stringResource(R.string.auto_scanningtitle)
                scan.best != null -> stringResource(R.string.auto_found)
                scan.stage == "done" -> stringResource(R.string.auto_notfound)
                else -> stringResource(R.string.auto_introtitle)
            }
            Text(stringResource(R.string.nav_auto).uppercase(), color = Accent, fontFamily = FontFamily.Monospace, fontSize = 12.sp)
            Text(title, color = TextMain, fontSize = 26.sp, fontWeight = FontWeight.SemiBold, lineHeight = 31.sp)
        }
        if (scan.running) {
            item {
                Card {
                    Text(scan.message, color = TextMain, fontFamily = FontFamily.Monospace, fontSize = 12.sp)
                    Spacer(Modifier.height(8.dp))
                    LinearProgressIndicator(
                        progress = { if (scan.total > 0) scan.done.toFloat() / scan.total else 0.05f },
                        modifier = Modifier.fillMaxWidth(), color = Accent, trackColor = Line,
                    )
                    Spacer(Modifier.height(8.dp))
                    SmallButton(stringResource(R.string.btn_cancel)) { Scanner.cancel() }
                }
            }
        } else if (scan.best == null) {
            item {
                Card {
                    Text(stringResource(R.string.auto_introbody), color = Muted, fontSize = 13.sp, lineHeight = 19.sp)
                    Spacer(Modifier.height(10.dp))
                    PrimaryButton(stringResource(R.string.btn_startscan), filled = true) { Scanner.start(ctx) }
                    Text(stringResource(R.string.auto_duration), color = Muted, fontSize = 12.sp)
                }
            }
        }
        scan.best?.takeIf { !scan.running }?.let { best ->
            item {
                Column(
                    Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(Accent).padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    Text(stringResource(R.string.auto_winner).uppercase(), color = Bg, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
                    Text(best.strategy(), color = Bg, fontSize = 26.sp, fontWeight = FontWeight.Bold)
                    val top = scan.combos.firstOrNull()
                    Text(
                        "${best.dns(stringResource(R.string.dns_system))}" + (top?.let { " · ${it.passed}/${it.total} · ${it.avgMillis} ms" } ?: ""),
                        color = Bg, fontFamily = FontFamily.Monospace, fontSize = 12.sp,
                    )
                    Box(
                        Modifier.fillMaxWidth().clip(RoundedCornerShape(10.dp)).background(Bg)
                            .clickable {
                                Store(ctx).selectedId = "auto"
                                if (DikecVpnService.state.value.phase == Phase.On) {
                                    DikecVpnService.restart(ctx)
                                } else if (VpnService.prepare(ctx) == null) {
                                    DikecVpnService.start(ctx)
                                }
                                onApplied()
                            }.padding(12.dp),
                        contentAlignment = Alignment.Center,
                    ) { Text(stringResource(R.string.btn_apply), color = TextMain, fontWeight = FontWeight.SemiBold) }
                }
            }
            item { SmallButton(stringResource(R.string.btn_rescan)) { Scanner.start(ctx) } }
        }
        if (scan.blocked.isNotEmpty() && !scan.running) {
            item { Text(stringResource(R.string.tg_blocked, scan.blocked.joinToString(", ")), color = Muted, fontSize = 12.sp) }
        }
        items(scan.combos.take(if (scan.running) 40 else 12)) { c -> ComboRow(c) }
        scan.error?.let { item { Text(it, color = Danger, fontSize = 13.sp) } }
        item { Spacer(Modifier.height(8.dp)) }
    }
}

@Composable
private fun ComboRow(c: Combo) {
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(8.dp)).background(Panel).padding(horizontal = 12.dp, vertical = 9.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text("${c.strategy} · ${c.dns}", color = TextMain, fontFamily = FontFamily.Monospace, fontSize = 12.sp, modifier = Modifier.weight(1f))
        val (bg, fg, txt) = when {
            c.passed == c.total && c.total > 0 -> Triple(Accent, Bg, "✓ ${c.avgMillis} ms")
            c.passed > 0 -> Triple(AccentMid, TextMain, "${c.passed}/${c.total}")
            else -> Triple(Line, Muted, "×")
        }
        Box(Modifier.clip(RoundedCornerShape(5.dp)).background(bg).padding(horizontal = 8.dp, vertical = 3.dp)) {
            Text(txt, color = fg, fontFamily = FontFamily.Monospace, fontSize = 11.sp)
        }
    }
}

// ---------------------------------------------------------------- profiles

@Composable
private fun ProfilesScreen(refresh: Int, onChanged: () -> Unit) {
    val ctx = LocalContext.current
    val store = remember { Store(ctx) }
    val all = remember(refresh) { store.all() }
    var region by remember { mutableStateOf("*") }
    var selected by remember(refresh) { mutableStateOf(store.selectedId) }
    val regions = listOf("*") + all.map { it.region }.distinct()

    LazyColumn(Modifier.fillMaxSize().padding(horizontal = 20.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        item { Spacer(Modifier.height(12.dp)) }
        item {
            Text(stringResource(R.string.pf_title), color = TextMain, fontSize = 24.sp, fontWeight = FontWeight.SemiBold, lineHeight = 29.sp)
        }
        item {
            LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                items(regions) { r ->
                    val sel = r == region
                    Box(
                        Modifier.clip(RoundedCornerShape(16.dp)).background(if (sel) TextMain else Color.Transparent)
                            .border(1.dp, if (sel) TextMain else Line, RoundedCornerShape(16.dp))
                            .clickable { region = r }.padding(horizontal = 14.dp, vertical = 8.dp),
                    ) { Text(if (r == "*") stringResource(R.string.pf_all) else regionName(ctx, r), color = if (sel) Bg else TextMain, fontSize = 13.sp) }
                }
            }
        }
        items(all.filter { region == "*" || it.region == region }) { p ->
            val sel = p.id == selected
            Column(
                Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(Panel)
                    .border(if (sel) 1.5.dp else 1.dp, if (sel) Accent else Line, RoundedCornerShape(12.dp))
                    .clickable {
                        store.selectedId = p.id
                        selected = p.id
                        onChanged()
                        // Apply right away when protection is on.
                        DikecVpnService.restart(ctx)
                    }.padding(14.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(regionName(ctx, p.region), color = Muted, fontSize = 11.sp, modifier = Modifier.weight(1f))
                    val (bg, fg, t) = if (p.verified) Triple(Accent, Bg, stringResource(R.string.badge_verified))
                    else Triple(Line, TextMain, stringResource(R.string.badge_starter))
                    Box(Modifier.clip(RoundedCornerShape(6.dp)).background(bg).padding(horizontal = 6.dp, vertical = 2.dp)) {
                        Text(t, color = fg, fontSize = 10.sp, fontFamily = FontFamily.Monospace)
                    }
                }
                Text(displayName(ctx, p), color = TextMain, fontSize = 16.sp, fontWeight = FontWeight.SemiBold)
                Text(p.label(stringResource(R.string.dns_system)), color = Accent, fontFamily = FontFamily.Monospace, fontSize = 11.sp)
                Text(description(ctx, p), color = Muted, fontSize = 12.sp, lineHeight = 17.sp)
            }
        }
        item { Spacer(Modifier.height(8.dp)) }
    }
}

private fun regionName(ctx: android.content.Context, r: String): String {
    val key = "region_" + r.lowercase().map {
        when (it) { 'ü' -> 'u'; 'ğ' -> 'g'; 'ı' -> 'i'; 'ö' -> 'o'; 'ş' -> 's'; 'ç' -> 'c'; else -> if (it.isLetterOrDigit()) it else '_' }
    }.joinToString("")
    val id = ctx.resources.getIdentifier(key, "string", ctx.packageName)
    return if (id != 0) ctx.getString(id) else r
}

private fun description(ctx: android.content.Context, p: Profile): String {
    val lang = ctx.resources.configuration.locales[0].language
    if (lang == "tr" && p.id != "auto") return p.description
    val id = ctx.resources.getIdentifier("pd_" + p.id.replace("-", "_"), "string", ctx.packageName)
    if (p.id == "auto") return ""
    return if (id != 0) ctx.getString(id) else p.description
}

// ---------------------------------------------------------------- about

@Composable
private fun AboutScreen(onOpenReleases: () -> Unit) {
    val vpn by DikecVpnService.state.collectAsState()
    LazyColumn(Modifier.fillMaxSize().padding(horizontal = 20.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        item { Spacer(Modifier.height(12.dp)) }
        item {
            Card {
                Text(stringResource(R.string.st_about), color = TextMain, fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
                Text("Dikec DPI ${Mobile.version()} · Apache 2.0", color = Muted, fontFamily = FontFamily.Monospace, fontSize = 12.sp)
                Text(stringResource(R.string.st_credits, "xvzc/SpoofDPI", "mertikavi/SpoofDPI-Turkiye"), color = Muted, fontSize = 13.sp, lineHeight = 19.sp)
                Text(stringResource(R.string.android_language), color = Muted, fontSize = 13.sp)
                Spacer(Modifier.height(6.dp))
                SmallButton(stringResource(R.string.android_openreleases), onOpenReleases)
            }
        }
        item { Text(stringResource(R.string.log_title), color = TextMain, fontSize = 16.sp, fontWeight = FontWeight.SemiBold) }
        items(vpn.log.takeLast(120).reversed()) { line ->
            Text(line, color = Muted, fontFamily = FontFamily.Monospace, fontSize = 10.5.sp, lineHeight = 14.sp)
        }
        if (vpn.log.isEmpty()) item { Text(stringResource(R.string.log_empty), color = Muted, fontSize = 13.sp) }
    }
}

// ---------------------------------------------------------------- bits

@Composable
private fun Card(modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    Column(
        modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(Panel)
            .border(1.dp, Color(0xFF1E2129), RoundedCornerShape(14.dp)).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) { content() }
}

@Composable
private fun PrimaryButton(text: String, filled: Boolean, enabled: Boolean = true, onClick: () -> Unit) {
    Box(
        Modifier.fillMaxWidth().height(52.dp).clip(RoundedCornerShape(12.dp))
            .background(if (!enabled) Panel2 else if (filled) Accent else Panel2)
            .clickable(enabled = enabled, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Text(text, color = if (!enabled) Accent else if (filled) Bg else TextMain, fontSize = 16.sp, fontWeight = FontWeight.SemiBold)
    }
}

@Composable
private fun SmallButton(text: String, onClick: () -> Unit) {
    Box(
        Modifier.clip(RoundedCornerShape(10.dp)).border(1.dp, Line, RoundedCornerShape(10.dp))
            .clickable(onClick = onClick).padding(horizontal = 14.dp, vertical = 10.dp),
    ) { Text(text, color = TextMain, fontSize = 13.sp, fontWeight = FontWeight.Medium) }
}
