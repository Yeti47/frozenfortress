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
 * Uploads an encrypted scan to a FrozenFortress instance's session-less
 * scan-handoff staging endpoint: `POST {baseUrl}/api/scan-handoff/{token}/upload`.
 * Not authenticated by session - the handoff token in the path is the credential
 * (see `webui/views/scanhandoff` on the server side).
 *
 * TLS trust is established via Trust-On-First-Use ([TofuTrustManager]) rather than
 * the platform CA store, since self-hosted FrozenFortress instances typically run
 * with a self-signed certificate. Hostname verification against the cert's CN/SAN
 * is deliberately skipped: the trust anchor here is the pinned public key, not the
 * hostname, and self-signed certs for a LAN IP or loopback host frequently don't
 * carry a matching SAN anyway.
 */
class UploadClient(
    baseUrl: String,
    pinStore: PinStore,
    onNewPin: (hostId: String, publicKeyHash: String) -> Boolean
) {

    private val hostId = hostIdOf(baseUrl)
    private val trustManager = TofuTrustManager(hostId, pinStore, onNewPin)

    private val client: OkHttpClient = OkHttpClient.Builder()
        .sslSocketFactory(buildSslSocketFactory(trustManager), trustManager)
        .hostnameVerifier(HostnameVerifier { _, _ -> true })
        .callTimeout(60, TimeUnit.SECONDS)
        .build()

    /**
     * Uploads [cipherBlob] (the companion app's AES-256-GCM output: a 12-byte nonce
     * followed by the sealed ciphertext) for the given handoff [token].
     *
     * Returns true on a 200 response, false on any other HTTP response. Throws on
     * network/TLS failure - including a TOFU pin mismatch - so callers can tell
     * "server rejected the upload" apart from "the connection wasn't trustworthy".
     */
    fun uploadScan(baseUrl: String, token: String, fileName: String, cipherBlob: ByteArray): Boolean {
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
