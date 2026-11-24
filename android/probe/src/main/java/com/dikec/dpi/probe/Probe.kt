package com.dikec.dpi.probe

import android.app.Activity
import android.os.Bundle
import android.util.Log
import java.net.HttpURLConnection
import java.net.URL

/** adb shell am start -n com.dikec.dpi.probe/.Probe --es urls "https://a,https://b" */
class Probe : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val urls = intent.getStringExtra("urls")?.split(",") ?: emptyList()
        Thread {
            for (u in urls) {
                val t = System.currentTimeMillis()
                val line = try {
                    val c = URL(u).openConnection() as HttpURLConnection
                    c.connectTimeout = 10000
                    c.readTimeout = 10000
                    c.instanceFollowRedirects = false
                    val code = c.responseCode
                    c.disconnect()
                    "OK $code ${System.currentTimeMillis() - t}ms $u"
                } catch (e: Exception) {
                    "FAIL ${e.javaClass.simpleName}: ${e.message} $u"
                }
                Log.i("DIKEC_PROBE", line)
            }
            Log.i("DIKEC_PROBE", "DONE")
            finish()
        }.start()
    }
}
