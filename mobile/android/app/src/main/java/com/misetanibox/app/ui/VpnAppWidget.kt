package com.misetanibox.app.ui

import com.misetanibox.app.MainActivity
import com.misetanibox.app.R
import com.misetanibox.app.vpn.MihomoVpnService
import com.misetanibox.app.vpn.VpnPrefs
import android.app.PendingIntent
import android.appwidget.AppWidgetManager
import android.appwidget.AppWidgetProvider
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.widget.RemoteViews

/**
 * Home-screen widget: status + tap to open app / toggle VPN.
 */
class VpnAppWidget : AppWidgetProvider() {

    override fun onUpdate(context: Context, manager: AppWidgetManager, ids: IntArray) {
        for (id in ids) {
            updateWidget(context, manager, id)
        }
    }

    override fun onReceive(context: Context, intent: Intent) {
        super.onReceive(context, intent)
        when (intent.action) {
            ACTION_TOGGLE -> {
                if (MihomoVpnService.isRunning) {
                    VpnPrefs.stopVpn(context)
                } else {
                    val err = VpnPrefs.startFromPrefs(context)
                    if (err != null) {
                        val open = Intent(context, MainActivity::class.java).apply {
                            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                            if (err.contains("разрешение")) putExtra(MainActivity.EXTRA_REQUEST_VPN, true)
                        }
                        context.startActivity(open)
                    }
                }
                requestUpdate(context)
            }
            "com.misetanibox.app.VPN_STATE", Intent.ACTION_BOOT_COMPLETED -> requestUpdate(context)
        }
    }

    companion object {
        const val ACTION_TOGGLE = "com.misetanibox.app.WIDGET_TOGGLE"

        fun requestUpdate(ctx: Context) {
            try {
                val mgr = AppWidgetManager.getInstance(ctx)
                val cn = ComponentName(ctx, VpnAppWidget::class.java)
                val ids = mgr.getAppWidgetIds(cn)
                if (ids.isEmpty()) return
                val intent = Intent(ctx, VpnAppWidget::class.java).apply {
                    action = AppWidgetManager.ACTION_APPWIDGET_UPDATE
                    putExtra(AppWidgetManager.EXTRA_APPWIDGET_IDS, ids)
                }
                ctx.sendBroadcast(intent)
            } catch (_: Exception) {
            }
        }

        fun updateWidget(context: Context, manager: AppWidgetManager, id: Int) {
            val views = RemoteViews(context.packageName, R.layout.widget_vpn)
            val on = MihomoVpnService.isRunning
            views.setTextViewText(R.id.widget_status, if (on) "VPN ON" else "VPN OFF")
            views.setTextViewText(
                R.id.widget_mode,
                if (on) VpnPrefs.modeLabel(MihomoVpnService.currentMode) else "тап — вкл/выкл",
            )
            val openPi = PendingIntent.getActivity(
                context, 10,
                Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
                PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
            )
            val togglePi = PendingIntent.getBroadcast(
                context, 11,
                Intent(context, VpnAppWidget::class.java).setAction(ACTION_TOGGLE),
                PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
            )
            views.setOnClickPendingIntent(R.id.widget_root, openPi)
            views.setOnClickPendingIntent(R.id.widget_toggle, togglePi)
            manager.updateAppWidget(id, views)
        }
    }
}
