package com.misetanibox.app.vpn

import com.misetanibox.app.R

import com.misetanibox.app.MainActivity
import com.misetanibox.app.ui.VpnAppWidget
import com.misetanibox.app.ui.VpnTileService
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.VpnService
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.ParcelFileDescriptor
import mobilecore.Mobilecore
import mobilecore.SocketProtector
import org.json.JSONObject
import java.io.File
import java.net.HttpURLConnection
import java.net.URI
import java.util.LinkedHashSet
import java.util.concurrent.atomic.AtomicBoolean

class MihomoVpnService : VpnService() {

    private var tunFd: ParcelFileDescriptor? = null
    private val running = AtomicBoolean(false)
    private val userStop = AtomicBoolean(false)
    private val mainHandler = Handler(Looper.getMainLooper())
    private var trafficTicker: Runnable? = null
    private var prevUp = 0L
    private var prevDown = 0L
    private var prevTs = 0L
    private var lastServerHint = ""

    companion object {
        const val ACTION_START = "com.misetanibox.app.START"
        const val ACTION_STOP = "com.misetanibox.app.STOP"
        const val ACTION_NOTIF_STOP = "com.misetanibox.app.NOTIF_STOP"
        const val ACTION_NOTIF_SWITCH = "com.misetanibox.app.NOTIF_SWITCH"
        const val EXTRA_SUB_URL = "sub_url"
        /** JSON array of subscription URLs, e.g. ["https://a","https://b"] */
        const val EXTRA_SUB_URLS_JSON = "sub_urls_json"
        const val EXTRA_HWID = "hwid"
        /** "rule" | "global" | "direct" */
        const val EXTRA_MODE = "mode"
        const val CHANNEL_ID = "misetanibox_vpn"
        const val NOTIF_ID = 7

        @JvmStatic
        @Volatile
        var isRunning: Boolean = false
            private set

        @JvmStatic
        @Volatile
        var currentMode: String = "rule"
            private set

        @JvmStatic
        @Volatile
        var lastDownRate: Long = 0
            private set

        @JvmStatic
        @Volatile
        var lastUpRate: Long = 0
            private set
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP, ACTION_NOTIF_STOP -> {
                userStop.set(true)
                stopTunnel()
                return START_NOT_STICKY
            }
            ACTION_NOTIF_SWITCH -> {
                val open = Intent(this, MainActivity::class.java).apply {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP)
                    putExtra(MainActivity.EXTRA_OPEN_TAB, "servers")
                }
                startActivity(open)
                return START_STICKY
            }
            else -> {
                userStop.set(false)
                var subUrl = intent?.getStringExtra(EXTRA_SUB_URL) ?: ""
                var subsJson = intent?.getStringExtra(EXTRA_SUB_URLS_JSON) ?: ""
                var hwid = intent?.getStringExtra(EXTRA_HWID) ?: ""
                var mode = VpnPrefs.normalizeMode(intent?.getStringExtra(EXTRA_MODE))

                // Sticky restart / empty intent → restore from prefs
                if (subUrl.isBlank() && (subsJson.isBlank() || subsJson == "[]")) {
                    val p = VpnPrefs.prefs(this)
                    subUrl = p.getString(VpnPrefs.KEY_SUB_URL, "") ?: ""
                    subsJson = p.getString(VpnPrefs.KEY_SUBS_JSON, "[]") ?: "[]"
                    hwid = p.getString(VpnPrefs.KEY_HWID, "") ?: ""
                    mode = VpnPrefs.normalizeMode(p.getString(VpnPrefs.KEY_MODE, "rule"))
                }

                val entries = parseSubEntries(subsJson, subUrl)
                if (entries.isEmpty()) {
                    broadcast("error", "нет URL подписки")
                    stopSelf()
                    return START_NOT_STICKY
                }
                startForegroundNotif("Подключение…")
                Thread {
                    startTunnel(entries, hwid, mode)
                }.start()
            }
        }
        return START_STICKY
    }

    data class SubEntry(
        val url: String,
        val label: String,
        /** Inline subscription body (share-links / YAML). When set → proxy-provider type: file */
        val body: String = "",
    )

    private fun parseSubEntries(json: String, fallback: String): List<SubEntry> {
        val out = ArrayList<SubEntry>()
        val seen = LinkedHashSet<String>()
        if (json.isNotBlank()) {
            try {
                val arr = org.json.JSONArray(json)
                for (i in 0 until arr.length()) {
                    val el = arr.opt(i) ?: continue
                    val url: String
                    val label: String
                    var body: String
                    if (el is org.json.JSONObject) {
                        url = el.optString("url", "").trim()
                        label = el.optString("label", "").trim()
                        body = el.optString("body", "")
                    } else {
                        url = el.toString().trim()
                        label = ""
                        body = ""
                    }
                    // Load local keys from disk (not Intent — avoids TransactionTooLarge / prefs wipe)
                    if (body.isBlank()) {
                        val lid = SubBodies.idFromUrl(url)
                        if (lid != null) {
                            body = SubBodies.get(this, lid)
                        }
                    }
                    // Local sub without keys on disk cannot be used
                    if (body.isBlank() && SubBodies.idFromUrl(url) != null) {
                        continue
                    }
                    val key = when {
                        body.isNotBlank() -> "body:" + body.hashCode() + ":" + label
                        url.isNotEmpty() -> url
                        else -> continue
                    }
                    if (!seen.add(key)) continue
                    val resolvedUrl = when {
                        url.isNotEmpty() -> url
                        body.isNotBlank() -> "local://sub${out.size}"
                        else -> continue
                    }
                    out.add(
                        SubEntry(
                            resolvedUrl,
                            label.ifBlank { defaultLabel(resolvedUrl, out.size) },
                            body,
                        ),
                    )
                }
            } catch (_: Exception) {
            }
        }
        if (out.isEmpty() && fallback.isNotBlank()) {
            val u = fallback.trim()
            out.add(SubEntry(u, defaultLabel(u, 0)))
        }
        return out
    }

    private fun defaultLabel(url: String, idx: Int): String {
        if (url.startsWith("local://", ignoreCase = true)) {
            return "файл${idx + 1}"
        }
        return try {
            val host = java.net.URI(url).host ?: "sub${idx + 1}"
            host.removePrefix("www.").take(18)
        } catch (_: Exception) {
            "Подп.${idx + 1}"
        }
    }

    private fun applySplitTunneling(builder: Builder) {
        val mode = VpnPrefs.splitMode(this)
        val apps = VpnPrefs.splitApps(this)

        // Android forbids mixing addAllowedApplication + addDisallowedApplication.
        when {
            mode == "include" && apps.isNotEmpty() -> {
                // Only listed apps use VPN. Do not list ourselves (UI / localhost API).
                for (pkg in apps) {
                    if (pkg == packageName) continue
                    try {
                        builder.addAllowedApplication(pkg)
                    } catch (_: Exception) {
                    }
                }
            }
            else -> {
                // all / exclude: disallowed apps (and always our package) bypass VPN
                try {
                    builder.addDisallowedApplication(packageName)
                } catch (_: Exception) {
                }
                if (mode == "exclude") {
                    for (pkg in apps) {
                        if (pkg == packageName) continue
                        try {
                            builder.addDisallowedApplication(pkg)
                        } catch (_: Exception) {
                        }
                    }
                }
            }
        }
    }

    private fun startTunnel(subEntries: List<SubEntry>, hwid: String, mode: String) {
        if (!running.compareAndSet(false, true)) return
        try {
            val builder = Builder()
                .setSession("Misetanibox")
                .setMtu(1500)
                .addAddress("172.19.0.1", 30)
                .addRoute("0.0.0.0", 0)
                .addDnsServer("172.19.0.2")
                // Blocking = hold packets when tunnel reader is busy (helps leak resistance)
                .setBlocking(true)
                .setMetered(false)

            if (Build.VERSION.SDK_INT >= 33) {
                try {
                    val m = builder.javaClass.getMethod("allowFamily", Int::class.javaPrimitiveType)
                    m.invoke(builder, 2) // AF_INET
                } catch (_: Exception) {
                }
            }

            applySplitTunneling(builder)

            val pfd = builder.establish() ?: run {
                broadcast("error", "establish() вернул null — нет разрешения VPN или активен другой VPN")
                running.set(false)
                stopSelf()
                return
            }

            val fd = pfd.detachFd()
            tunFd = null

            val home = File(filesDir, "clash").apply { mkdirs() }
            // Drop stale provider files so disabled subscriptions cannot linger
            val providersDir = File(home, "providers").apply { mkdirs() }
            providersDir.listFiles()?.forEach { f ->
                if (f.isFile) f.delete()
            }
            // Also remove unused geo dumps if left from older installs
            listOf("GeoSite.dat", "geoip.metadb", "country.mmdb").forEach { n ->
                try { File(home, n).delete() } catch (_: Exception) {}
            }

            // Materialize local subscription bodies for mihomo type:file providers.
            var totalNodes = 0
            val readyEntries = ArrayList<SubEntry>()
            subEntries.forEachIndexed { idx, entry ->
                if (entry.body.isNotBlank()) {
                    val tag = sanitizePrefix(entry.label, idx)
                    val prefix = "[$tag] "
                    val converted = try {
                        Mobilecore.convertSubscription(entry.body, prefix)
                    } catch (e: Exception) {
                        broadcast("error", "конвертация $tag: ${e.message}")
                        ""
                    }
                    val n = try {
                        Mobilecore.convertSubscriptionCount(entry.body, prefix).toInt()
                    } catch (_: Exception) {
                        0
                    }
                    if (converted.isBlank() || n <= 0) {
                        return@forEachIndexed
                    }
                    File(providersDir, "sub${readyEntries.size}.yaml")
                        .writeText(converted, Charsets.UTF_8)
                    totalNodes += n
                    readyEntries.add(entry.copy(body = "__file__", label = entry.label))
                } else {
                    readyEntries.add(entry)
                }
            }
            if (readyEntries.isEmpty()) {
                broadcast("error", "нет валидных узлов в подписке (0 из ключей)")
                stopTunnel()
                maybeKillSwitchRestart()
                return
            }

            val config = buildConfig(readyEntries, hwid, mode, totalNodes)

            Mobilecore.setProtect(object : SocketProtector {
                override fun protect(sockFd: Long): Boolean {
                    return this@MihomoVpnService.protect(sockFd.toInt())
                }
            })

            try {
                PingCore.stopIfNeeded()
            } catch (_: Exception) {
            }

            val err = Mobilecore.start(home.absolutePath, config, fd.toLong())
            if (err.isNotEmpty()) {
                broadcast("error", err)
                stopTunnel()
                maybeKillSwitchRestart()
                return
            }

            currentMode = mode
            isRunning = true
            val modeLabel = VpnPrefs.modeLabel(mode)
            val nodesHint = if (totalNodes > 0) " · $totalNodes узлов" else ""
            lastServerHint = "TUN · $modeLabel$nodesHint"
            updateNotif(lastServerHint)
            startTrafficTicker()
            broadcast("connected", if (totalNodes > 0) "nodes=$totalNodes" else mode)
        } catch (e: Exception) {
            broadcast("error", e.message ?: "неизвестная ошибка запуска")
            stopTunnel()
            maybeKillSwitchRestart()
        }
    }

    private fun maybeKillSwitchRestart() {
        if (userStop.get()) return
        if (!VpnPrefs.isKillSwitch(this)) return
        if (!VpnPrefs.hasSubscription(this)) return
        mainHandler.postDelayed({
            if (isRunning || userStop.get()) return@postDelayed
            try {
                VpnPrefs.startFromPrefs(this)
            } catch (_: Exception) {
            }
        }, 1200)
    }

    private fun startTrafficTicker() {
        stopTrafficTicker()
        prevUp = 0L
        prevDown = 0L
        prevTs = System.currentTimeMillis()
        val tick = object : Runnable {
            override fun run() {
                if (!isRunning) return
                val self = this
                Thread {
                    try {
                        pollTrafficAndUpdateNotif()
                    } catch (_: Exception) {
                    }
                    if (isRunning) {
                        mainHandler.postDelayed(self, 3000)
                    }
                }.start()
            }
        }
        trafficTicker = tick
        mainHandler.postDelayed(tick, 2500)
    }

    private fun stopTrafficTicker() {
        trafficTicker?.let { mainHandler.removeCallbacks(it) }
        trafficTicker = null
        lastDownRate = 0
        lastUpRate = 0
    }

    private fun pollTrafficAndUpdateNotif() {
        val uri = URI("http://127.0.0.1:9090/connections")
        val conn = uri.toURL().openConnection() as HttpURLConnection
        conn.connectTimeout = 2000
        conn.readTimeout = 3000
        conn.requestMethod = "GET"
        val code = conn.responseCode
        val text = (if (code in 200..299) conn.inputStream else conn.errorStream)
            ?.bufferedReader()?.use { it.readText() } ?: ""
        conn.disconnect()
        if (code !in 200..299 || text.isBlank()) return
        val json = JSONObject(text)
        val up = json.optLong("uploadTotal", 0L)
        val down = json.optLong("downloadTotal", 0L)
        val now = System.currentTimeMillis()
        val dt = ((now - prevTs).coerceAtLeast(500)) / 1000.0
        val upR = ((up - prevUp).coerceAtLeast(0) / dt).toLong()
        val dnR = ((down - prevDown).coerceAtLeast(0) / dt).toLong()
        prevUp = up
        prevDown = down
        prevTs = now
        lastUpRate = upR
        lastDownRate = dnR
        val line = "↓ ${fmtRate(dnR)}  ↑ ${fmtRate(upR)}" +
            if (lastServerHint.isNotBlank()) " · ${lastServerHint.removePrefix("TUN · ")}" else ""
        updateNotif(line)
    }

    private fun fmtRate(bps: Long): String {
        if (bps < 1024) return "$bps B/s"
        val kb = bps / 1024.0
        if (kb < 1024) return String.format("%.1f KB/s", kb)
        return String.format("%.1f MB/s", kb / 1024.0)
    }

    private fun stopTunnel() {
        stopTrafficTicker()
        try {
            Mobilecore.stop()
        } catch (_: Exception) {
        }
        try {
            Mobilecore.setProtect(null)
        } catch (_: Exception) {
        }
        try {
            tunFd?.close()
        } catch (_: Exception) {
        }
        tunFd = null
        running.set(false)
        isRunning = false
        broadcast("disconnected", "")
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onDestroy() {
        if (running.get() || isRunning) stopTunnel()
        super.onDestroy()
    }

    override fun onRevoke() {
        val wantRestart = VpnPrefs.isKillSwitch(this) && !userStop.get()
        stopTunnel()
        if (wantRestart) {
            mainHandler.postDelayed({
                try {
                    VpnPrefs.startFromPrefs(this)
                } catch (_: Exception) {
                }
            }, 800)
        }
        super.onRevoke()
    }

    /**
     * Config with three routing modes:
     * - global: all traffic → selected PROXY server
     * - rule: groups from rules.md
     * - direct: VPN up, traffic goes DIRECT (test / DNS / logs)
     */
    private fun buildConfig(
        subEntries: List<SubEntry>,
        hwid: String,
        mode: String,
        @Suppress("UNUSED_PARAMETER") preCountedNodes: Int = 0,
    ): String {
        val header = if (hwid.isNotEmpty()) {
            """
            |    header:
            |      x-hwid: ["$hwid"]
            |      x-device-os: ["Android"]
            |      x-ver-os: ["${Build.VERSION.RELEASE}"]
            |      x-device-model: ["${Build.MODEL}"]
            """.trimMargin()
        } else {
            ""
        }
        val modeYaml = when (VpnPrefs.normalizeMode(mode)) {
            "global" -> "global"
            "direct" -> "direct"
            else -> "rule"
        }

        val providerNames = mutableListOf<String>()
        val providersYaml = StringBuilder()
        subEntries.forEachIndexed { idx, entry ->
            val name = "sub$idx"
            providerNames.add(name)
            val tag = sanitizePrefix(entry.label, idx)
            val safePrefix = tag.replace("\"", "'")
            if (entry.body.isNotBlank()) {
                providersYaml.append(
                    """
                    |  $name:
                    |    type: file
                    |    path: ./providers/$name.yaml
                    |    health-check:
                    |      enable: false
                    |      url: https://www.gstatic.com/generate_204
                    |      interval: 3600
                    |      lazy: true
                    """.trimMargin()
                ).append('\n')
            } else {
                val safeUrl = entry.url.replace("\"", "%22")
                providersYaml.append(
                    """
                    |  $name:
                    |    type: http
                    |    url: "$safeUrl"
                    |    interval: 3600
                    |    path: ./providers/$name.yaml
                    |    override:
                    |      additional-prefix: "[$safePrefix] "
                    |    health-check:
                    |      enable: false
                    |      url: https://www.gstatic.com/generate_204
                    |      interval: 3600
                    |      lazy: true
                    |$header
                    """.trimMargin()
                ).append('\n')
            }
        }
        val useList = providerNames.joinToString("\n") { "      - $it" }

        return """
            |mixed-port: 7890
            |mode: $modeYaml
            |log-level: info
            |ipv6: false
            |unified-delay: true
            |tcp-concurrent: true
            |external-controller: 127.0.0.1:9090
            |find-process-mode: off
            |dns:
            |  enable: true
            |  listen: 0.0.0.0:1053
            |  ipv6: false
            |  enhanced-mode: fake-ip
            |  fake-ip-range: 198.18.0.1/16
            |  fake-ip-filter:
            |    - "*.lan"
            |    - "*.local"
            |    - "localhost.ptlogin2.qq.com"
            |    - "dns.msftncsi.com"
            |    - "www.msftconnecttest.com"
            |    - "connectivitycheck.gstatic.com"
            |  default-nameserver:
            |    - 1.1.1.1
            |    - 8.8.8.8
            |  nameserver:
            |    - https://1.1.1.1/dns-query
            |    - https://8.8.8.8/dns-query
            |  proxy-server-nameserver:
            |    - https://1.1.1.1/dns-query
            |    - 1.1.1.1
            |  direct-nameserver:
            |    - 1.1.1.1
            |    - 8.8.8.8
            |proxy-providers:
            |$providersYaml
            |proxy-groups:
            |  - name: PROXY
            |    type: select
            |    use:
            |$useList
            |  - name: Fastest
            |    type: url-test
            |    use:
            |$useList
            |    url: https://www.gstatic.com/generate_204
            |    interval: 600
            |    tolerance: 50
            |    lazy: true
            |  - name: Зарубежные
            |    type: select
            |    proxies:
            |      - PROXY
            |      - Fastest
            |      - DIRECT
            |  - name: Российские
            |    type: select
            |    proxies:
            |      - DIRECT
            |      - PROXY
            |  - name: AI
            |    type: select
            |    proxies:
            |      - Зарубежные
            |      - PROXY
            |      - DIRECT
            |  - name: Discord
            |    type: select
            |    proxies:
            |      - Зарубежные
            |      - PROXY
            |      - DIRECT
            |  - name: Telegram
            |    type: select
            |    proxies:
            |      - Зарубежные
            |      - PROXY
            |      - DIRECT
            |  - name: YouTube
            |    type: select
            |    proxies:
            |      - Зарубежные
            |      - PROXY
            |      - DIRECT
            |  - name: Игры
            |    type: select
            |    proxies:
            |      - DIRECT
            |      - PROXY
            |  - name: Torrent
            |    type: select
            |    proxies:
            |      - DIRECT
            |      - PROXY
            |rules:
            |  - IP-CIDR,127.0.0.0/8,DIRECT,no-resolve
            |  - IP-CIDR,10.0.0.0/8,DIRECT,no-resolve
            |  - IP-CIDR,172.16.0.0/12,DIRECT,no-resolve
            |  - IP-CIDR,192.168.0.0/16,DIRECT,no-resolve
            |  - IP-CIDR,169.254.0.0/16,DIRECT,no-resolve
            |  - DOMAIN-SUFFIX,openai.com,AI
            |  - DOMAIN-SUFFIX,chatgpt.com,AI
            |  - DOMAIN-SUFFIX,oaistatic.com,AI
            |  - DOMAIN-SUFFIX,oaiusercontent.com,AI
            |  - DOMAIN-SUFFIX,anthropic.com,AI
            |  - DOMAIN-SUFFIX,claude.ai,AI
            |  - DOMAIN-SUFFIX,gemini.google.com,AI
            |  - DOMAIN-SUFFIX,aistudio.google.com,AI
            |  - DOMAIN-SUFFIX,perplexity.ai,AI
            |  - DOMAIN-SUFFIX,mistral.ai,AI
            |  - DOMAIN-SUFFIX,grok.com,AI
            |  - DOMAIN-SUFFIX,x.ai,AI
            |  - DOMAIN-SUFFIX,poe.com,AI
            |  - DOMAIN-SUFFIX,huggingface.co,AI
            |  - DOMAIN-SUFFIX,replicate.com,AI
            |  - DOMAIN-SUFFIX,together.ai,AI
            |  - DOMAIN-SUFFIX,fireworks.ai,AI
            |  - DOMAIN-SUFFIX,groq.com,AI
            |  - DOMAIN-SUFFIX,cohere.ai,AI
            |  - DOMAIN-SUFFIX,deepseek.com,AI
            |  - DOMAIN-SUFFIX,discord.com,Discord
            |  - DOMAIN-SUFFIX,discord.gg,Discord
            |  - DOMAIN-SUFFIX,discordapp.com,Discord
            |  - DOMAIN-SUFFIX,discord.media,Discord
            |  - DOMAIN-SUFFIX,discordapp.net,Discord
            |  - DOMAIN-SUFFIX,discordcdn.com,Discord
            |  - DOMAIN-SUFFIX,discordstatus.com,Discord
            |  - DOMAIN-SUFFIX,t.me,Telegram
            |  - DOMAIN-SUFFIX,telegram.me,Telegram
            |  - DOMAIN-SUFFIX,telegram.org,Telegram
            |  - DOMAIN-SUFFIX,telegra.ph,Telegram
            |  - DOMAIN-SUFFIX,tdesktop.com,Telegram
            |  - DOMAIN-SUFFIX,telegram-cdn.org,Telegram
            |  - IP-CIDR,91.108.4.0/22,Telegram,no-resolve
            |  - IP-CIDR,91.108.8.0/22,Telegram,no-resolve
            |  - IP-CIDR,91.108.12.0/22,Telegram,no-resolve
            |  - IP-CIDR,91.108.16.0/22,Telegram,no-resolve
            |  - IP-CIDR,91.108.20.0/22,Telegram,no-resolve
            |  - IP-CIDR,91.108.56.0/22,Telegram,no-resolve
            |  - IP-CIDR,149.154.160.0/20,Telegram,no-resolve
            |  - IP-CIDR,185.76.151.0/24,Telegram,no-resolve
            |  - DOMAIN-SUFFIX,youtube.com,YouTube
            |  - DOMAIN-SUFFIX,youtu.be,YouTube
            |  - DOMAIN-SUFFIX,ytimg.com,YouTube
            |  - DOMAIN-SUFFIX,googlevideo.com,YouTube
            |  - DOMAIN-SUFFIX,yt3.ggpht.com,YouTube
            |  - DOMAIN-SUFFIX,youtubei.googleapis.com,YouTube
            |  - DOMAIN-SUFFIX,ggpht.com,YouTube
            |  - DOMAIN-SUFFIX,yandex.ru,Российские
            |  - DOMAIN-SUFFIX,yandex.net,Российские
            |  - DOMAIN-SUFFIX,yandex.com,Российские
            |  - DOMAIN-SUFFIX,ya.ru,Российские
            |  - DOMAIN-SUFFIX,yastatic.net,Российские
            |  - DOMAIN-SUFFIX,vk.com,Российские
            |  - DOMAIN-SUFFIX,vk.ru,Российские
            |  - DOMAIN-SUFFIX,vk.me,Российские
            |  - DOMAIN-SUFFIX,userapi.com,Российские
            |  - DOMAIN-SUFFIX,vkuservideo.net,Российские
            |  - DOMAIN-SUFFIX,ok.ru,Российские
            |  - DOMAIN-SUFFIX,mail.ru,Российские
            |  - DOMAIN-SUFFIX,imgsmail.ru,Российские
            |  - DOMAIN-SUFFIX,mycdn.me,Российские
            |  - DOMAIN-SUFFIX,gosuslugi.ru,Российские
            |  - DOMAIN-SUFFIX,sberbank.ru,Российские
            |  - DOMAIN-SUFFIX,sber.ru,Российские
            |  - DOMAIN-SUFFIX,tinkoff.ru,Российские
            |  - DOMAIN-SUFFIX,tbank.ru,Российские
            |  - DOMAIN-SUFFIX,alfabank.ru,Российские
            |  - DOMAIN-SUFFIX,vtb.ru,Российские
            |  - DOMAIN-SUFFIX,wildberries.ru,Российские
            |  - DOMAIN-SUFFIX,wb.ru,Российские
            |  - DOMAIN-SUFFIX,wbbasket.ru,Российские
            |  - DOMAIN-SUFFIX,ozon.ru,Российские
            |  - DOMAIN-SUFFIX,ozonusercontent.com,Российские
            |  - DOMAIN-SUFFIX,avito.ru,Российские
            |  - DOMAIN-SUFFIX,avito.st,Российские
            |  - DOMAIN-SUFFIX,dzen.ru,Российские
            |  - DOMAIN-SUFFIX,rutube.ru,Российские
            |  - DOMAIN-SUFFIX,2gis.ru,Российские
            |  - DOMAIN-SUFFIX,2gis.com,Российские
            |  - DOMAIN-SUFFIX,hh.ru,Российские
            |  - DOMAIN-SUFFIX,rbc.ru,Российские
            |  - DOMAIN-SUFFIX,rambler.ru,Российские
            |  - DOMAIN-SUFFIX,lenta.ru,Российские
            |  - DOMAIN-SUFFIX,ria.ru,Российские
            |  - DOMAIN-SUFFIX,rt.com,Российские
            |  - DOMAIN-SUFFIX,kinopoisk.ru,Российские
            |  - DOMAIN-SUFFIX,ivi.ru,Российские
            |  - DOMAIN-SUFFIX,okko.tv,Российские
            |  - DOMAIN-SUFFIX,premier.one,Российские
            |  - DOMAIN-SUFFIX,dns-shop.ru,Российские
            |  - DOMAIN-SUFFIX,mvideo.ru,Российские
            |  - DOMAIN-SUFFIX,citilink.ru,Российские
            |  - DOMAIN-SUFFIX,cdek.ru,Российские
            |  - DOMAIN-SUFFIX,pochta.ru,Российские
            |  - DOMAIN-SUFFIX,mos.ru,Российские
            |  - DOMAIN-SUFFIX,nalog.ru,Российские
            |  - DOMAIN-SUFFIX,gosuslugi.cloud,Российские
            |  - DOMAIN-KEYWORD,.ru,Российские
            |  - DOMAIN-KEYWORD,.su,Российские
            |  - DOMAIN-KEYWORD,.xn--p1ai,Российские
            |  - MATCH,Зарубежные
        """.trimMargin()
    }

    private fun sanitizePrefix(label: String, idx: Int): String {
        val base = label
            .replace(Regex("[\\[\\]\"'\\\\]"), "")
            .replace(Regex("\\s+"), " ")
            .trim()
            .ifBlank { "S${idx + 1}" }
        val short = if (base.length > 14) base.take(14) else base
        return "S${idx + 1}·$short"
    }

    private fun startForegroundNotif(text: String) {
        ensureChannel()
        val notif = buildNotif(text)
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(NOTIF_ID, notif, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else {
            startForeground(NOTIF_ID, notif)
        }
    }

    private fun updateNotif(text: String) {
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        nm.notify(NOTIF_ID, buildNotif(text))
    }

    private fun ensureChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        val ch = NotificationChannel(CHANNEL_ID, "VPN", NotificationManager.IMPORTANCE_LOW).apply {
            description = "Статус VPN-туннеля Misetanibox"
            setShowBadge(false)
        }
        nm.createNotificationChannel(ch)
    }

    private fun buildNotif(text: String): Notification {
        val openPi = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        val stopPi = PendingIntent.getService(
            this, 1,
            Intent(this, MihomoVpnService::class.java).setAction(ACTION_NOTIF_STOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        val switchPi = PendingIntent.getService(
            this, 2,
            Intent(this, MihomoVpnService::class.java).setAction(ACTION_NOTIF_SWITCH),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("Misetanibox · VPN")
            .setContentText(text)
            .setStyle(Notification.BigTextStyle().bigText(text))
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentIntent(openPi)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setCategory(Notification.CATEGORY_SERVICE)
            .addAction(Notification.Action.Builder(null, "Отключить", stopPi).build())
            .addAction(Notification.Action.Builder(null, "Сервер", switchPi).build())
            .build()
    }

    private fun broadcast(state: String, message: String) {
        val i = Intent("com.misetanibox.app.VPN_STATE")
        i.setPackage(packageName)
        i.putExtra("state", state)
        i.putExtra("message", message)
        sendBroadcast(i)
        VpnTileService.requestUpdate(this)
        VpnAppWidget.requestUpdate(this)
    }
}
