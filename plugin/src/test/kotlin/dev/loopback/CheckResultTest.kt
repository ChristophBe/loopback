package dev.loopback

import com.google.gson.Gson
import dev.loopback.api.model.CheckResult
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull

class CheckResultTest {
    @Test
    fun decodesTheFailFixture() {
        val json = File("../api/schemas/fixtures/check-result-fail.json").readText()

        val result = Gson().fromJson(json, CheckResult::class.java)

        assertFalse(result.passed)
        assertEquals(2, result.tests.size)
        assertEquals("TestParseSimpleString", result.tests[0].name)
        assertEquals("+OK\\r\\n", result.tests[0].expected)
        assertEquals(3, result.tests[0].durationMs)
        assertNull(result.tests[1].got)
    }
}
