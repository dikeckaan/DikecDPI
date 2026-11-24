package com.dikec.dpi

import android.content.Context
import com.dikec.dpi.core.mobile.Mobile
import com.dikec.dpi.core.mobile.ScanListener
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import org.json.JSONArray
import org.json.JSONObject

data class Combo(val strategy: String, val dns: String, val passed: Int, val total: Int, val avgMillis: Long)

data class ScanState(
    val running: Boolean = false,
    val stage: String = "",
    val done: Int = 0,
    val total: Int = 0,
    val message: String = "",
    val combos: List<Combo> = emptyList(),
    val best: Profile? = null,
    val blocked: List<String> = emptyList(),
    val error: String? = null,
)

/** Auto Bypass, run by the Go core; results are kept for the UI. */
object Scanner {
    private val _state = MutableStateFlow(ScanState())
    val state: StateFlow<ScanState> = _state.asStateFlow()

    fun load(context: Context) {
        val last = Store(context).lastScan ?: return
        if (!_state.value.running) _state.value = parseReport(last, context)
    }

    fun start(context: Context) {
        if (_state.value.running) return
        val app = context.applicationContext
        val store = Store(app)
        _state.value = ScanState(running = true, stage = "baseline")
        Mobile.scan(JSONArray(store.targets).toString(), object : ScanListener {
            override fun progress(json: String) {
                val p = JSONObject(json)
                val combo = p.optJSONObject("combo")?.let { toCombo(it, app) }
                _state.update { s ->
                    s.copy(
                        stage = p.optString("stage"),
                        done = p.optInt("done"),
                        total = p.optInt("total"),
                        message = combo?.let { "${it.strategy} · ${it.dns}" } ?: p.optString("message"),
                        combos = if (combo != null) s.combos + combo else s.combos,
                    )
                }
            }

            override fun done(json: String) {
                val out = JSONObject(json)
                if (BuildConfig.DEBUG) {
                    val best = out.optJSONObject("report")?.optJSONObject("best")?.let { Profile(it) }
                    android.util.Log.i("DIKEC_SCAN", "done best=${best?.label("system")} error=${out.optString("error")}")
                }
                val err = out.optString("error").ifBlank { null }
                val report = out.optJSONObject("report")
                if (report != null) {
                    store.lastScan = report
                    report.optJSONObject("best")?.let { store.autoProfile = Profile(it) }
                    _state.value = parseReport(report, app).copy(error = err)
                } else {
                    _state.update { it.copy(running = false, error = err) }
                }
            }
        })
    }

    fun cancel() = Mobile.cancelScan()

    private fun toCombo(c: JSONObject, context: Context): Combo {
        val p = Profile(c.getJSONObject("profile"))
        return Combo(
            p.strategy(), p.dns(context.getString(R.string.dns_system)),
            c.optInt("passed"), c.optInt("total"), c.optLong("avgMillis"),
        )
    }

    private fun parseReport(r: JSONObject, context: Context): ScanState {
        val results = r.optJSONArray("results") ?: JSONArray()
        val combos = (0 until results.length()).map { toCombo(results.getJSONObject(it), context) }
        val blocked = r.optJSONArray("blocked")?.let { a -> (0 until a.length()).map { a.getString(it) } } ?: emptyList()
        return ScanState(
            running = false, stage = "done", combos = combos,
            best = r.optJSONObject("best")?.let { Profile(it) }, blocked = blocked,
        )
    }
}
