package dev.loopback

import dev.loopback.api.model.CheckResult
import dev.loopback.api.model.CheckResultTestsInner.Status

/** The hint sources of a lesson's front matter (`hints` and `failureExplanations`). */
data class LessonHints(
    val hints: List<String> = emptyList(),
    val failureExplanations: Map<String, String> = emptyMap(),
)

/** One row of the feedback list. [hint] is the lesson's explanation for this test, null when it has none. */
data class FeedbackRow(
    val name: String,
    val status: Status,
    val expected: String?,
    val got: String?,
    val hint: String?,
)

/**
 * What the learner sees for one run. [generalHints] is the lesson's ordered `hints` list, revealed one at a
 * time by the UI. It is empty when the run passed.
 */
data class Feedback(
    val passed: Boolean,
    val rows: List<FeedbackRow>,
    val generalHints: List<String>,
)

/** Maps the normalized result of any [CheckRunner] and the lesson's hints to what the lesson window shows. */
object FeedbackMapper {
    fun map(
        result: CheckResult,
        lesson: LessonHints,
    ): Feedback {
        val rows =
            result.tests.map {
                val hint = if (it.status == Status.pass) null else lesson.failureExplanations[it.name]
                FeedbackRow(it.name, it.status, it.expected, it.got, hint)
            }
        return Feedback(result.passed, rows, if (result.passed) emptyList() else lesson.hints)
    }
}
