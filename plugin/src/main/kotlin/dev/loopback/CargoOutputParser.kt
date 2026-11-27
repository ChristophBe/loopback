package dev.loopback

import com.google.gson.JsonParser
import dev.loopback.api.model.CheckResult
import dev.loopback.api.model.CheckResultTestsInner
import dev.loopback.api.model.CheckResultTestsInner.Status

/**
 * Turns the output of `cargo test` into a [CheckResult]. Pure functions over strings, so no Rust toolchain is needed
 * to test them; running cargo is the job of [CargoTestRunner].
 *
 * `expected` is the `left` and `got` the `right` value of a failed `assert_eq!(expected, got)`.
 */
object CargoOutputParser {
    /** Name of the single row reported when the project does not compile. */
    const val BUILD_ROW = "cargo build"

    /** Name of the row reported when cargo fails without any recognizable output. */
    const val UNKNOWN_ROW = "cargo test"

    private const val MILLIS_PER_SECOND = 1000
    private const val MAX_MESSAGE_LINES = 40

    private val textTest = Regex("""^test (.+) \.\.\. (ok|FAILED)\b.*$""")
    private val textFailure = Regex("""^---- (.+) stdout ----$""")
    private val assertion = Regex("""^assertion `left [=!]= right` failed""")
    private val compilerError =
        Regex("""^error(\[E\d+])?: (?!could not compile|test failed|doctest failed|\d+ targets? failed)""")

    /** Parses the output of `cargo test -- -Z unstable-options --format json`; null without libtest events. */
    fun parseLibtestJson(stdout: String): CheckResult? {
        val events =
            stdout
                .lineSequence()
                .filter { it.startsWith("{") }
                .mapNotNull { runCatching { JsonParser.parseString(it).asJsonObject }.getOrNull() }
                .toList()
        if (events.none { it.get("type")?.asString == "suite" }) return null
        val tests =
            events
                .filter { it.get("type")?.asString == "test" && it.get("event")?.asString in setOf("ok", "failed") }
                .map { event ->
                    val ok = event.get("event").asString == "ok"
                    val message = event.get("stdout")?.asString.orEmpty()
                    val duration =
                        event
                            .get("exec_time")
                            ?.asDouble
                            ?.times(MILLIS_PER_SECOND)
                            ?.toInt()
                    row(event.get("name").asString, ok, duration, message)
                }
        return CheckResult(tests.none { it.status == Status.fail }, tests)
    }

    /** Parses the plain `cargo test` output of a stable toolchain; null when no test summary is found. */
    fun parseText(stdout: String): CheckResult? {
        val lines = stdout.lines()
        if (lines.none { it.startsWith("test result:") }) return null
        val messages = failureMessages(lines)
        val tests =
            lines
                .mapNotNull { textTest.matchEntire(it)?.groupValues }
                .map { (_, name, status) -> row(name, status == "ok", null, messages[name].orEmpty()) }
        return CheckResult(tests.none { it.status == Status.fail }, tests)
    }

    /** Returns one `fail` row holding the compiler message when [stderr] reports a compile error, otherwise null. */
    fun parseBuildError(stderr: String): CheckResult? {
        val lines = stderr.lines()
        val start = lines.indexOfFirst { compilerError.containsMatchIn(it) }
        if (start < 0 || lines.none { it.startsWith("error: could not compile") }) return null
        val message =
            lines
                .drop(start)
                .takeWhile { !it.startsWith("error: could not compile") }
                .take(MAX_MESSAGE_LINES)
                .joinToString("\n")
                .trim()
        return CheckResult(false, listOf(CheckResultTestsInner(BUILD_ROW, Status.fail, got = message)))
    }

    private fun failureMessages(lines: List<String>): Map<String, String> {
        val messages = mutableMapOf<String, String>()
        var name: String? = null
        val body = mutableListOf<String>()

        fun flush() = name?.let { messages[it] = body.joinToString("\n") }
        for (line in lines) {
            val header = textFailure.matchEntire(line)
            when {
                header != null -> {
                    flush()
                    name = header.groupValues[1]
                    body.clear()
                }
                line == "failures:" -> {
                    flush()
                    name = null
                }
                name != null -> body.add(line)
            }
        }
        flush()
        return messages
    }

    private fun row(
        name: String,
        passed: Boolean,
        durationMs: Int?,
        message: String,
    ): CheckResultTestsInner {
        if (passed) return CheckResultTestsInner(name, Status.pass, durationMs)
        val (expected, got) = assertionValues(message)
        return CheckResultTestsInner(name, Status.fail, durationMs, expected, got)
    }

    /** Reads the `left` and `right` values of a failed `assert_eq!` from the panic message. */
    private fun assertionValues(message: String): Pair<String?, String?> {
        val lines = message.lines()
        val start = lines.indexOfFirst { assertion.containsMatchIn(it) }
        val rest = lines.drop(start + 1)
        val leftAt = rest.indexOfFirst { it.startsWith("  left: ") }
        val rightAt = rest.indexOfFirst { it.startsWith(" right: ") }
        if (start < 0 || leftAt < 0 || rightAt < leftAt) return null to null
        val left = rest.subList(leftAt, rightAt).joinToString("\n").removePrefix("  left: ")
        val right = rest.drop(rightAt).takeWhile { !it.startsWith("note: ") }.joinToString("\n")
        return left.trimEnd() to right.removePrefix(" right: ").trimEnd()
    }
}
