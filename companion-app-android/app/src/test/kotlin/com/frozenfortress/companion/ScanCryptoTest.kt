package com.frozenfortress.companion

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

class ScanCryptoTest {

    @Test
    fun `encrypt output decrypts back to the original plaintext with a manual AES-GCM decrypt`() {
        val keyHex = randomKeyHex()
        val plaintext = "a scanned document, pretend PDF bytes".toByteArray()

        val cipherBlob = ScanCrypto.encrypt(plaintext, keyHex)
        val decrypted = decryptGoStyle(cipherBlob, keyHex)

        assertArrayEquals(plaintext, decrypted)
    }

    @Test
    fun `encrypt prepends a 12-byte nonce before the sealed ciphertext`() {
        val keyHex = randomKeyHex()
        val plaintext = "x".toByteArray()

        val cipherBlob = ScanCrypto.encrypt(plaintext, keyHex)

        // 12-byte nonce + plaintext + 16-byte GCM tag
        assertEquals(12 + plaintext.size + 16, cipherBlob.size)
    }

    @Test
    fun `two encryptions of the same plaintext use different nonces`() {
        val keyHex = randomKeyHex()
        val plaintext = "same plaintext".toByteArray()

        val first = ScanCrypto.encrypt(plaintext, keyHex)
        val second = ScanCrypto.encrypt(plaintext, keyHex)

        assertArrayEquals(first.copyOfRange(0, 12), first.copyOfRange(0, 12)) // sanity
        assert(!first.copyOfRange(0, 12).contentEquals(second.copyOfRange(0, 12))) {
            "two encryptions produced the same nonce"
        }
    }

    @Test
    fun `rejects a key that does not decode to 32 bytes`() {
        assertThrows(IllegalArgumentException::class.java) {
            ScanCrypto.encrypt("data".toByteArray(), "aabb")
        }
    }

    private fun randomKeyHex(): String {
        val bytes = ByteArray(32)
        SecureRandom().nextBytes(bytes)
        return bytes.joinToString("") { "%02x".format(it) }
    }

    /** Mirrors core/encryption.EncryptionService.DecryptBytes: nonce || ciphertext+tag, both GCM. */
    private fun decryptGoStyle(cipherBlob: ByteArray, keyHex: String): ByteArray {
        val keyBytes = ByteArray(keyHex.length / 2) { i ->
            ((Character.digit(keyHex[i * 2], 16) shl 4) + Character.digit(keyHex[i * 2 + 1], 16)).toByte()
        }
        val nonce = cipherBlob.copyOfRange(0, 12)
        val sealed = cipherBlob.copyOfRange(12, cipherBlob.size)

        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, SecretKeySpec(keyBytes, "AES"), GCMParameterSpec(128, nonce))
        return cipher.doFinal(sealed)
    }
}
