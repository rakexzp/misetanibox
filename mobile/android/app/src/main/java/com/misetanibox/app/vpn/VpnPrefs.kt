package com.misetanibox.app.vpn
import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.os.Build
import org.json.JSONArray

/** Shared prefs + helpers for BootReceiver / Quick Settings tile / split / kill switch. */
object VpnPrefs {
    const val PREFS = "misetanibox"
    const val KEY_AUTOSTART = "autostart"
    const val KEY_SUB_URL = "sub_url"
    const val KEY_SUBS_JSON = "subs_json"
    const val KEY_HWID = "hwid"
    const val KEY_MODE = "mode"
    const val KEY_KILL_SWITCH = "kill_switch"
    /** all | include | exclude */
    const val KEY_SPLIT_MODE = "split_mode"
    /** JSON array of package names */
    const val KEY_SPLIT_APPS = "split_apps"

    fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun normalizeMode(raw: String?): String {
        return when (raw?.lowercase()) {
            "global" -> "global"
            "direct" -> "direct"
            else -> "rule"
        }
    }

    fun modeLabel(mode: String): String {
        return when (normalizeMode(mode)) {
            "global" -> "глобально"
            "direct" -> "direct"
            else -> "правила"
        }
    }

    fun hasSubscription(ctx: Context): Boolean {
        val p = prefs(ctx)
        val json = p.getString(KEY_SUBS_JSON, "") ?: ""
        if (json.contains("http") || json.contains("local:") || json.contains("\"body\"")) return true
        val url = p.getString(KEY_SUB_URL, "") ?: ""
        return url.isNotBlank()
    }

    fun isKillSwitch(ctx: Context): Boolean =
        prefs(ctx).getBoolean(KEY_KILL_SWITCH, false)

    fun splitMode(ctx: Context): String {
        val m = prefs(ctx).getString(KEY_SPLIT_MODE, "all") ?: "all"
        return when (m) {
            "include", "exclude" -> m
            else -> "all"
        }
    }

    fun splitApps(ctx: Context): List<String> {
        val raw = prefs(ctx).getString(KEY_SPLIT_APPS, "[]") ?: "[]"
        return try {
            val arr = JSONArray(raw)
            val out = ArrayList<String>()
            for (i in 0 until arr.length()) {
                val p = arr.optString(i, "").trim()
                if (p.isNotEmpty()) out.add(p)
            }
            out
        } catch (_: Exception) {
            emptyList()
        }
    }

    fun saveLaunchState(
        ctx: Context,
        subUrl: String,
        subsJson: String,
        hwid: String,
        mode: String,
    ) {
        prefs(ctx).edit()
            .putString(KEY_SUB_URL, subUrl)
            .putString(KEY_SUBS_JSON, if (subsJson.isNotBlank()) subsJson else "[]")
            .putString(KEY_HWID, hwid)
            .putString(KEY_MODE, normalizeMode(mode))
            .apply()
    }

    /**
     * Start VPN from saved prefs.
     * @return null if started, or reason string if cannot start
     */
    fun startFromPrefs(ctx: Context): String? {
        if (!hasSubscription(ctx)) return "нет подписки"
        if (VpnService.prepare(ctx) != null) return "нужно разрешение VPN"
        val p = prefs(ctx)
        val subUrl = p.getString(KEY_SUB_URL, "") ?: ""
        var subsJson = p.getString(KEY_SUBS_JSON, "[]") ?: "[]"
        if (!subsJson.contains("http") && !subsJson.contains("local:") && subUrl.isNotBlank()) {
            subsJson = JSONArray().put(subUrl).toString()
        }
        val hwid = p.getString(KEY_HWID, "") ?: ""
        val mode = normalizeMode(p.getString(KEY_MODE, "rule"))
        val i = Intent(ctx, MihomoVpnService::class.java).apply {
            action = MihomoVpnService.ACTION_START
            putExtra(MihomoVpnService.EXTRA_SUB_URL, subUrl)
            putExtra(MihomoVpnService.EXTRA_SUB_URLS_JSON, subsJson)
            putExtra(MihomoVpnService.EXTRA_HWID, hwid)
            putExtra(MihomoVpnService.EXTRA_MODE, mode)
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            ctx.startForegroundService(i)
        } else {
            ctx.startService(i)
        }
        return null
    }

    fun stopVpn(ctx: Context) {
        val i = Intent(ctx, MihomoVpnService::class.java).apply {
            action = MihomoVpnService.ACTION_STOP
        }
        ctx.startService(i)
    }
}
