package dev.loopback

import com.google.gson.Gson
import dev.loopback.api.model.CheckResult
import dev.loopback.api.model.CheckResultTestsInner.Status
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class CargoTestRunnerTest {
    /** The recording folder of the cargo version under test; set by [eachVersion]. */
    private var fixtures = File(".")

    /** Runs [check] once for every recorded cargo version. */
    private fun eachVersion(check: () -> Unit) =
        RecordedFixtures.forEachVersion("cargo") { dir ->
            fixtures = dir
            check()
        }

    private fun fixture(
        case: String,
        file: String,
    ) = File(fixtures, "$case/$file").readText()

    private fun expected(case: String) = Gson().fromJson(fixture(case, "expected.json"), CheckResult::class.java)

    /** Text output has no per-test times, and lists the tests in the order the test threads finish. */
    private fun withoutDurations(result: CheckResult): CheckResult {
        val tests = result.tests.map { it.copy(durationMs = null) }.sortedBy { it.name }
        return result.copy(tests = tests)
    }

    /** Serves the recorded output of a case: the libtest files for the JSON command, the text files otherwise. */
    private class Recorded(
        private val dir: File,
    ) : ProcessExecutor {
        val commands = mutableListOf<List<String>>()

        override fun execute(
            command: List<String>,
            workDir: File,
        ): ProcessOutput {
            commands.add(command)
            val prefix = if ("--format" in command) "libtest" else "text"
            return ProcessOutput(
                File(dir, "$prefix.exit").readText().trim().toInt(),
                File(dir, "$prefix.stdout.${if (prefix == "libtest") "jsonl" else "txt"}").readText(),
                File(dir, "$prefix.stderr.txt").readText(),
            )
        }
    }

    private fun runner(
        case: String,
        preferJson: Boolean = true,
    ): Pair<CargoTestRunner, Recorded> {
        val executor = Recorded(File(fixtures, case))
        return CargoTestRunner(File("."), executor, preferJson) to executor
    }

    @Test
    fun parsesPassingLibtestJson() =
        eachVersion {
            assertEquals(expected("pass"), CargoOutputParser.parseLibtestJson(fixture("pass", "libtest.stdout.jsonl")))
        }

    @Test
    fun failingTestYieldsNameExpectedAndGotFromLibtestJson() =
        eachVersion {
            val result = CargoOutputParser.parseLibtestJson(fixture("fail", "libtest.stdout.jsonl"))!!

            assertEquals(expected("fail"), result)
            val failed = result.tests.single { it.status == Status.fail }
            assertEquals("tests::adds_negative_numbers", failed.name)
            assertEquals("-2", failed.expected)
            assertEquals("-3", failed.got)
        }

    @Test
    fun parsesPassingText() =
        eachVersion {
            val result = CargoOutputParser.parseText(fixture("pass", "text.stdout.txt"))

            assertEquals(withoutDurations(expected("pass")), withoutDurations(result!!))
        }

    @Test
    fun failingTestYieldsNameExpectedAndGotFromText() =
        eachVersion {
            val result = CargoOutputParser.parseText(fixture("fail", "text.stdout.txt"))!!

            assertFalse(result.passed)
            assertEquals(
                withoutDurations(expected("fail")).tests.sortedBy { it.name },
                result.tests.sortedBy { it.name },
            )
        }

    @Test
    fun buildErrorYieldsOneFailRowWithTheCompilerMessage() =
        eachVersion {
            val fromJsonRun = CargoOutputParser.parseBuildError(fixture("build-error", "libtest.stderr.txt"))
            val fromTextRun = CargoOutputParser.parseBuildError(fixture("build-error", "text.stderr.txt"))

            assertEquals(expected("build-error"), fromJsonRun)
            // rustc may print a different column for the same note in two runs, so only the start must match
            val row = fromTextRun!!.tests.single()
            assertFalse(fromTextRun.passed)
            assertEquals(expected("build-error").tests.single().copy(got = null), row.copy(got = null))
            assertTrue(row.got!!.startsWith("error[E0277]: cannot add `&str` to `i32`"))
        }

    @Test
    fun aFailingTestIsNoBuildError() =
        eachVersion {
            assertNull(CargoOutputParser.parseBuildError(fixture("fail", "text.stderr.txt")))
            assertNull(CargoOutputParser.parseBuildError(fixture("stable-no-json", "libtest.stderr.txt")))
        }

    @Test
    fun outputWithoutTestsIsNotParsed() {
        assertNull(CargoOutputParser.parseLibtestJson(""))
        assertNull(CargoOutputParser.parseText("running 1 test\n"))
    }

    @Test
    fun plainPanicHasNoExpectedAndGot() {
        val json =
            """{ "type": "suite", "event": "started", "test_count": 1 }
            |{ "type": "test", "name": "t", "event": "failed", "stdout": "thread 't' panicked at x.rs:1:1:\nboom\n" }
            """.trimMargin()

        val row = CargoOutputParser.parseLibtestJson(json)!!.tests.single()

        assertEquals(Status.fail, row.status)
        assertNull(row.expected)
        assertNull(row.got)
    }

    @Test
    fun assertionWithACustomMessageStillYieldsBothValues() {
        val text =
            """
            running 1 test
            test t ... FAILED

            failures:

            ---- t stdout ----

            thread 't' panicked at src/lib.rs:1:1:
            assertion `left == right` failed: sums differ
              left: "a"
             right: "b"
            note: run with `RUST_BACKTRACE=1`


            failures:
                t

            test result: FAILED. 0 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s
            """.trimIndent()

        val row = CargoOutputParser.parseText(text)!!.tests.single()

        assertEquals("\"a\"", row.expected)
        assertEquals("\"b\"", row.got)
    }

    @Test
    fun runnerPrefersLibtestJson() =
        eachVersion {
            val (runner, executor) = runner("fail")

            assertEquals(expected("fail"), runner.run())
            assertEquals(1, executor.commands.size)
        }

    @Test
    fun runnerFallsBackToTextWhenLibtestJsonNeedsNightly() =
        eachVersion {
            val json = Recorded(File(fixtures, "stable-no-json"))
            val text = Recorded(File(fixtures, "fail"))
            val executor =
                ProcessExecutor { command, dir -> (if ("--format" in command) json else text).execute(command, dir) }

            val result = CargoTestRunner(File("."), executor).run()

            assertEquals(
                withoutDurations(expected("fail")).tests.sortedBy { it.name },
                result.tests.sortedBy { it.name },
            )
            assertFalse(result.passed)
            assertEquals(1, json.commands.size)
            assertEquals(1, text.commands.size)
        }

    @Test
    fun runnerCanSkipLibtestJson() =
        eachVersion {
            val (runner, executor) = runner("pass", preferJson = false)

            assertEquals(withoutDurations(expected("pass")), withoutDurations(runner.run()))
            assertEquals(listOf(listOf("cargo", "test", "--no-fail-fast")), executor.commands)
        }

    @Test
    fun runnerReportsABuildErrorWithoutFallingBack() =
        eachVersion {
            val (runner, executor) = runner("build-error")

            assertEquals(expected("build-error"), runner.run())
            assertEquals(1, executor.commands.size)
        }

    @Test
    fun runnerReportsAnUnrecognizedFailureAsOneFailRow() {
        val executor = ProcessExecutor { _, _ -> ProcessOutput(101, "", "error: could not find `Cargo.toml`\n") }

        val result = CargoTestRunner(File("."), executor).run()

        assertFalse(result.passed)
        assertEquals(CargoOutputParser.UNKNOWN_ROW, result.tests.single().name)
        assertEquals("error: could not find `Cargo.toml`", result.tests.single().got)
    }

    @Test
    fun runnerTreatsSilentSuccessAsPassed() {
        val result = CargoTestRunner(File("."), { _, _ -> ProcessOutput(0, "", "") }).run()

        assertTrue(result.passed)
        assertTrue(result.tests.isEmpty())
    }

    @Test
    fun systemExecutorCapturesExitCodeAndStreams() {
        val java = File(System.getProperty("java.home"), "bin/java").path

        val output = SystemProcessExecutor().execute(listOf(java, "-version"), File("."))

        assertEquals(0, output.exitCode)
        assertEquals("", output.stdout)
        assertTrue(output.stderr.contains("version"))
    }
}
