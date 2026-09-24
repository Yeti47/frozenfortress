package com.frozenfortress.companion

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Tests for [ReturnPathValidator.validate], the allowlist that keeps an
 * app-supplied `return` parameter from steering navigation after a scan.
 *
 * The Android SDK's `android.net.Uri` is a stub in plain JVM unit tests (it throws
 * "not mocked"), which is why `validate` avoids it - every case here is real logic.
 * [ReturnPathValidator.buildReturnUrl] does use `Uri`, so it is exercised by manual
 * E2E rather than here.
 */
class ReturnPathValidatorTest {

    // --- accepted -----------------------------------------------------------

    @Test
    fun `create document without parameters is accepted`() {
        assertEquals("/create-document", ReturnPathValidator.validate("/create-document"))
    }

    @Test
    fun `edit document with id and tab is accepted`() {
        val value = "/edit-document?id=abc123&tab=files"
        assertEquals(value, ReturnPathValidator.validate(value))
    }

    @Test
    fun `edit document with only an id is accepted`() {
        val value = "/edit-document?id=abc-123_XYZ"
        assertEquals(value, ReturnPathValidator.validate(value))
    }

    @Test
    fun `all known tab names are accepted`() {
        for (tab in listOf("metadata", "files", "notes")) {
            val value = "/edit-document?id=a&tab=$tab"
            assertEquals(value, ReturnPathValidator.validate(value))
        }
    }

    // --- fallback to the default -------------------------------------------

    @Test
    fun `missing or blank value falls back to create document`() {
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate(null))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate(""))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("   "))
    }

    @Test
    fun `unknown path falls back to create document`() {
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/account"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/documents?x=1"))
    }

    @Test
    fun `unknown parameter falls back to create document`() {
        assertEquals(
            ReturnPathValidator.DEFAULT_PATH,
            ReturnPathValidator.validate("/edit-document?id=a&next=https://evil.example"),
        )
        assertEquals(
            ReturnPathValidator.DEFAULT_PATH,
            ReturnPathValidator.validate("/create-document?scan=stolen"),
        )
    }

    @Test
    fun `parameter valid for one path is rejected on another`() {
        // "id" is meaningful for /edit-document only.
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/create-document?id=a"))
    }

    // --- off-origin attempts -----------------------------------------------

    @Test
    fun `absolute urls are rejected`() {
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("https://evil.example/"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("http://evil.example/create-document"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("ffscan://scan?host=x"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("javascript:alert(1)"))
    }

    @Test
    fun `scheme relative and non rooted values are rejected`() {
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("//evil.example/create-document"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("create-document"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("../create-document"))
    }

    @Test
    fun `fragment and userinfo are rejected`() {
        assertEquals(
            ReturnPathValidator.DEFAULT_PATH,
            ReturnPathValidator.validate("/edit-document?id=a#@evil.example"),
        )
        assertEquals(
            ReturnPathValidator.DEFAULT_PATH,
            ReturnPathValidator.validate("/edit-document@evil.example?id=a"),
        )
    }

    // --- traversal and encoding --------------------------------------------

    @Test
    fun `dot segments are rejected`() {
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/edit-document/../account"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/./create-document"))
    }

    @Test
    fun `encoded separators are rejected`() {
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/edit-document%2f..%2faccount"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/create-document%5cfoo"))
    }

    @Test
    fun `control characters are rejected`() {
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/create-document\n"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/create-document\u0000"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/edit-document?id=a\r\nX: y"))
    }

    // --- per-parameter value checks ----------------------------------------

    @Test
    fun `id with unsafe or oversized values is rejected`() {
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/edit-document?id="))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/edit-document?id=a%20b"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/edit-document?id=<script>"))
        assertEquals(
            ReturnPathValidator.DEFAULT_PATH,
            ReturnPathValidator.validate("/edit-document?id=" + "a".repeat(65)),
        )
    }

    @Test
    fun `unknown tab value is rejected`() {
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/edit-document?id=a&tab=admin"))
        assertEquals(ReturnPathValidator.DEFAULT_PATH, ReturnPathValidator.validate("/edit-document?id=a&tab="))
    }

    // --- ambiguity ----------------------------------------------------------

    @Test
    fun `repeated parameters are rejected`() {
        assertEquals(
            ReturnPathValidator.DEFAULT_PATH,
            ReturnPathValidator.validate("/edit-document?id=a&id=b"),
        )
        assertEquals(
            ReturnPathValidator.DEFAULT_PATH,
            ReturnPathValidator.validate("/edit-document?id=a&tab=files&tab=metadata"),
        )
    }

    @Test
    fun `empty parameter pair is rejected`() {
        assertEquals(
            ReturnPathValidator.DEFAULT_PATH,
            ReturnPathValidator.validate("/edit-document?id=a&&tab=files"),
        )
    }
}
