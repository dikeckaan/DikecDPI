package com.dikec.dpi

import android.content.Context
import com.dikec.dpi.core.mobile.Mobile
import org.json.JSONArray
import org.json.JSONObject

/** One bypass setting (same JSON shape as the Go engine.Profile). */
data class Profile(val json: JSONObject) {
    val id: String get() = json.optString("id")
    val name: String get() = json.optString("name")
    val region: String get() = json.optString("region")
    val description: String get() = json.optString("description")
    val verified: Boolean get() = json.optBoolean("verified")
    val needsRoot: Boolean get() = json.optInt("fakeCount") > 0

    /** "tlsrec · Yandex" style summary, mirroring the desktop app. */
    fun label(systemName: String): String = "${strategy()} · ${dns(systemName)}"

    fun strategy(): String {
        var s = json.optString("splitMode")
        if (s == "chunk") s = "chunk/${json.optInt("chunkSize")}"
        if (json.optBoolean("disorder")) s += "+disorder"
        if (json.optInt("fakeCount") > 0) s += "+fake×${json.optInt("fakeCount")}"
        return s
    }

    fun dns(systemName: String): String {
        val doh = json.optString("dohUrl")
        val addr = json.optString("dnsAddr")
        return when (json.optString("dnsMode")) {
            "https" -> when {
                "1.1.1.1" in doh || "cloudflare" in doh -> "CF DoH"
                "8.8.8.8" in doh || "dns.google" in doh -> "Google DoH"
                "9.9.9.9" in doh || "quad9" in doh -> "Quad9 DoH"
                else -> "DoH"
            }
            "system" -> systemName
            else -> when {
                addr.startsWith("77.88.8.8") -> "Yandex"
                addr.startsWith("1.1.1.1") -> "CF UDP"
                else -> "UDP $addr"
            }
        }
    }
}

/** Persists the selected profile and the auto-bypass winner. */
class Store(context: Context) {
    private val prefs = context.getSharedPreferences("dikec", Context.MODE_PRIVATE)

    fun presets(): List<Profile> {
        val arr = JSONArray(Mobile.presets())
        // Fake-packet profiles need root/raw sockets, which Android apps lack.
        return (0 until arr.length()).map { Profile(arr.getJSONObject(it)) }.filterNot { it.needsRoot }
    }

    var autoProfile: Profile?
        get() = prefs.getString("auto", null)?.let { Profile(JSONObject(it)) }
        set(v) { prefs.edit().putString("auto", v?.json?.toString()).apply() }

    var lastScan: JSONObject?
        get() = prefs.getString("lastScan", null)?.let { JSONObject(it) }
        set(v) { prefs.edit().putString("lastScan", v?.toString()).apply() }

    var selectedId: String
        get() = prefs.getString("selected", "tr") ?: "tr"
        set(v) { prefs.edit().putString("selected", v).apply() }

    var targets: List<String>
        get() = prefs.getString("targets", null)?.split(",")?.filter { it.isNotBlank() }
            ?: JSONArray(Mobile.defaultTargets()).let { a -> (0 until a.length()).map { a.getString(it) } }
        set(v) { prefs.edit().putString("targets", v.joinToString(",")).apply() }

    fun all(): List<Profile> = listOfNotNull(autoProfile) + presets()

    fun selected(): Profile = all().firstOrNull { it.id == selectedId } ?: presets().first()
}
