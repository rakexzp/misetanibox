package com.misetanibox.app.plugin

import com.misetanibox.app.security.AppLock
import com.misetanibox.app.network.NetworkAuto
import com.misetanibox.app.ui.QrScanActivity
import com.misetanibox.app.vpn.MihomoVpnService
import com.misetanibox.app.vpn.PingCore
import com.misetanibox.app.vpn.SubBodies
import com.misetanibox.app.vpn.VpnPrefs
import android.app.Activity
import android.content.BroadcastReceiver
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.VpnService
import android.os.Build
import android.provider.Settings
import androidx.activity.result.ActivityResult
import com.getcapacitor.JSArray
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.ActivityCallback
import com.getcapacitor.annotation.CapacitorPlugin
import mobilecore.Mobilecore
import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.InetSocketAddress
import java.net.Proxy
import java.net.URL

@CapacitorPlugin(name = "Vpn")
class VpnPlugin : Plugin() {

    private var pendingSubUrl = ""
    private var pendingSubsJson = "[]"
    private var pendingHwid = ""
    private var pendingMode = "rule"
    private var receiver: BroadcastReceiver? = null
    private var pendingQrCall: PluginCall? = null

    override fun load() {
        val filter = IntentFilter("com.misetanibox.app.VPN_STATE")
        receiver = object : BroadcastReceiver() {
            override fun onReceive(c: Context?, i: Intent?) {
                val ret = JSObject()
                ret.put("state", i?.getStringExtra("state") ?: "")
                ret.put("message", i?.getStringExtra("message") ?: "")
                notifyListeners("vpnState", ret)
            }
        }
        if (Build.VERSION.SDK_INT >= 33) {
            context.registerReceiver(receiver, filter, Context.RECEIVER_NOT_EXPORTED)
        } else {
            @Suppress("UnspecifiedRegisterReceiverFlag")
            context.registerReceiver(receiver, filter)
        }
    }

    override fun handleOnDestroy() {
        receiver?.let {
            try {
                context.unregisterReceiver(it)
            } catch (_: Exception) {
            }
        }
        receiver = null
    }

    @PluginMethod
    fun start(call: PluginCall) {
        pendingSubUrl = call.getString("subUrl") ?: ""
        pendingSubsJson = call.getString("subsJson") ?: "[]"
        pendingHwid = call.getString("hwid") ?: ""
        pendingMode = VpnPrefs.normalizeMode(call.getString("mode"))
        val hasSubs = pendingSubsJson.contains("http")
            || pendingSubsJson.contains("local:")
            || pendingSubsJson.contains("\"body\"")
            || pendingSubUrl.isNotEmpty()
        if (!hasSubs) {
            call.reject("нет подписки")
            return
        }
        val prepare = VpnService.prepare(context)
        if (prepare != null) {
            startActivityForResult(call, prepare, "vpnPermCallback")
        } else {
            launchService()
            call.resolve()
        }
    }

    @ActivityCallback
    private fun vpnPermCallback(call: PluginCall, result: ActivityResult) {
        if (result.resultCode == Activity.RESULT_OK) {
            launchService()
            call.resolve()
        } else {
            call.reject("пользователь отклонил разрешение VPN")
        }
    }

    private fun launchService() {
        val lightJson = stripBodiesFromSubsJson(pendingSubsJson)
        VpnPrefs.saveLaunchState(
            context,
            pendingSubUrl,
            lightJson,
            pendingHwid,
            pendingMode,
        )
        val i = Intent(context, MihomoVpnService::class.java).apply {
            action = MihomoVpnService.ACTION_START
            putExtra(MihomoVpnService.EXTRA_SUB_URL, pendingSubUrl)
            putExtra(MihomoVpnService.EXTRA_SUB_URLS_JSON, lightJson)
            putExtra(MihomoVpnService.EXTRA_HWID, pendingHwid)
            putExtra(MihomoVpnService.EXTRA_MODE, pendingMode)
        }
        context.startForegroundService(i)
    }

    /** Drop inline bodies; service loads them from SubBodies files. */
    private fun stripBodiesFromSubsJson(raw: String): String {
        if (raw.isBlank()) return "[]"
        return try {
            val arr = JSONArray(raw)
            val out = JSONArray()
            for (i in 0 until arr.length()) {
                val el = arr.opt(i) ?: continue
                if (el is JSONObject) {
                    val o = JSONObject()
                    o.put("url", el.optString("url", ""))
                    o.put("label", el.optString("label", ""))
                    if (el.has("kind")) o.put("kind", el.optString("kind"))
                    val body = el.optString("body", "")
                    val url = o.optString("url")
                    val id = SubBodies.idFromUrl(url)
                    if (body.isNotBlank() && id != null) {
                        SubBodies.put(context, id, body)
                    }
                    out.put(o)
                } else {
                    out.put(el.toString())
                }
            }
            out.toString()
        } catch (_: Exception) {
            raw
        }
    }

