package com.misetanibox.app.security

import com.misetanibox.app.vpn.VpnPrefs
import android.app.Activity
import android.content.Context
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity

object AppLock {
    const val KEY_BIOMETRIC = "biometric_lock"

    interface Callback {
        fun onSuccess()
        fun onFail(message: String)
    }

    fun isEnabled(ctx: Context): Boolean =
        VpnPrefs.prefs(ctx).getBoolean(KEY_BIOMETRIC, false)

    fun canAuthenticate(ctx: Context): Boolean {
        val bm = BiometricManager.from(ctx)
        val res = bm.canAuthenticate(
            BiometricManager.Authenticators.BIOMETRIC_WEAK or
                BiometricManager.Authenticators.DEVICE_CREDENTIAL,
        )
        return res == BiometricManager.BIOMETRIC_SUCCESS
    }

    /**
     * Show system biometric / device credential prompt.
     * @return true if prompt shown, false if unavailable
     */
    fun prompt(activity: Activity, cb: Callback): Boolean {
        if (activity !is FragmentActivity) {
            cb.onFail("activity")
            return false
        }
        if (!canAuthenticate(activity)) {
            cb.onFail("unavailable")
            return false
        }
        val executor = ContextCompat.getMainExecutor(activity)
        val prompt = BiometricPrompt(
            activity,
            executor,
            object : BiometricPrompt.AuthenticationCallback() {
                override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) {
                    cb.onSuccess()
                }

                override fun onAuthenticationError(errorCode: Int, errString: CharSequence) {
                    cb.onFail(errString.toString())
                }

                override fun onAuthenticationFailed() {
                    // keep prompt open
                }
            },
        )
        val info = BiometricPrompt.PromptInfo.Builder()
            .setTitle("Misetanibox")
            .setSubtitle("Подтвердите вход")
            .setAllowedAuthenticators(
                BiometricManager.Authenticators.BIOMETRIC_WEAK or
                    BiometricManager.Authenticators.DEVICE_CREDENTIAL,
            )
            .build()
        prompt.authenticate(info)
        return true
    }
}
