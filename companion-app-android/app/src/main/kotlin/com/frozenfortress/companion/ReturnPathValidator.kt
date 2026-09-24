package com.frozenfortress.companion

import android.net.Uri
import androidx.core.net.toUri

/**
 * Validates the `return` deep-link parameter - the FrozenFortress page the
 * companion app sends the user back to once a scan has been uploaded.
 *
 * `ffscan` is a custom scheme, not an Android App Link (see AndroidManifest.xml),
 * so any app on the device can fire this intent with arbitrary parameters. `host`
 * is the trust anchor for the TLS/TOFU pin, which limits the blast radius, but a
 * first-contact host only raises the TOFU prompt, which the user can accept - so
 * the destination is not self-protecting. This allowlist is what keeps an
 * app-supplied value from steering navigation.
 *
 * The parameter carries a *site-relative* path (e.g. `/edit-document?id=abc&tab=files`),
 * never an absolute URL: the caller joins it onto the already-validated `host`, so
 * there is no second origin to validate and this object only has to reason about
 * paths. The scan key is deliberately never part of the return URL.
 *
 * [validate] is deliberately free of `android.net.Uri` so it can be covered by
 * plain JVM unit tests; only [buildReturnUrl] touches the framework URL builder.
 */
object ReturnPathValidator {

    /** Default destination when `return` is absent, empty, or not recognised. */
    const val DEFAULT_PATH = "/create-document"

    /** Longest accepted `id` value; server ids are well under this. */
    private const val MAX_ID_LENGTH = 64

    /**
     * Allowed path -> the query parameters that path may carry. A parameter not
     * listed here is a mismatch, not something to pass through silently.
     */
    private val ALLOWED: Map<String, Set<String>> = mapOf(
        "/create-document" to emptySet(),
        "/edit-document" to setOf("id", "tab"),
    )

    /**
     * Per-parameter value checks. Every allowed parameter must have one, enforced
     * in [init] so the allowlist can't be extended without also deciding what a
     * valid value looks like.
     */
    private val VALUE_VALIDATORS: Map<String, (String) -> Boolean> = mapOf(
        "id" to { value ->
            value.isNotEmpty() && value.length <= MAX_ID_LENGTH &&
                value.all { it.isLetterOrDigit() || it == '-' || it == '_' }
        },
        // Mirrors the tab names the edit page can actually show.
        "tab" to { value -> value in setOf("metadata", "files", "notes") },
    )

    init {
        for ((path, params) in ALLOWED) {
            for (param in params) {
                requireNotNull(VALUE_VALIDATORS[param]) {
                    "ReturnPathValidator: $path allows '$param' but has no value validator"
                }
            }
        }
    }

    /**
     * Returns the relative path to use, or [DEFAULT_PATH] if [raw] is missing or
     * fails validation.
     *
     * Rejected: absolute URLs / scheme-relative values (a crafted link must not be
     * able to redirect off-origin), unknown paths, unknown parameters, repeated
     * parameters, values failing their per-parameter check, fragments, userinfo,
     * traversal segments, encoded separators, and control characters.
     */
    fun validate(raw: String?): String {
        val candidate = raw.orEmpty()
        // Check control characters on the raw value, before any trimming could
        // quietly remove them.
        if (candidate.any { it.code < 0x20 || it.code == 0x7f }) return DEFAULT_PATH
        if (candidate.isBlank()) return DEFAULT_PATH

        // Must be a rooted relative path, not an absolute URL ("https://x") or
        // scheme-relative ("//evil.example"). "://" catches the former up front.
        if (!candidate.startsWith("/")) return DEFAULT_PATH
        if (candidate.startsWith("//")) return DEFAULT_PATH
        if (candidate.contains("://")) return DEFAULT_PATH
        // Fragments and userinfo have no place in a handoff destination.
        if (candidate.contains('#')) return DEFAULT_PATH
        if (candidate.contains('@')) return DEFAULT_PATH
        // Encoded separators could be decoded by a later hop and shift the path
        // boundary; there is no legitimate need for one here.
        if (candidate.contains("%2f", ignoreCase = true)) return DEFAULT_PATH
        if (candidate.contains("%5c", ignoreCase = true)) return DEFAULT_PATH

        val queryStart = candidate.indexOf('?')
        val path = if (queryStart < 0) candidate else candidate.substring(0, queryStart)
        val query = if (queryStart < 0) "" else candidate.substring(queryStart + 1)

        val allowedParams = ALLOWED[path] ?: return DEFAULT_PATH

        // Reject traversal/dot segments outright rather than trying to normalise.
        if (path.split('/').any { it == "." || it == ".." }) return DEFAULT_PATH

        val seen = mutableSetOf<String>()
        if (query.isNotEmpty()) {
            for (pair in query.split('&')) {
                if (pair.isEmpty()) return DEFAULT_PATH
                val separator = pair.indexOf('=')
                val name = if (separator < 0) pair else pair.substring(0, separator)
                val value = if (separator < 0) "" else pair.substring(separator + 1)

                if (name !in allowedParams) return DEFAULT_PATH
                // A repeated parameter is ambiguous downstream - reject rather than
                // guess which occurrence the page will honour.
                if (!seen.add(name)) return DEFAULT_PATH
                if (VALUE_VALIDATORS.getValue(name)(value) != true) return DEFAULT_PATH
            }
        }

        return candidate
    }

    /**
     * Builds the full return URL for [host] + [relativePath], appending the handoff
     * token as a query parameter.
     *
     * Uses [Uri.Builder] rather than string concatenation. The path may itself carry
     * a query string (`/edit-document?id=...&tab=files`), so its pairs are re-added
     * one by one: appending the whole string to the path would leave a literal `?`
     * inside the path component, and hand-building this URL is the usual source of
     * parameter smuggling.
     *
     * [relativePath] is expected to have already passed [validate]; this only
     * assembles.
     */
    fun buildReturnUrl(host: String, relativePath: String, token: String): Uri {
        val relative = relativePath.toUri()
        val builder = host.trimEnd('/').toUri().buildUpon()
        (relative.path ?: DEFAULT_PATH).split('/').forEach { segment ->
            if (segment.isNotEmpty()) builder.appendPath(segment)
        }
        for (name in relative.queryParameterNames) {
            for (value in relative.getQueryParameters(name)) {
                builder.appendQueryParameter(name, value)
            }
        }
        return builder.appendQueryParameter("scan", token).build()
    }
}