    @PluginMethod
    fun putSubBody(call: PluginCall) {
        val id = call.getString("id")?.trim().orEmpty()
        val body = call.getString("body") ?: ""
        if (id.isEmpty()) {
            call.reject("id required")
            return
        }
        try {
            SubBodies.put(context, id, body)
            val ret = JSObject()
            ret.put("ok", true)
            ret.put("bytes", body.length)
            call.resolve(ret)
        } catch (e: Exception) {
            call.reject(e.message ?: "putSubBody failed")
        }
    }

    @PluginMethod
    fun getSubBody(call: PluginCall) {
        val id = call.getString("id")?.trim().orEmpty()
        if (id.isEmpty()) {
            call.reject("id required")
            return
        }
        val ret = JSObject()
        ret.put("body", SubBodies.get(context, id))
        call.resolve(ret)
    }

    @PluginMethod
    fun deleteSubBody(call: PluginCall) {
        val id = call.getString("id")?.trim().orEmpty()
        if (id.isNotEmpty()) SubBodies.delete(context, id)
        call.resolve()
    }

    @PluginMethod
    fun stop(call: PluginCall) {
        val i = Intent(context, MihomoVpnService::class.java).apply {
            action = MihomoVpnService.ACTION_STOP
        }
        context.startService(i)
        call.resolve()
    }

    @PluginMethod
    fun status(call: PluginCall) {
        val ret = JSObject()
        ret.put("running", MihomoVpnService.isRunning)
        ret.put("mode", MihomoVpnService.currentMode)
        ret.put("downRate", MihomoVpnService.lastDownRate)
        ret.put("upRate", MihomoVpnService.lastUpRate)
        try {
            ret.put("tun", Mobilecore.tunStatus())
        } catch (_: Exception) {
            ret.put("tun", "")
        }
        call.resolve(ret)
    }

    @PluginMethod
    fun getDeviceInfo(call: PluginCall) {
        val ret = JSObject()
        ret.put("model", Build.MODEL)
        ret.put("manufacturer", Build.MANUFACTURER)
        ret.put("androidRelease", Build.VERSION.RELEASE)
        ret.put("sdkInt", Build.VERSION.SDK_INT)
        call.resolve(ret)
    }

    /** Persist settings used by BootReceiver / QS tile / split / kill switch. */
    @PluginMethod
    fun setNativePrefs(call: PluginCall) {
        val prefs = VpnPrefs.prefs(context).edit()
        if (call.data.has("autostart")) {
            prefs.putBoolean(VpnPrefs.KEY_AUTOSTART, call.getBoolean("autostart", false) ?: false)
        }
        if (call.data.has("killSwitch")) {
            prefs.putBoolean(VpnPrefs.KEY_KILL_SWITCH, call.getBoolean("killSwitch", false) ?: false)
        }
        if (call.data.has("biometricLock")) {
            prefs.putBoolean(AppLock.KEY_BIOMETRIC, call.getBoolean("biometricLock", false) ?: false)
        }
        if (call.data.has("netAuto")) {
            prefs.putBoolean(NetworkAuto.KEY_NET_AUTO, call.getBoolean("netAuto", false) ?: false)
        }
        if (call.data.has("awayConnect")) {
            prefs.putBoolean(NetworkAuto.KEY_AWAY_CONNECT, call.getBoolean("awayConnect", true) ?: true)
        }
        call.getString("trustedSsids")?.let {
            prefs.putString(NetworkAuto.KEY_TRUSTED_SSIDS, it)
        }
        call.getString("homeAction")?.let {
            prefs.putString(NetworkAuto.KEY_HOME_ACTION, it)
        }
        call.getString("subUrl")?.let { prefs.putString(VpnPrefs.KEY_SUB_URL, it) }
        call.getString("subsJson")?.let {
            prefs.putString(VpnPrefs.KEY_SUBS_JSON, stripBodiesFromSubsJson(it))
        }
        call.getString("hwid")?.let { prefs.putString(VpnPrefs.KEY_HWID, it) }
        call.getString("mode")?.let {
            prefs.putString(VpnPrefs.KEY_MODE, VpnPrefs.normalizeMode(it))
        }
        call.getString("splitMode")?.let {
            val m = when (it.lowercase()) {
                "include", "exclude" -> it.lowercase()
                else -> "all"
            }
            prefs.putString(VpnPrefs.KEY_SPLIT_MODE, m)
        }
        call.getString("splitApps")?.let {
            prefs.putString(VpnPrefs.KEY_SPLIT_APPS, it)
        }
        prefs.apply()
        try {
            if (NetworkAuto.isEnabled(context)) NetworkAuto.ensureRegistered(context)
            else NetworkAuto.unregister(context)
        } catch (_: Exception) {
        }
        call.resolve()
    }

