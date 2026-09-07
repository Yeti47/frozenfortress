package com.frozenfortress.companion

import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * AES-256-GCM encryption matching the server's `core/encryption.EncryptionService`
 * byte layout exactly: a random 12-byte nonce, followed by the GCM-sealed
 * ciphertext with its 16-byte authentication tag appended - which is both what
 * Go's `cipher.AEAD.Seal(nonce, nonce, plaintext, nil)` produces and what Java's
 * `Cipher.doFinal` produces by default. The server never gets this key from this
 * app (see [UploadClient]); it already holds its own copy from the browser's own
 * `/api/scan-handoff/start` call.
 */
object ScanCrypto {

    private const val TRANSFORMATION = "AES/GCM/NoPadding"
    private const val NONCE_SIZE_BYTES = 12
    private const val TAG_LENGTH_BITS = 128
    private const val KEY_SIZE_BYTES = 32 // AES-256

    /**
     * Encrypts [plainData] with [keyHex] (a 64-character hex string decoding to a
     * 32-byte AES-256 key - matches `generateScanKey()` in create-document.html).
     * Returns `nonce || ciphertext || tag`.
     */
    fun encrypt(plainData: ByteArray, keyHex: String): ByteArray {
        val keyBytes = decodeHex(keyHex)
        require(keyBytes.size == KEY_SIZE_BYTES) {
            "key must decode to $KEY_SIZE_BYTES bytes, got ${keyBytes.size}"
        }

        val nonce = ByteArray(NONCE_SIZE_BYTES)
        SecureRandom().nextBytes(nonce)

        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, SecretKeySpec(keyBytes, "AES"), GCMParameterSpec(TAG_LENGTH_BITS, nonce))
        val sealed = cipher.doFinal(plainData)

        return nonce + sealed
    }

    private fun decodeHex(hex: String): ByteArray {
        require(hex.length % 2 == 0) { "hex string must have an even length" }
        return ByteArray(hex.length / 2) { i ->
            val hi = Character.digit(hex[i * 2], 16)
            val lo = Character.digit(hex[i * 2 + 1], 16)
            require(hi >= 0 && lo >= 0) { "invalid hex string" }
            ((hi shl 4) + lo).toByte()
        }
    }
}
