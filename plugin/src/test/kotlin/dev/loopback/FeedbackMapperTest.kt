package dev.loopback

import com.google.gson.Gson
import dev.loopback.api.model.CheckResult
import dev.loopback.api.model.CheckResultTestsInner.Status
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class FeedbackMapperTest {
    private val goLesson =
        LessonHints(
            hints = listOf("Look at the prefix.", "The reply starts with +."),
            failureExplanations = mapOf("TestParseSimpleString" to "A simple string keeps its + prefix."),
        )

    private val rustLesson =
        LessonHints(
            hints = listOf("Check the sign."),
            failureExplanations = mapOf("tests::adds_negative_numbers" to "Adding two negatives stays negative."),
        )

    // The newest recorded toolchain version is enough here, the parsers are tested against every version.
    private fun go(name: String): CheckResult {
        val output = File(RecordedFixtures.versions("go").last(), "$name.jsonl").readText()
        return GoTestJsonParser.parse(output)
    }

    private fun cargo(case: String): CheckResult {
        val json = File(RecordedFixtures.versions("cargo").last(), "$case/expected.json").readText()
        return Gson().fromJson(json, CheckResult::class.java)
    }

    @Test
    fun goFailureShowsExpectedGotAndTheHintOfThatTest() {
        val row = FeedbackMapper.map(go("fail"), goLesson).rows.first { it.name == "TestParseSimpleString" }

        assertEquals(Status.fail, row.status)
        assertEquals("\"+OK\\r\\n\"", row.expected)
        assertEquals("\"OK\\r\\n\"", row.got)
        assertEquals("A simple string keeps its + prefix.", row.hint)
    }

    @Test
    fun cargoFailureShowsExpectedGotAndTheHintOfThatTest() {
        val feedback = FeedbackMapper.map(cargo("fail"), rustLesson)

        val row = feedback.rows.single { it.status == Status.fail }
        val hint = "Adding two negatives stays negative."
        assertEquals(FeedbackRow("tests::adds_negative_numbers", Status.fail, "-2", "-3", hint), row)
        assertNull(feedback.rows.single { it.status == Status.pass }.hint)
    }

    @Test
    fun failureWithoutAnExplanationShowsTheRowWithoutAHint() {
        val row = FeedbackMapper.map(go("fail"), goLesson).rows.first { it.name == "TestParseError" }

        assertEquals(Status.fail, row.status)
        assertEquals("\"-ERR\"", row.expected)
        assertNull(row.hint)
    }

    @Test
    fun lessonWithoutHintsYieldsRowsWithoutHints() {
        val feedback = FeedbackMapper.map(go("fail"), LessonHints())

        assertTrue(feedback.rows.all { it.hint == null })
        assertTrue(feedback.generalHints.isEmpty())
    }

    @Test
    fun generalHintsKeepTheirOrderWhileTheRunFails() {
        assertEquals(goLesson.hints, FeedbackMapper.map(go("fail"), goLesson).generalHints)
    }

    @Test
    fun buildErrorRowTakesNoHintAndKeepsTheCompilerMessage() {
        val feedback = FeedbackMapper.map(go("build-error"), goLesson)

        val row = feedback.rows.single()
        assertNull(row.hint)
        assertTrue(row.got!!.isNotEmpty())
        assertEquals(goLesson.hints, feedback.generalHints)
    }

    @Test
    fun passingRunShowsNoHintsEvenForExplainedTests() {
        val lesson = LessonHints(listOf("h"), mapOf("TestParseError" to "explained"))

        val feedback = FeedbackMapper.map(go("pass"), lesson)

        assertTrue(feedback.passed)
        assertTrue(feedback.rows.all { it.status == Status.pass && it.hint == null })
        assertTrue(feedback.generalHints.isEmpty())
    }

    @Test
    fun cargoPassingRunShowsNoHints() {
        val feedback = FeedbackMapper.map(cargo("pass"), rustLesson)

        assertTrue(feedback.generalHints.isEmpty())
        assertTrue(feedback.rows.all { it.hint == null })
    }
}
