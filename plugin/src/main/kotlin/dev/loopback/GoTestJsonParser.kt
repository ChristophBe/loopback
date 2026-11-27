package dev.loopback

import com.google.gson.JsonParser
import dev.loopback.api.model.CheckResult
import dev.loopback.api.model.CheckResultTestsInner
import dev.loopback.api.model.CheckResultTestsInner.Status
import kotlin.math.roundToInt

/** Turns the output of `go test -json` into a [CheckResult]. Pure, so it needs neither the IDE nor a Go toolchain. */
object GoTestJsonParser {
    const val BUILD_ERROR_NAME = "build"

    private val expectedGot = Regex("""(?:expected|want)\s+(.+?),\s*(?:but\s+)?got\s+(.+)""")
    private val gotWant = Regex("""got\s+(.+?),\s*(?:expected|want)\s+(.+)""")

    private class Event(
        val action: String,
        val test: String?,
        val output: String?,
        val elapsedSeconds: Double?,
    )

    fun parse(output: String): CheckResult {
        val events = output.lineSequence().mapNotNull(::parseEvent).toList()
        val buildErrors = buildErrors(events)
        if (buildErrors.isNotEmpty()) {
            val row = CheckResultTestsInner(BUILD_ERROR_NAME, Status.fail, got = buildErrors)
            return CheckResult(passed = false, tests = listOf(row))
        }

        val outputs = mutableMapOf<String, StringBuilder>()
        val tests = mutableListOf<CheckResultTestsInner>()
        for (event in events.filter { it.test != null }) {
            val name = event.test.orEmpty()
            val status = STATUSES[event.action]
            if (event.action == "output") {
                outputs.getOrPut(name) { StringBuilder() }.append(event.output.orEmpty())
            } else if (status != null) {
                val duration = event.elapsedSeconds?.let { (it * MILLIS).roundToInt() }
                val (expected, got) =
                    if (status == Status.fail) expectedAndGot(outputs[name].toString()) else null to null
                tests += CheckResultTestsInner(name, status, duration, expected, got)
            }
        }
        return CheckResult(passed = tests.isNotEmpty() && tests.none { it.status == Status.fail }, tests = tests)
    }

    private fun parseEvent(line: String): Event? =
        runCatching {
            val json = JsonParser.parseString(line).asJsonObject
            Event(
                action = json.get("Action").asString,
                test = json.get("Test")?.asString,
                output = json.get("Output")?.asString,
                elapsedSeconds = json.get("Elapsed")?.asDouble,
            )
        }.getOrNull()

    // Go 1.24+ reports compiler output as build-output events, without a Test field.
    private fun buildErrors(events: List<Event>): String =
        if (events.none { it.action == "build-fail" }) {
            ""
        } else {
            events
                .filter { it.action == "build-output" }
                .joinToString("") { it.output.orEmpty() }
                .lineSequence()
                .filterNot { it.startsWith("#") }
                .joinToString("\n")
                .trim()
        }

    private fun expectedAndGot(testOutput: String): Pair<String?, String?> {
        val lines = testOutput.lineSequence().map { it.trim() }
        val match =
            lines.firstNotNullOfOrNull { line ->
                expectedGot.find(line)?.let { it.groupValues[1] to it.groupValues[2] }
                    ?: gotWant.find(line)?.let { it.groupValues[2] to it.groupValues[1] }
            }
        return match ?: (null to null)
    }

    private const val MILLIS = 1000
    private val STATUSES = mapOf("pass" to Status.pass, "fail" to Status.fail)
}
