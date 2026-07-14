package com.misetanibox.app.vpn
import android.content.Context
import java.io.File

/**
 * Local subscription payloads (vless/ss/vmess lists, YAML) live on disk —
 * never in Intent extras or SharedPreferences (those drop/truncate large bodies
 * and made subscriptions "disappear" when VPN started).
 */
object SubBodies {
    private const val DIR = "sub_bodies"

    private fun dir(ctx: Context): File =
        File(ctx.filesDir, DIR).apply { mkdirs() }

    private fun safeId(id: String): String =
        id.replace(Regex("[^a-zA-Z0-9._-]"), "_").take(80).ifBlank { "sub" }

    fun file(ctx: Context, id: String): File =
        File(dir(ctx), safeId(id) + ".txt")

    fun put(ctx: Context, id: String, body: String) {
        file(ctx, id).writeText(body, Charsets.UTF_8)
    }

    fun get(ctx: Context, id: String): String {
        val f = file(ctx, id)
        return if (f.isFile) f.readText(Charsets.UTF_8) else ""
    }

    fun delete(ctx: Context, id: String) {
        try {
            file(ctx, id).delete()
        } catch (_: Exception) {
        }
    }

    /** local://abc → abc */
    fun idFromUrl(url: String): String? {
        val u = url.trim()
        if (!u.startsWith("local://", ignoreCase = true)) return null
        return u.removePrefix("local://").removePrefix("LOCAL://").substringBefore('?').trim()
            .ifBlank { null }
    }
}
