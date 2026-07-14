package com.misetanibox.app.boot

import com.misetanibox.app.vpn.VpnPrefs
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

/**
 * Optional auto-start after reboot when the user enabled it in settings.
 */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        val action = intent?.action ?: return
        if (action != Intent.ACTION_BOOT_COMPLETED && action != Intent.ACTION_MY_PACKAGE_REPLACED) {
            return
        }
        val prefs = VpnPrefs.prefs(context)
        if (!prefs.getBoolean(VpnPrefs.KEY_AUTOSTART, false)) return
        VpnPrefs.startFromPrefs(context)
    }
}
