package com.misetanibox.app.vpn
import android.content.Context
import android.os.Build
import mobilecore.Mobilecore
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.util.LinkedHashSet
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Offline ping: start mihomo with proxies + API only (no VpnService TUN).
 * UI then hits /proxies/.../delay on 127.0.0.1:9090.
 */
object PingCore {
    private val startedByUs = AtomicBoolean(false)

    data class Entry(val url: String, val label: String, val body: String)

    fun isActive(): Boolean = startedByUs.get() && !MihomoVpnService.isRunning

    /**
     * Ensure a core is reachable for delay tests.
     * @return map: mode=vpn|test, nodes=Int, error?=String
     */
    fun ensure(ctx: Context, subsJson: String, hwid: String): Map<String, Any> {
        if (MihomoVpnService.isRunning) {
            return mapOf("mode" to "vpn", "nodes" to -1)
        }
        // Already have test core up
        if (startedByUs.get()) {
            return mapOf("mode" to "test", "nodes" to -1)
        }

        val entries = parseEntries(ctx, subsJson)
        if (entries.isEmpty()) {
            return mapOf("mode" to "none", "nodes" to 0, "error" to "нет подписки")
        }

        val home = File(ctx.filesDir, "clash-ping").apply { mkdirs() }
        val providersDir = File(home, "providers").apply { mkdirs() }
        providersDir.listFiles()?.forEach { if (it.isFile) it.delete() }

        var total = 0
        val providerNames = mutableListOf<String>()
        val providersYaml = StringBuilder()
        var pIdx = 0
        for (entry in entries) {
            val name = "sub$pIdx"
            val tag = sanitizePrefix(entry.label, pIdx)
            if (entry.body.isNotBlank()) {
                val prefix = "[$tag] "
                val converted = try {
                    Mobilecore.convertSubscription(entry.body, prefix)
                } catch (_: Exception) {
                    ""
                }
                val n = try {
                    Mobilecore.convertSubscriptionCount(entry.body, prefix).toInt()
                } catch (_: Exception) {
                    0
                }
                if (converted.isBlank() || n <= 0) continue
                File(providersDir, "$name.yaml").writeText(converted, Charsets.UTF_8)
                providersYaml.append(
                    """
                    |  $name:
                    |    type: file
                    |    path: ./providers/$name.yaml
                    |    health-check:
                    |      enable: false
                    |      lazy: true
                    """.trimMargin()
                ).append('\n')
                providerNames.add(name)
                total += n
                pIdx++
            } else if (entry.url.startsWith("http", ignoreCase = true)) {
                val safeUrl = entry.url.replace("\"", "%22")
                val header = if (hwid.isNotEmpty()) {
                    """
                    |    header:
                    |      x-hwid: ["$hwid"]
                    |      x-device-os: ["Android"]
                    |      x-ver-os: ["${Build.VERSION.RELEASE}"]
                    |      x-device-model: ["${Build.MODEL}"]
                    """.trimMargin()
                } else ""
                providersYaml.append(
                    """
                    |  $name:
                    |    type: http
                    |    url: "$safeUrl"
                    |    interval: 86400
                    |    path: ./providers/$name.yaml
                    |    override:
                    |      additional-prefix: "[$tag] "
                    |    health-check:
                    |      enable: false
                    |      lazy: true
                    |$header
                    """.trimMargin()
                ).append('\n')
                providerNames.add(name)
                pIdx++
            }
        }
        if (providerNames.isEmpty()) {
            return mapOf("mode" to "none", "nodes" to 0, "error" to "0 валидных узлов")
        }
        val useList = providerNames.joinToString("\n") { "      - $it" }
        // IMPORTANT: no fake-ip here — without TUN, fake-ip (198.18.x.x) makes every delay timeout.
        // redir-host + plain DNS so proxy dials resolve real IPs for the test URL.
        val yaml = """
            |mixed-port: 7897
            |mode: global
            |log-level: info
            |ipv6: false
            |unified-delay: true
            |tcp-concurrent: true
            |external-controller: 127.0.0.1:9090
            |find-process-mode: off
            |dns:
            |  enable: true
            |  listen: 0.0.0.0:0
            |  ipv6: false
            |  enhanced-mode: redir-host
            |  default-nameserver:
            |    - 8.8.8.8
            |    - 1.1.1.1
            |    - 9.9.9.9
            |  nameserver:
            |    - 8.8.8.8
            |    - 1.1.1.1
            |    - tls://dns.google
            |  proxy-server-nameserver:
            |    - 8.8.8.8
            |    - 1.1.1.1
            |  direct-nameserver:
            |    - 8.8.8.8
            |    - 1.1.1.1
            |proxy-providers:
            |$providersYaml
            |proxy-groups:
            |  - name: PROXY
            |    type: select
            |    use:
            |$useList
            |rules:
            |  - MATCH,DIRECT
        """.trimMargin()

        File(home, "config.yaml").writeText(yaml, Charsets.UTF_8)
        val err = Mobilecore.startTest(home.absolutePath, yaml)
        if (err.isNotEmpty()) {
            startedByUs.set(false)
            return mapOf("mode" to "none", "nodes" to 0, "error" to err)
        }
        startedByUs.set(true)
        return mapOf("mode" to "test", "nodes" to total)
    }

    fun stopIfNeeded() {
        if (!startedByUs.get()) return
        if (MihomoVpnService.isRunning) {
            startedByUs.set(false)
            return
        }
        try {
            Mobilecore.stopTest()
        } catch (_: Exception) {
        }
        startedByUs.set(false)
    }

    private fun parseEntries(ctx: Context, json: String): List<Entry> {
        val out = ArrayList<Entry>()
        val seen = LinkedHashSet<String>()
        if (json.isBlank()) return out
        try {
            val arr = JSONArray(json)
            for (i in 0 until arr.length()) {
                val el = arr.opt(i) ?: continue
                var url = ""
                var label = ""
                var body = ""
                if (el is JSONObject) {
                    url = el.optString("url", "").trim()
                    label = el.optString("label", "").trim()
                    body = el.optString("body", "")
                } else {
                    url = el.toString().trim()
                }
                if (body.isBlank()) {
                    val lid = SubBodies.idFromUrl(url)
                    if (lid != null) body = SubBodies.get(ctx, lid)
                }
                if (body.isBlank() && SubBodies.idFromUrl(url) != null) continue
                val key = if (body.isNotBlank()) "b:${body.hashCode()}" else url
                if (key.isEmpty() || !seen.add(key)) continue
                if (url.isEmpty() && body.isBlank()) continue
                out.add(
                    Entry(
                        url.ifEmpty { "local://$i" },
                        label.ifBlank { "S${out.size + 1}" },
                        body,
                    ),
                )
            }
        } catch (_: Exception) {
        }
        return out
    }

    private fun sanitizePrefix(label: String, idx: Int): String {
        val base = label.trim().ifBlank { "S${idx + 1}" }
            .replace(Regex("[\\[\\]\"']"), "")
            .take(18)
        return if (base.isBlank()) "S${idx + 1}" else base
    }
}
