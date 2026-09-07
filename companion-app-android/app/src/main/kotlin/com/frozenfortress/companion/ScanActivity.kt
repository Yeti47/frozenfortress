package com.frozenfortress.companion

import android.app.Activity
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.view.View
import android.widget.ProgressBar
import android.widget.TextView
import androidx.activity.result.IntentSenderRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.core.net.toUri
import androidx.lifecycle.lifecycleScope
import com.google.android.material.button.MaterialButton
import com.google.mlkit.vision.documentscanner.GmsDocumentScannerOptions
import com.google.mlkit.vision.documentscanner.GmsDocumentScanning
import com.google.mlkit.vision.documentscanner.GmsDocumentScanningResult
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.util.concurrent.CountDownLatch

/**
 * Entry point for the `ffscan://scan` deep link opened from FrozenFortress's
 * "Create Document" page.
 *
 * Expected query parameters (see `webui/views/documents/create-document.html`):
 *  - `host`: the FrozenFortress origin to upload the scan back to
 *  - `token`: the one-time scan-handoff token
 *  - `key`: hex-encoded AES-256-GCM key to encrypt the scan with before upload
 *
 * Flow: parse the link -> TOFU TLS handshake against `host` (fails fast, before
 * wasting the user's time scanning) -> launch the ML Kit Document Scanner ->
 * encrypt the resulting PDF with `key` ([ScanCrypto]) -> upload it via
 * [UploadClient] -> prompt the user to return to the browser, which then fetches
 * and decrypts the scan itself using its own copy of the key.
 *
 * The key is held only in memory for this activity's lifetime (as a query
 * parameter string and inside [ScanCrypto]'s locals) and is never written to disk
 * or logged.
 */
class ScanActivity : AppCompatActivity() {

    private lateinit var statusText: TextView
    private lateinit var progressBar: ProgressBar
    private lateinit var returnButton: MaterialButton
    private lateinit var retryButton: MaterialButton
    private lateinit var cancelButton: MaterialButton

    private var host: String? = null
    private var token: String? = null
    private var key: String? = null

    private var uploadClient: UploadClient? = null

    private val scannerLauncher =
        registerForActivityResult(ActivityResultContracts.StartIntentSenderForResult()) { result ->
            handleScanResult(result.resultCode, result.data)
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_scan)

        statusText = findViewById(R.id.statusText)
        progressBar = findViewById(R.id.progressBar)
        returnButton = findViewById(R.id.returnButton)
        retryButton = findViewById(R.id.retryButton)
        cancelButton = findViewById(R.id.cancelButton)

        cancelButton.setOnClickListener { finish() }
        retryButton.setOnClickListener { startFlow() }

        if (!parseParams(intent.data)) {
            showError(getString(R.string.status_missing_params), allowRetry = false)
            return
        }