    @PluginMethod
    fun getNativePrefs(call: PluginCall) {
        val p = VpnPrefs.prefs(context)
        val ret = JSObject()
        ret.put("autostart", p.getBoolean(VpnPrefs.KEY_AUTOSTART, false))
        ret.put("killSwitch", p.getBoolean(VpnPrefs.KEY_KILL_SWITCH, false))
        ret.put("biometricLock", p.getBoolean(AppLock.KEY_BIOMETRIC, false))
        ret.put("netAuto", p.getBoolean(NetworkAuto.KEY_NET_AUTO, false))
        ret.put("awayConnect", p.getBoolean(NetworkAuto.KEY_AWAY_CONNECT, true))
        ret.put("trustedSsids", p.getString(NetworkAuto.KEY_TRUSTED_SSIDS, "") ?: "")
        ret.put("homeAction", p.getString(NetworkAuto.KEY_HOME_ACTION, "disconnect") ?: "disconnect")
        ret.put("mode", VpnPrefs.normalizeMode(p.getString(VpnPrefs.KEY_MODE, "rule")))
        ret.put("splitMode", VpnPrefs.splitMode(context))
        ret.put("splitApps", p.getString(VpnPrefs.KEY_SPLIT_APPS, "[]") ?: "[]")
        val pendingTab = p.getString("pending_open_tab", "") ?: ""
        if (pendingTab.isNotBlank()) {
            ret.put("pendingOpenTab", pendingTab)
            p.edit().remove("pending_open_tab").apply()
        }
        call.resolve(ret)
    }

    @PluginMethod
    fun biometricAvailable(call: PluginCall) {
        val ret = JSObject()
        ret.put("available", AppLock.canAuthenticate(context))
        call.resolve(ret)
    }

    @PluginMethod
    fun getClipboard(call: PluginCall) {
        try {
            val cm = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
            val clip = cm.primaryClip
            val text = if (clip != null && clip.itemCount > 0) {
                clip.getItemAt(0).coerceToText(context)?.toString() ?: ""
            } else {
                ""
            }
            val ret = JSObject()
            ret.put("text", text)
            call.resolve(ret)
        } catch (e: Exception) {
            call.reject(e.message ?: "clipboard failed")
        }
    }

    @PluginMethod
    fun setClipboard(call: PluginCall) {
        val text = call.getString("text") ?: ""
        try {
            val cm = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
            cm.setPrimaryClip(ClipData.newPlainText("misetanibox", text))
            call.resolve()
        } catch (e: Exception) {
            call.reject(e.message ?: "clipboard set failed")
        }
    }

    /** Built-in QR scanner (CameraX + ML Kit). */
    @PluginMethod
    fun scanQr(call: PluginCall) {
        val intent = Intent(context, QrScanActivity::class.java)
        startActivityForResult(call, intent, "qrScanCallback")
    }

    @ActivityCallback
    private fun qrScanCallback(call: PluginCall, result: ActivityResult) {
        pendingQrCall = null
        if (result.resultCode != Activity.RESULT_OK) {
            call.reject("cancelled")
            return
        }
        val contents = result.data?.getStringExtra(QrScanActivity.EXTRA_RESULT)?.trim().orEmpty()
        if (contents.isBlank()) {
            call.reject("empty")
            return
        }
        val ret = JSObject()
        ret.put("text", contents)
        call.resolve(ret)
    }

