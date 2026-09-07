@file:Suppress("DEPRECATION") // EncryptedSharedPreferences/MasterKey - see class doc below.

package com.frozenfortress.companion

import android.content.Context
import android.content.SharedPreferences
import androidx.core.content.edit
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/**
 * Persists the Trust-On-First-Use certificate pin (hex-encoded SHA-256 hash of the
 * leaf certificate's public key) for each FrozenFortress host this app has
 * connected to.
 *
 * Backed by [EncryptedSharedPreferences] so a pin can't be read or swapped by
 * another app, or by inspecting app-private storage on a rooted device, without
 * also compromising the Android Keystore-backed master key.
 *
 * `EncryptedSharedPreferences`/`MasterKey` are deprecated in security-crypto 1.1.0
 * in favor of using Tink/Keystore directly (optionally via DataStore), but remain
 * functional. Deliberately kept for this single key-value pin store: the
 * replacement adds real complexity (managing an `AndroidKeysetManager`-backed
 * `Aead` by hand) for no benefit at this scale.
 */
class PinStore(context: Context) {

    private val prefs: SharedPreferences by lazy {
        val masterKey = MasterKey.Builder(context)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()

        EncryptedSharedPreferences.create(
            context,
            PREFS_FILE_NAME,
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
        )
    }

    /** Returns the pinned public-key hash for [hostId], or null if nothing is pinned yet. */
    fun getPin(hostId: String): String? = prefs.getString(hostId, null)

    /**
     * Pins [publicKeyHash] for [hostId], overwriting any previous pin.
     *
     * Callers must only do this after the user has explicitly confirmed the
     * fingerprint on first trust - see [TofuTrustManager]'s `onNewPin` callback.
     * Never re-pin silently on a mismatch; that defeats the point of TOFU.
     */
    fun pin(hostId: String, publicKeyHash: String) {
        prefs.edit { putString(hostId, publicKeyHash) }
    }

    companion object {
        private const val PREFS_FILE_NAME = "ffscan_tofu_pins"
    }
}