        startFlow()
    }

    private fun parseParams(uri: Uri?): Boolean {
        val h = uri?.getQueryParameter("host")
        val t = uri?.getQueryParameter("token")
        val k = uri?.getQueryParameter("key")

        if (h.isNullOrBlank() || t.isNullOrBlank() || !k.isValidScanKeyHex()) {
            return false
        }

        host = h
        token = t
        key = k
        return true
    }

    private fun String?.isValidScanKeyHex(): Boolean =
        this != null && length == 64 && all { it.isDigit() || it.lowercaseChar() in 'a'..'f' }

    private fun startFlow() {
        val h = host ?: return
        setBusy(getString(R.string.status_connecting))

        lifecycleScope.launch {
            try {
                val client = withContext(Dispatchers.IO) {
                    UploadClient(h, PinStore(applicationContext), ::confirmNewPinOnMainThread).apply {
                        verifyTrust()
                    }
                }
                uploadClient = client
                launchScanner()
            } catch (e: Exception) {
                showError(getString(R.string.status_connect_failed, e.message ?: e.toString()))
            }
        }
    }

    /**
     * Runs on the TLS handshake thread (see [TofuTrustManager]'s contract) - blocks
     * it until the user responds to a dialog shown on the main thread.
     */
    private fun confirmNewPinOnMainThread(hostId: String, publicKeyHash: String): Boolean {
        val latch = CountDownLatch(1)
        var accepted = false

        runOnUiThread {
            AlertDialog.Builder(this)
                .setTitle(R.string.tofu_dialog_title)
                .setMessage(
                    getString(
                        R.string.tofu_dialog_message,
                        hostId,
                        TofuTrustManager.formatFingerprint(publicKeyHash)
                    )
                )
                .setCancelable(false)
                .setPositiveButton(R.string.tofu_dialog_trust) { _, _ ->
                    accepted = true
                    latch.countDown()
                }
                .setNegativeButton(R.string.tofu_dialog_reject) { _, _ ->
                    accepted = false
                    latch.countDown()
                }
                .show()
        }

        latch.await()
        return accepted
    }

    private fun launchScanner() {
        setBusy(getString(R.string.status_opening_scanner))

        val options = GmsDocumentScannerOptions.Builder()
            .setGalleryImportAllowed(false)
            .setResultFormats(GmsDocumentScannerOptions.RESULT_FORMAT_PDF)
            .setScannerMode(GmsDocumentScannerOptions.SCANNER_MODE_FULL)
            .build()

        GmsDocumentScanning.getClient(options)
            .getStartScanIntent(this)
            .addOnSuccessListener { intentSender ->
                scannerLauncher.launch(IntentSenderRequest.Builder(intentSender).build())
            }
            .addOnFailureListener { e ->
                showError(getString(R.string.status_scanner_failed, e.message ?: e.toString()))
            }
    }

    private fun handleScanResult(resultCode: Int, data: Intent?) {
        if (resultCode != Activity.RESULT_OK) {
            // User backed out of the scanner UI - not an error, just leave quietly.
            finish()
            return
        }

        val pdfUri = GmsDocumentScanningResult.fromActivityResultIntent(data)?.pdf?.uri
        if (pdfUri == null) {
            showError(getString(R.string.status_scan_empty))
            return
        }

        encryptAndUpload(pdfUri)
    }

    private fun encryptAndUpload(pdfUri: Uri) {
        val client = uploadClient
        val scanToken = token
        val scanKey = key
        if (client == null || scanToken == null || scanKey == null) {
            showError(getString(R.string.status_connect_failed, "internal state was lost"))
            return
        }

        setBusy(getString(R.string.status_processing))

        lifecycleScope.launch {
            try {
                val success = withContext(Dispatchers.IO) {
                    val plainBytes = contentResolver.openInputStream(pdfUri)?.use { it.readBytes() }
                        ?: throw IllegalStateException("could not read the scanned document")
                    val cipherBlob = ScanCrypto.encrypt(plainBytes, scanKey)
                    client.uploadScan(scanToken, "scan-$scanToken.pdf", cipherBlob)
                }

                if (success) {
                    showReturnPrompt()
                } else {
                    showError(getString(R.string.status_upload_failed_server))
                }
            } catch (e: Exception) {
                showError(getString(R.string.status_upload_failed_error, e.message ?: e.toString()))
            }
        }
    }

    private fun showReturnPrompt() {
        progressBar.visibility = View.GONE
        statusText.text = getString(R.string.status_upload_success)
        retryButton.visibility = View.GONE
        cancelButton.visibility = View.GONE
        returnButton.visibility = View.VISIBLE
        returnButton.setOnClickListener { returnToBrowser() }
    }

    private fun returnToBrowser() {
        val h = host
        val t = token
        if (h == null || t == null) {
            finish()
            return
        }

        // RFC 8252-style external-user-agent handoff back to the browser -
        // deliberately without the key, which the browser already has its own copy of.
        val uri = "${h.trimEnd('/')}/create-document?scan=${Uri.encode(t)}".toUri()
        startActivity(Intent(Intent.ACTION_VIEW, uri))
        finish()
    }

    private fun setBusy(message: String) {
        statusText.text = message
        progressBar.visibility = View.VISIBLE
        returnButton.visibility = View.GONE
        retryButton.visibility = View.GONE
        cancelButton.visibility = View.GONE
    }

    private fun showError(message: String, allowRetry: Boolean = true) {
        statusText.text = message
        progressBar.visibility = View.GONE
        returnButton.visibility = View.GONE
        retryButton.visibility = if (allowRetry) View.VISIBLE else View.GONE
        cancelButton.visibility = View.VISIBLE
    }
}
