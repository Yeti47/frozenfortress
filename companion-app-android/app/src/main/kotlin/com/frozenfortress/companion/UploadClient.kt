package com.frozenfortress.companion

import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLContext

/**
 * Talks to a single FrozenFortress instance's session-less scan-handoff staging
 * endpoints, scoped to [baseUrl] (the browser's `window.location.origin`, e.g.
 * `https://192.168.1.10:8443`).
 *
 * TLS trust is established via Trust-On-First-Use ([TofuTrustManager]) rather than
 * the platform CA store, since self-hosted FrozenFortress instances typically run
 * with a self-signed certificate. Hostname verification against the cert's CN/SAN
 * is deliberately skipped: the trust anchor here is the pinned public key, not the
 * hostname, and self-signed certs for a LAN IP or loopback host frequently don't
 * carry a matching SAN anyway. For a plain-HTTP `baseUrl` (self-hosted instances
 * without TLS on a trusted LAN), none of this engages at all - OkHttp simply never
 * negotiates TLS, and [verifyTrust] trivially succeeds.
 */
class UploadClient(
    private val baseUrl: String,
    pinStore: PinStore,
    onNewPin: (hostId: String, publicKeyHash: String) -> Boolean
) {

    private val trustManager = TofuTrustManager(hostIdOf(baseUrl), pinStore, onNewPin)

    private val client: OkHttpClient = OkHttpClient.Builder()
        .sslSocketFactory(buildSslSocketFactory(trustManager), trustManager)
        .hostnameVerifier(HostnameVerifier { _, _ -> true })
        .callTimeout(60, TimeUnit.SECONDS)
        .build()

    /**
     * Runs the TOFU TLS handshake against [baseUrl] up front, before the scan
     * capture UI is launched, so a rejected or mismatched certificate fails fast
     * instead of after the user has already scanned a document. The HTTP response
     * itself is irrelevant - only whether the handshake (and any user
     * pin-confirmation prompt) succeeded matters.
     *
     * Throws on any handshake/network failure, including a TOFU pin mismatch or
     * the user declining a new pin (surfaced as a `javax.net.ssl.SSLException`
     * wrapping the `CertificateException` from [TofuTrustManager]).
     */
    fun verifyTrust() {
        val request = Request.Builder().url(baseUrl).head().build()
        client.newCall(request).execute().close()
    }

    /**
     * Uploads [cipherBlob] (the companion app's AES-256-GCM output: a 12-byte nonce
     * followed by the sealed ciphertext - see [ScanCrypto]) for the given handoff
     * [token] via `POST {baseUrl}/api/scan-handoff/{token}/upload`. Not
     * authenticated by session - the handoff token in the path is the credential
     * (see `webui/views/scanhandoff` on the server side).
     *
     * Returns true on a 200 response, false on any other HTTP response. Throws on
     * network/TLS failure so callers can tell "server rejected the upload" apart
     * from "the connection wasn't trustworthy".
     */
    fun uploadScan(token: String, fileName: String, cipherBlob: ByteArray): Boolean {
        val url = "${baseUrl.trimEnd('/')}/api/scan-handoff/$token/upload"

        val body = MultipartBody.Builder()
            .setType(MultipartBody.FORM)
            .addFormDataPart(
                "file",
                fileName,
                cipherBlob.toRequestBody("application/octet-stream".toMediaType())
            )
            .build()

        val request = Request.Builder()
            .url(url)
            .post(body)
            .build()

        client.newCall(request).execute().use { response ->
            return response.isSuccessful
        }
    }

    private fun buildSslSocketFactory(trustManager: TofuTrustManager) =
        SSLContext.getInstance("TLS").apply {
            init(null, arrayOf(trustManager), null)
        }.socketFactory

    companion object {
        /** host:port identifies the pin - two ports on the same host may run different instances/certs. */
        private fun hostIdOf(baseUrl: String): String {
            val url = baseUrl.toHttpUrl()
            return "${url.host}:${url.port}"
        }
    }
}
