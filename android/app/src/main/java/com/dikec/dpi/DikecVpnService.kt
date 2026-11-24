package com.dikec.dpi

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.service.quicksettings.TileService
import android.content.ComponentName
import com.dikec.dpi.core.mobile.Logger
import com.dikec.dpi.core.mobile.Mobile
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

enum class Phase { Off, Starting, On, Stopping }

data class VpnState(
    val phase: Phase = Phase.Off,
    val since: Long = 0,
    val profileName: String = "",
    val error: String? = null,
    val log: List<String> = emptyList(),
)

/**
 * Routes the phone's traffic into the Go core through Android's VPN API.
 * Nothing leaves the device through another server: the core relays every
 * connection itself and only reshapes the TLS ClientHello. The app excludes
 * itself from the VPN, so the core's own sockets go straight out.
 */
class DikecVpnService : VpnService() {

    companion object {
        const val ACTION_START = "com.dikec.dpi.START"
        const val ACTION_STOP = "com.dikec.dpi.STOP"
        private const val CHANNEL = "status"
        private const val MTU = 1500

        private val _state = MutableStateFlow(VpnState())
        val state: StateFlow<VpnState> = _state.asStateFlow()

        fun start(context: Context) {
            val i = Intent(context, DikecVpnService::class.java).setAction(ACTION_START)
            if (Build.VERSION.SDK_INT >= 26) context.startForegroundService(i) else context.startService(i)
        }

        fun stop(context: Context) {
            context.startService(Intent(context, DikecVpnService::class.java).setAction(ACTION_STOP))
        }

        @Volatile private var restartAfterStop = false

        /** Applies a new profile: stop, then start again with it. */
        fun restart(context: Context) {
            if (_state.value.phase != Phase.On) return
            restartAfterStop = true
            stop(context)
        }

        internal fun log(line: String) {
            android.util.Log.i("DIKEC_CORE", line)
            _state.update { it.copy(log = (it.log + line).takeLast(400)) }
        }
    }

    private var tun: ParcelFileDescriptor? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            stopVpn()
            return START_NOT_STICKY
        }
        startVpn()
        return START_STICKY
    }

    private fun startVpn() {
        if (Mobile.isRunning() || _state.value.phase == Phase.Starting) return
        val store = Store(this)
        val profile = store.selected()
        val label = displayName(this, profile)
        _state.update { it.copy(phase = Phase.Starting, error = null, profileName = label) }
        startInForeground(label)

        Thread {
            try {
                val builder = Builder()
                    .setSession("Dikec DPI")
                    .setMtu(MTU)
                    .addAddress("10.111.222.1", 32)
                    .addRoute("0.0.0.0", 0)
                    .addAddress("fd66:6466:7069::1", 128)
                    .addRoute("::", 0)
                    // DNS sent here is answered by the core with the
                    // profile's resolver (e.g. Yandex :1253 or DoH).
                    .addDnsServer("1.1.1.1")
                    .addDisallowedApplication(packageName)
                    .setBlocking(true)
                if (Build.VERSION.SDK_INT >= 29) builder.setMetered(false)
                val pfd = builder.establish() ?: error(getString(R.string.android_consentdenied))
                tun = pfd
                Mobile.start(pfd.detachFd().toLong(), MTU.toLong(), profile.json.toString(), object : Logger {
                    override fun log(level: String, message: String) = Companion.log("[$level] $message")
                })
                _state.update { it.copy(phase = Phase.On, since = System.currentTimeMillis()) }
            } catch (e: Throwable) {
                _state.update { it.copy(phase = Phase.Off, error = e.message ?: e.toString()) }
                cleanup()
            }
            refreshTile()
        }.start()
    }

    private fun stopVpn() {
        if (_state.value.phase == Phase.Off && !Mobile.isRunning()) {
            cleanup()
            return
        }
        _state.update { it.copy(phase = Phase.Stopping) }
        Thread {
            Mobile.stop() // also closes the TUN device (we handed the fd over)
            cleanup()
            _state.update { it.copy(phase = Phase.Off, since = 0) }
            refreshTile()
            if (restartAfterStop) {
                restartAfterStop = false
                start(applicationContext)
            }
        }.start()
    }

    private fun cleanup() {
        try { tun?.close() } catch (_: Exception) {}
        tun = null
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onRevoke() {
        // Another VPN took over, or the user revoked it in Settings.
        _state.update { it.copy(error = getString(R.string.android_othervpn)) }
        stopVpn()
    }

    override fun onDestroy() {
        if (Mobile.isRunning()) Mobile.stop()
        _state.update { it.copy(phase = Phase.Off, since = 0) }
        super.onDestroy()
    }

    private fun startInForeground(profileLabel: String) {
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(CHANNEL, getString(R.string.android_notifchannel), NotificationManager.IMPORTANCE_LOW)
        )
        val open = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE
        )
        val stop = PendingIntent.getService(
            this, 1, Intent(this, DikecVpnService::class.java).setAction(ACTION_STOP), PendingIntent.FLAG_IMMUTABLE
        )
        val n: Notification = Notification.Builder(this, CHANNEL)
            .setSmallIcon(R.drawable.ic_mark)
            .setContentTitle(getString(R.string.android_notiftitle))
            .setContentText(getString(R.string.android_notiftext, profileLabel))
            .setContentIntent(open)
            .setOngoing(true)
            .addAction(Notification.Action.Builder(null, getString(R.string.android_stopaction), stop).build())
            .build()
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(1, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else {
            startForeground(1, n)
        }
    }

    private fun refreshTile() {
        TileService.requestListeningState(this, ComponentName(this, ToggleTileService::class.java))
    }
}

/** Localized profile name (Go presets carry Turkish names). */
fun displayName(context: Context, p: Profile): String {
    val id = context.resources.getIdentifier("p_" + p.id.replace("-", "_"), "string", context.packageName)
    return if (id != 0) context.getString(id) else p.name
}
