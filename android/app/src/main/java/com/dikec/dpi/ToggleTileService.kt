package com.dikec.dpi

import android.content.Intent
import android.net.VpnService
import android.service.quicksettings.Tile
import android.service.quicksettings.TileService

/** Quick Settings tile: one tap to turn protection on or off. */
class ToggleTileService : TileService() {
    override fun onStartListening() = update()

    override fun onClick() {
        val running = DikecVpnService.state.value.phase.let { it == Phase.On || it == Phase.Starting }
        if (running) {
            DikecVpnService.stop(this)
        } else if (VpnService.prepare(this) == null) {
            DikecVpnService.start(this)
        } else {
            // First use needs Android's VPN consent dialog: open the app.
            val i = Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            if (android.os.Build.VERSION.SDK_INT >= 34) {
                startActivityAndCollapse(
                    android.app.PendingIntent.getActivity(this, 0, i, android.app.PendingIntent.FLAG_IMMUTABLE)
                )
            } else {
                @Suppress("DEPRECATION")
                startActivityAndCollapse(i)
            }
        }
        update()
    }

    private fun update() {
        val t = qsTile ?: return
        val on = DikecVpnService.state.value.phase == Phase.On
        t.state = if (on) Tile.STATE_ACTIVE else Tile.STATE_INACTIVE
        t.updateTile()
    }
}