    /** Outbound IP via mixed-port when VPN is up (app itself is excluded from TUN). */
    @PluginMethod
    fun checkOutboundIp(call: PluginCall) {
        Thread {
            val ret = JSObject()
            try {
                val useProxy = MihomoVpnService.isRunning
                val urls = listOf(
                    "https://api.ipify.org?format=json",
                    "https://api.ip.sb/ip",
                    "https://ifconfig.me/ip",
                )
                var ip = ""
                var err = ""
                for (u in urls) {
                    try {
                        val conn = if (useProxy) {
                            val proxy = Proxy(Proxy.Type.HTTP, InetSocketAddress("127.0.0.1", 7890))
                            URL(u).openConnection(proxy) as HttpURLConnection
                        } else {
                            URL(u).openConnection() as HttpURLConnection
                        }
                        conn.connectTimeout = 8000
                        conn.readTimeout = 10000
                        conn.requestMethod = "GET"
                        conn.setRequestProperty("User-Agent", "Misetanibox/1.5")
                        val code = conn.responseCode
                        val text = (if (code in 200..299) conn.inputStream else conn.errorStream)
                            ?.bufferedReader()?.use { it.readText() }?.trim() ?: ""
                        conn.disconnect()
                        if (code !in 200..299 || text.isBlank()) {
                            err = "http $code"
                            continue
                        }
                        ip = if (text.startsWith("{")) {
                            JSONObject(text).optString("ip", text)
                        } else {
                            text.lines().firstOrNull()?.trim() ?: text
                        }
                        if (ip.matches(Regex("""^[\d.a-fA-F:]+$"""))) break
                        // ipify json ok
                        if (ip.isNotBlank() && !ip.contains(" ")) break
                        ip = ""
                    } catch (e: Exception) {
                        err = e.message ?: "fail"
                    }
                }
                if (ip.isBlank()) {
                    ret.put("ok", false)
                    ret.put("error", err.ifBlank { "не удалось получить IP" })
                } else {
                    ret.put("ok", true)
                    ret.put("ip", ip)
                    ret.put("viaProxy", useProxy)
                }
            } catch (e: Exception) {
                ret.put("ok", false)
                ret.put("error", e.message ?: "ip check failed")
            }
            call.resolve(ret)
        }.start()
    }

    /**
     * Full installed apps list for split tunneling.
     * Includes system apps (flagged). Requires QUERY_ALL_PACKAGES on Android 11+.
     */
    @PluginMethod
    fun listApps(call: PluginCall) {
        val includeSystem = call.getBoolean("includeSystem", true) ?: true
        Thread {
            try {
                val pm = context.packageManager
                val installed = if (Build.VERSION.SDK_INT >= 33) {
                    pm.getInstalledApplications(android.content.pm.PackageManager.ApplicationInfoFlags.of(0))
                } else {
                    @Suppress("DEPRECATION")
                    pm.getInstalledApplications(0)
                }
                data class AppRow(
                    val pkg: String,
                    val label: String,
                    val system: Boolean,
                    val uid: Int,
                )
                val rows = ArrayList<AppRow>()
                val self = context.packageName
                for (ai in installed) {
                    val pkg = ai.packageName ?: continue
                    if (pkg == self) continue
                    // Skip pure shared libraries / no process
                    if (ai.packageName.isNullOrBlank()) continue
                    val isSystem = (ai.flags and android.content.pm.ApplicationInfo.FLAG_SYSTEM) != 0 ||
                        (ai.flags and android.content.pm.ApplicationInfo.FLAG_UPDATED_SYSTEM_APP) != 0
                    if (!includeSystem && isSystem) continue
                    val label = try {
                        pm.getApplicationLabel(ai)?.toString()?.trim().orEmpty()
                    } catch (_: Exception) {
                        ""
                    }.ifBlank { pkg }
                    rows.add(AppRow(pkg, label, isSystem, ai.uid))
                }
                // Also merge launcher-only apps in case some were filtered
                try {
                    val launch = Intent(Intent.ACTION_MAIN).addCategory(Intent.CATEGORY_LAUNCHER)
                    val resolved = pm.queryIntentActivities(launch, 0)
                    val have = rows.map { it.pkg }.toHashSet()
                    for (ri in resolved) {
                        val pkg = ri.activityInfo?.packageName ?: continue
                        if (pkg == self || !have.add(pkg)) continue
                        val label = try {
                            ri.loadLabel(pm)?.toString() ?: pkg
                        } catch (_: Exception) {
                            pkg
                        }
                        rows.add(AppRow(pkg, label, false, 0))
                    }
                } catch (_: Exception) {
                }
                rows.sortWith(compareBy({ it.label.lowercase() }, { it.pkg }))
                val out = JSArray()
                for (row in rows) {
                    val o = JSObject()
                    o.put("packageName", row.pkg)
                    o.put("label", row.label)
                    o.put("system", row.system)
                    out.put(o)
                }
                val ret = JSObject()
                ret.put("apps", out)
                ret.put("count", rows.size)
                call.resolve(ret)
            } catch (e: Exception) {
                call.reject(e.message ?: "listApps failed")
            }
        }.start()
    }

