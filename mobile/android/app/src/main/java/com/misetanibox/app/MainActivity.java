package com.misetanibox.app;

import android.Manifest;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.graphics.Color;
import android.net.VpnService;
import android.os.Build;
import android.os.Bundle;
import android.view.View;
import android.view.WindowManager;
import android.widget.Toast;

import androidx.activity.result.ActivityResultLauncher;
import androidx.activity.result.contract.ActivityResultContracts;
import androidx.core.app.ActivityCompat;
import androidx.core.content.ContextCompat;
import androidx.core.graphics.Insets;
import androidx.core.view.ViewCompat;
import androidx.core.view.WindowCompat;
import androidx.core.view.WindowInsetsCompat;
import androidx.core.view.WindowInsetsControllerCompat;

import com.misetanibox.app.network.NetworkAuto;
import com.misetanibox.app.plugin.VpnPlugin;
import com.misetanibox.app.security.AppLock;
import com.misetanibox.app.vpn.MihomoVpnService;
import com.misetanibox.app.vpn.VpnPrefs;
import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    public static final String EXTRA_REQUEST_VPN = "request_vpn";
    /** Open app on a specific tab (e.g. "servers" from notification action). */
    public static final String EXTRA_OPEN_TAB = "open_tab";
    private static final int REQ_POST_NOTIFICATIONS = 1001;

    private ActivityResultLauncher<Intent> vpnPrepareLauncher;
    private boolean pendingStartFromTile = false;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(VpnPlugin.class);

        vpnPrepareLauncher = registerForActivityResult(
                new ActivityResultContracts.StartActivityForResult(),
                result -> {
                    if (result.getResultCode() == RESULT_OK && pendingStartFromTile) {
                        pendingStartFromTile = false;
                        String err = VpnPrefs.INSTANCE.startFromPrefs(this);
                        if (err != null) {
                            Toast.makeText(this, err, Toast.LENGTH_SHORT).show();
                        }
                    } else {
                        pendingStartFromTile = false;
                    }
                }
        );

        super.onCreate(savedInstanceState);

        // Android 15/16: edge-to-edge is mandatory when targeting API 36
        WindowCompat.setDecorFitsSystemWindows(getWindow(), false);
        getWindow().setStatusBarColor(Color.TRANSPARENT);
        getWindow().setNavigationBarColor(Color.TRANSPARENT);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            getWindow().setNavigationBarContrastEnforced(false);
            getWindow().setStatusBarContrastEnforced(false);
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            WindowManager.LayoutParams lp = getWindow().getAttributes();
            lp.layoutInDisplayCutoutMode =
                    WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
            getWindow().setAttributes(lp);
        }

        WindowInsetsControllerCompat controller =
                WindowCompat.getInsetsController(getWindow(), getWindow().getDecorView());
        if (controller != null) {
            controller.setAppearanceLightStatusBars(false);
            controller.setAppearanceLightNavigationBars(false);
        }

        View content = findViewById(android.R.id.content);
        if (content != null) {
            ViewCompat.setOnApplyWindowInsetsListener(content, (v, insets) -> {
                Insets bars = insets.getInsets(WindowInsetsCompat.Type.systemBars()
                        | WindowInsetsCompat.Type.displayCutout());
                v.setPadding(0, 0, 0, 0);
                return insets;
            });
        }

        requestNotificationPermissionIfNeeded();
        handleIntent(getIntent());
        try {
            if (NetworkAuto.INSTANCE.isEnabled(this)) {
                NetworkAuto.INSTANCE.ensureRegistered(this);
            }
        } catch (Exception ignored) {
        }
        maybeAppLock();
    }

    private boolean unlockedThisSession = false;

    private void maybeAppLock() {
        if (!AppLock.INSTANCE.isEnabled(this)) return;
        if (unlockedThisSession) return;
        boolean shown = AppLock.INSTANCE.prompt(this, new AppLock.Callback() {
            @Override
            public void onSuccess() {
                unlockedThisSession = true;
            }

            @Override
            public void onFail(String message) {
                if ("unavailable".equals(message)) {
                    unlockedThisSession = true;
                    return;
                }
                if (!unlockedThisSession) {
                    Toast.makeText(MainActivity.this, "Нужна разблокировка", Toast.LENGTH_SHORT).show();
                    finish();
                }
            }
        });
        if (!shown) {
            unlockedThisSession = true;
        }
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);
        handleIntent(intent);
    }

    private void handleIntent(Intent intent) {
        if (intent == null) return;

        String openTab = intent.getStringExtra(EXTRA_OPEN_TAB);
        if (openTab != null && !openTab.isEmpty()) {
            intent.removeExtra(EXTRA_OPEN_TAB);
            // Bridge loads www; pass tab via localStorage-like query not available —
            // store for JS poll on next focus via SharedPreferences.
            getSharedPreferences("misetanibox", MODE_PRIVATE)
                    .edit()
                    .putString("pending_open_tab", openTab)
                    .apply();
        }

        if (!intent.getBooleanExtra(EXTRA_REQUEST_VPN, false)) return;
        // Clear so we don't loop on rotate
        intent.removeExtra(EXTRA_REQUEST_VPN);
        if (MihomoVpnService.isRunning()) return;
        if (!VpnPrefs.INSTANCE.hasSubscription(this)) {
            Toast.makeText(this, "Добавьте подписку в Misetanibox", Toast.LENGTH_SHORT).show();
            return;
        }
        Intent prepare = VpnService.prepare(this);
        if (prepare != null) {
            pendingStartFromTile = true;
            vpnPrepareLauncher.launch(prepare);
        } else {
            String err = VpnPrefs.INSTANCE.startFromPrefs(this);
            if (err != null) {
                Toast.makeText(this, err, Toast.LENGTH_SHORT).show();
            }
        }
    }

    private void requestNotificationPermissionIfNeeded() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return;
        if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS)
                == PackageManager.PERMISSION_GRANTED) {
            return;
        }
        ActivityCompat.requestPermissions(
                this,
                new String[]{Manifest.permission.POST_NOTIFICATIONS},
                REQ_POST_NOTIFICATIONS
        );
    }
}
