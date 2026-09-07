package com.frozenfortress.companion

import android.annotation.SuppressLint
import java.security.MessageDigest
import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager

/**
 * Trust-On-First-Use TLS verification for a single FrozenFortress host.
 *
 * OkHttp's built-in `CertificatePinner` doesn't work here: it requires the pin to
 * be known in advance, which is exactly what TOFU by definition can't provide for
 * a self-hosted instance's self-signed certificate. Instead:
 *
 *  - No pin yet for [hostId]: [onNewPin] is invoked with the certificate's public
 *    key hash so the caller can show it to the user for confirmation (mirroring a
 *    browser's self-signed cert warning). If the user accepts, the pin is stored
 *    via [pinStore] and the connection proceeds; if not, the handshake fails.
 *  - Pin exists and matches: connection proceeds silently.
 *  - Pin exists and does NOT match: hard failure, always. There is no silent
 *    re-TOFU - a changed fingerprint on a previously-trusted host is exactly the
 *    MITM scenario TOFU exists to catch.
 *
 * [onNewPin] runs synchronously on the TLS handshake thread (never the main
 * thread). A caller that needs to show a dialog must block this thread until the
 * user responds, e.g. via a latch signaled from the dialog's callback on the main
 * thread.
 *
 * Lint's CustomX509TrustManager check is suppressed deliberately: a custom trust
 * manager that skips platform CA validation is exactly what TOFU pinning requires
 * - it's the point of this class, not an oversight.
 */
@SuppressLint("CustomX509TrustManager")
class TofuTrustManager(
    private val hostId: String,
    private val pinStore: PinStore,
    private val onNewPin: (hostId: String, publicKeyHash: String) -> Boolean
) : X509TrustManager {

    override fun checkClientTrusted(chain: Array<out X509Certificate>?, authType: String?) {
        throw CertificateException("Client certificates are not supported")
    }

    override fun checkServerTrusted(chain: Array<out X509Certificate>?, authType: String?) {
        val leaf = chain?.firstOrNull()
            ?: throw CertificateException("Certificate chain is empty")

        val publicKeyHash = hashPublicKey(leaf)
        val existingPin = pinStore.getPin(hostId)

        if (existingPin == null) {
            if (!onNewPin(hostId, publicKeyHash)) {
                throw CertificateException("Certificate for $hostId was not accepted by the user")
            }
            pinStore.pin(hostId, publicKeyHash)
            return
        }

        if (existingPin != publicKeyHash) {
            throw CertificateException(
                "Certificate for $hostId does not match the previously pinned fingerprint. " +
                    "Refusing to connect - this may indicate the server changed or a " +
                    "man-in-the-middle attack."
            )
        }
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()

    private fun hashPublicKey(certificate: X509Certificate): String {
        val digest = MessageDigest.getInstance("SHA-256")
        val hash = digest.digest(certificate.publicKey.encoded)
        return hash.joinToString(separator = "") { "%02x".format(it) }
    }

    companion object {
        /** Formats a raw pin hash as a colon-separated hex fingerprint for display to the user. */
        fun formatFingerprint(publicKeyHash: String): String =
            publicKeyHash.chunked(2).joinToString(":")
    }
}
