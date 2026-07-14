package com.misetanibox.app.network

import com.misetanibox.app.vpn.MihomoVpnService
import com.misetanibox.app.vpn.VpnPrefs
import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.net.wifi.WifiManager
import android.os.Build
import android.os.Handler
import android.os.Looper

/**
 * Auto connect/disconnect VPN based on network type and trusted Wi‑Fi SSIDs.
 * Controlled via VpnPrefs keys set from the Web UI.
 */
object NetworkAuto {
    private var registered = false
    private var callback: ConnectivityManager.NetworkCallback? = null
    private val handler = Handler(Looper.getMainLooper())
    private var pending: Runnable? = null

    const val KEY_NET_AUTO = "net_auto"
    const val KEY_TRUSTED_SSIDS = "trusted_ssids"
    /** off | direct | disconnect */
    const val KEY_HOME_ACTION = "home_action"
    /** connect when mobile / untrusted wifi */
    const val KEY_AWAY_CONNECT = "away_connect"

    fun isEnabled(ctx: Context): Boolean =
        VpnPrefs.prefs(ctx).getBoolean(KEY_NET_AUTO, false)

    fun ensureRegistered(ctx: Context) {
        if (!isEnabled(ctx)) {
            unregister(ctx)
            return
        }
        if (registered) return
        val app = ctx.applicationContext
        val cm = app.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val cb = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) = scheduleEvaluate(app)
            override fun onLost(network: Network) = scheduleEvaluate(app)
            override fun onCapabilitiesChanged(network: Network, caps: NetworkCapabilities) =
                scheduleEvaluate(app)
        }
        callback = cb
        val req = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .build()
        try {
            cm.registerNetworkCallback(req, cb)
            registered = true
            scheduleEvaluate(app)
        } catch (_: Exception) {
            registered = false
        }
    }

    fun unregister(ctx: Context) {
        if (!registered) return
        try {
            val cm = ctx.applicationContext.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
            callback?.let { cm.unregisterNetworkCallback(it) }
        } catch (_: Exception) {
        }
        callback = null
        registered = false
        pending?.let { handler.removeCallbacks(it) }
        pending = null
    }

    private fun scheduleEvaluate(ctx: Context) {
        pending?.let { handler.removeCallbacks(it) }
        val r = Runnable { evaluate(ctx) }
        pending = r
        handler.postDelayed(r, 900)
    }

    private fun evaluate(ctx: Context) {
        if (!isEnabled(ctx)) return
        if (!VpnPrefs.hasSubscription(ctx)) return
        val cm = ctx.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val net = cm.activeNetwork ?: run {
            // no network — leave VPN as-is
            return
        }
        val caps = cm.getNetworkCapabilities(net) ?: return
        val isWifi = caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI)
        val isCell = caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR)
        val ssid = if (isWifi) currentSsid(ctx) else ""
        val trusted = trustedSsids(ctx)
        val home = isWifi && ssid.isNotBlank() && trusted.any { it.equals(ssid, ignoreCase = true) }
        val awayConnect = VpnPrefs.prefs(ctx).getBoolean(KEY_AWAY_CONNECT, true)
        val homeAction = VpnPrefs.prefs(ctx).getString(KEY_HOME_ACTION, "disconnect") ?: "disconnect"

        if (home) {
            when (homeAction) {
                "disconnect" -> if (MihomoVpnService.isRunning) VpnPrefs.stopVpn(ctx)
                // "direct" would need live mode patch — stop is safer for home
                else -> if (MihomoVpnService.isRunning) VpnPrefs.stopVpn(ctx)
            }
            return
        }

        if (awayConnect && (isCell || isWifi)) {
            if (!MihomoVpnService.isRunning) {
                VpnPrefs.startFromPrefs(ctx)
            }
        }
    }

    private fun trustedSsids(ctx: Context): List<String> {
        val raw = VpnPrefs.prefs(ctx).getString(KEY_TRUSTED_SSIDS, "") ?: ""
        return raw.split(',', '\n', ';')
            .map { it.trim().trim('"') }
            .filter { it.isNotEmpty() }
    }

    @Suppress("DEPRECATION")
    private fun currentSsid(ctx: Context): String {
        return try {
            val wm = ctx.applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
            val info = wm.connectionInfo ?: return ""
            var ssid = info.ssid ?: return ""
            if (ssid == "<unknown ssid>") return ""
            if (ssid.startsWith("\"") && ssid.endsWith("\"") && ssid.length >= 2) {
                ssid = ssid.substring(1, ssid.length - 1)
            }
            ssid
        } catch (_: Exception) {
            ""
        }
    }
}
