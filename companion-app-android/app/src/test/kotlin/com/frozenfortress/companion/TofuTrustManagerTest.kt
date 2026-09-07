package com.frozenfortress.companion

import org.junit.Assert.assertEquals
import org.junit.Test

class TofuTrustManagerTest {

    @Test
    fun `formatFingerprint groups hex hash into colon-separated pairs`() {
        val hash = "0a1b2c3d"

        val formatted = TofuTrustManager.formatFingerprint(hash)

        assertEquals("0a:1b:2c:3d", formatted)
    }
}
