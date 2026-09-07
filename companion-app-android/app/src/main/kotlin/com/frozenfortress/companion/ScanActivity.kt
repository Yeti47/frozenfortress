package com.frozenfortress.companion

import android.net.Uri
import android.os.Bundle
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity

/**
 * Entry point for the `ffscan://scan` deep link opened from FrozenFortress's
 * "Create Document" page.
 *
 * Expected query parameters (see `webui/views/documents/create-document.html`):
 *  - `host`: the FrozenFortress origin to upload the scan back to
 *  - `token`: the one-time scan-handoff token
 *  - `key`: hex-encoded AES-256-GCM key to encrypt the scan with before upload
 *
 * The actual ML Kit Document Scanner capture, client-side encryption, upload via
 * [UploadClient], and "return to browser" handoff are implemented in the
 * follow-up sub-issue (YETI-61). This scaffold only parses the incoming link so
 * the module builds and installs end-to-end.
 */
class ScanActivity : AppCompatActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val status = TextView(this)
        status.text = describeIntent(intent.data)
        setContentView(status)
    }

    private fun describeIntent(uri: Uri?): String {
        if (uri == null) {
            return "No scan link data received."
        }

        val host = uri.getQueryParameter("host")
        val token = uri.getQueryParameter("token")
        val hasKey = uri.getQueryParameter("key") != null

        return "Scan requested for $host (token: $token, key present: $hasKey).\n" +
            "Capture/upload flow not implemented yet (YETI-61)."
    }
}
