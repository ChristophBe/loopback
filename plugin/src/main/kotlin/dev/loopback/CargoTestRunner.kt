package dev.loopback

import dev.loopback.api.model.CheckResult
import dev.loopback.api.model.CheckResultTestsInner
import dev.loopback.api.model.CheckResultTestsInner.Status
import java.io.File
import java.util.concurrent.CompletableFuture
import java.util.concurrent.TimeUnit

/** Exit code and output of a finished process. */
data class ProcessOutput(
    val exitCode: Int,
    val stdout: String,
    val stderr: String,
)

/** Seam between [CargoTestRunner] and the operating system, so the runner can be tested with canned output. */
fun interface ProcessExecutor {
    fun execute(
        command: List<String>,
        workDir: File,
    ): ProcessOutput
}

/** Runs the command as a child process and waits at most [timeoutSeconds]. */
class SystemProcessExecutor(
    private val timeoutSeconds: Long = DEFAULT_TIMEOUT_SECONDS,
) : ProcessExecutor {
    override fun execute(
        command: List<String>,
        workDir: File,
    ): ProcessOutput {
        val process = ProcessBuilder(command).directory(workDir).start()
        process.outputStream.close()
        val stderr = CompletableFuture.supplyAsync { process.errorStream.bufferedReader().readText() }
        val stdout = CompletableFuture.supplyAsync { process.inputStream.bufferedReader().readText() }
        if (!process.waitFor(timeoutSeconds, TimeUnit.SECONDS)) {
            process.destroyForcibly()
            return ProcessOutput(TIMEOUT_EXIT_CODE, "", "timed out after $timeoutSeconds seconds")
        }
        return ProcessOutput(process.exitValue(), stdout.get(), stderr.get())
    }

    private companion object {
        const val DEFAULT_TIMEOUT_SECONDS = 300L
        const val TIMEOUT_EXIT_CODE = -1
    }
}

/**
 * Runs `cargo test` in [projectDir] and normalizes the result. Order of preference:
 *
 * 1. libtest JSON (`-- -Z unstable-options --format json`): structured and exact, but accepted by nightly only.
 * 2. Plain text output of `cargo test`, which every toolchain produces. This is also the fallback when the learner
 *    has no nightly toolchain.
 *
 * cargo-nextest (`--message-format libtest-json`) is not used: it is a separate install and the stable text output
 * already covers the learners without nightly. A compile error is recognized in both modes and reported as a single
 * `fail` row.
 */
class CargoTestRunner(
    private val projectDir: File,
    private val executor: ProcessExecutor = SystemProcessExecutor(),
    private val preferLibtestJson: Boolean = true,
) : CheckRunner {
    override fun run(): CheckResult = (if (preferLibtestJson) runLibtestJson() else null) ?: runText()

    /** Null when the toolchain rejects the libtest JSON flags, so that the caller falls back to the text output. */
    private fun runLibtestJson(): CheckResult? {
        val output = executor.execute(JSON_COMMAND, projectDir)
        val result =
            CargoOutputParser.parseBuildError(output.stderr) ?: CargoOutputParser.parseLibtestJson(output.stdout)
        return result ?: if (NIGHTLY_ONLY in output.stderr) null else unrecognized(output)
    }

    private fun runText(): CheckResult {
        val output = executor.execute(TEXT_COMMAND, projectDir)
        val result = CargoOutputParser.parseBuildError(output.stderr) ?: CargoOutputParser.parseText(output.stdout)
        return result ?: unrecognized(output)
    }

    /** A cargo failure that is neither a compile error nor a test report, for example a missing `Cargo.toml`. */
    private fun unrecognized(output: ProcessOutput): CheckResult {
        if (output.exitCode == 0) return CheckResult(true, emptyList())
        val message = output.stderr.trim().ifEmpty { "cargo exited with code ${output.exitCode}" }
        val row = CheckResultTestsInner(CargoOutputParser.UNKNOWN_ROW, Status.fail, got = message)
        return CheckResult(false, listOf(row))
    }

    private companion object {
        const val NIGHTLY_ONLY = "only accepted on the nightly compiler"
        val TEXT_COMMAND = listOf("cargo", "test", "--no-fail-fast")
        val JSON_COMMAND =
            TEXT_COMMAND + listOf("--quiet", "--", "-Z", "unstable-options", "--format", "json", "--report-time")
    }
}