    @PluginMethod
    fun openVpnSettings(call: PluginCall) {
        try {
            val i = Intent(Settings.ACTION_VPN_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            context.startActivity(i)
            call.resolve()
        } catch (e: Exception) {
            call.reject(e.message ?: "cannot open VPN settings")
        }
    }

    // Download subscription preview (servers list) via native HTTP with clash UA + HWID headers.
    @PluginMethod
    fun fetchSub(call: PluginCall) {
        val url = call.getString("url") ?: ""
        val hwid = call.getString("hwid") ?: ""
        val inlineBody = call.getString("body") ?: ""
        if (inlineBody.isNotBlank()) {
            val ret = JSObject()
            ret.put("status", 200)
            ret.put("body", inlineBody)
            call.resolve(ret)
            return
        }
        if (url.isEmpty()) {
            call.reject("url required")
            return
        }
        if (url.startsWith("local://", ignoreCase = true)) {
            val ret = JSObject()
            ret.put("status", 0)
            ret.put("body", "")
            ret.put("error", "local sub without body")
            call.resolve(ret)
            return
        }
        Thread {
            val ret = JSObject()
            try {
                val conn = java.net.URL(url).openConnection() as java.net.HttpURLConnection
                conn.connectTimeout = 10000
                conn.readTimeout = 20000
                conn.instanceFollowRedirects = true
                conn.setRequestProperty("User-Agent", "clash.meta/android")
                if (hwid.isNotEmpty()) {
                    conn.setRequestProperty("x-hwid", hwid)
                    conn.setRequestProperty("x-device-os", "Android")
                    conn.setRequestProperty("x-ver-os", Build.VERSION.RELEASE)
                    conn.setRequestProperty("x-device-model", Build.MODEL)
                }
                val code = conn.responseCode
                val text = (if (code in 200..299) conn.inputStream else conn.errorStream)
                    ?.bufferedReader()?.use { it.readText() } ?: ""
                conn.disconnect()
                ret.put("status", code)
                ret.put("body", text)
            } catch (e: Exception) {
                ret.put("status", 0)
                ret.put("body", "")
                ret.put("error", e.message ?: "fetch failed")
            }
            call.resolve(ret)
        }.start()
    }

    @PluginMethod
    fun ensureCoreForPing(call: PluginCall) {
        val subsJson = call.getString("subsJson") ?: "[]"
        val hwid = call.getString("hwid") ?: ""
        Thread {
            val ret = JSObject()
            try {
                val light = stripBodiesFromSubsJson(subsJson)
                val result = PingCore.ensure(context, light, hwid)
                for ((k, v) in result) {
                    when (v) {
                        is Int -> ret.put(k, v)
                        is Long -> ret.put(k, v)
                        is Boolean -> ret.put(k, v)
                        else -> ret.put(k, v.toString())
                    }
                }
                if (result["error"] != null && result["mode"] == "none") {
                    call.reject(result["error"].toString())
                } else {
                    call.resolve(ret)
                }
            } catch (e: Exception) {
                call.reject(e.message ?: "ensureCoreForPing failed")
            }
        }.start()
    }

    @PluginMethod
    fun stopPingCore(call: PluginCall) {
        Thread {
            try {
                PingCore.stopIfNeeded()
            } catch (_: Exception) {
            }
            call.resolve()
        }.start()
    }

    @PluginMethod
    fun coreRequest(call: PluginCall) {
        val method = (call.getString("method") ?: "GET").uppercase()
        val path = call.getString("path") ?: "/"
        val body = call.getString("body")
        val longTimeout = path.contains("delay") || path.contains("healthcheck")
        Thread {
            try {
                val uri = java.net.URI("http://127.0.0.1:9090$path")
                val conn = uri.toURL().openConnection() as java.net.HttpURLConnection
                conn.requestMethod = method
                conn.connectTimeout = 8000
                conn.readTimeout = if (longTimeout) 45000 else 12000
                if (body != null) {
                    conn.doOutput = true
                    conn.setRequestProperty("Content-Type", "application/json")
                    conn.outputStream.use { it.write(body.toByteArray(Charsets.UTF_8)) }
                }
                val code = conn.responseCode
                val stream = if (code in 200..299) conn.inputStream else conn.errorStream
                val text = stream?.bufferedReader()?.use { it.readText() } ?: ""
                conn.disconnect()
                val ret = JSObject()
                ret.put("status", code)
                ret.put("body", text)
                call.resolve(ret)
            } catch (e: Exception) {
                val ret = JSObject()
                ret.put("status", 0)
                ret.put("body", "")
                ret.put("error", e.message ?: "core unreachable")
                call.resolve(ret)
            }
        }.start()
    }
}
