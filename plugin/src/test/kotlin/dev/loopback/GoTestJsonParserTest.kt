package dev.loopback

import dev.loopback.api.model.CheckResultTestsInner.Status
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class GoTestJsonParserTest {
    private fun fixture(name: String) = GoTestJsonParser.parse(File("../api/schemas/fixtures/$name").readText())

    @Test
    fun passingRunPasses() {
        val result = fixture("go-pass.jsonl")

        assertTrue(result.passed)
        assertEquals(listOf("TestParseSimpleString", "TestParseError"), result.tests.map { it.name })
        assertTrue(result.tests.all { it.status == Status.pass && it.expected == null && it.got == null })
    }

    @Test
    fun failingTestYieldsNameExpectedAndGot() {
        val result = fixture("go-fail.jsonl")

        assertFalse(result.passed)
        val first = result.tests[0]
        assertEquals("TestParseSimpleString", first.name)
        assertEquals(Status.fail, first.status)
        assertEquals("\"+OK\\r\\n\"", first.expected)
        assertEquals("\"OK\\r\\n\"", first.got)
        assertEquals(0, first.durationMs)
        assertEquals("\"-ERR\"", result.tests[1].expected)
    }

    @Test
    fun buildErrorYieldsOneFailRowWithCompilerMessage() {
        val result = fixture("go-build-error.jsonl")

        assertFalse(result.passed)
        assertEquals(1, result.tests.size)
        val row = result.tests[0]
        assertEquals(GoTestJsonParser.BUILD_ERROR_NAME, row.name)
        assertEquals(Status.fail, row.status)
        assertEquals("./parse.go:3:38: undefined: undefinedVar", row.got)
        assertNull(row.expected)
    }

    @Test
    fun gotWantOrderIsSwapped() {
        val line =
            """{"Action":"output","Test":"T","Output":"    x_test.go:1: got 1, want 2\n"}""" + "\n" +
                """{"Action":"fail","Test":"T","Elapsed":0.25}"""

        val row = GoTestJsonParser.parse(line).tests.single()

        assertEquals("2", row.expected)
        assertEquals("1", row.got)
        assertEquals(250, row.durationMs)
    }

    @Test
    fun packageFailureWithPassingTestsIsNotPassed() {
        val output =
            """{"Action":"pass","Test":"T","Elapsed":0}""" + "\n" + """{"Action":"fail","Package":"p","Elapsed":0}"""

        assertFalse(GoTestJsonParser.parse(output).passed)
    }

    @Test
    fun emptyOutputIsNotPassed() {
        assertFalse(GoTestJsonParser.parse("not json\n").passed)
    }
}
