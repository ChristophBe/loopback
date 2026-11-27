package dev.loopback

import dev.loopback.api.model.CheckResult
import java.io.File

/** Runs `go test -json` in [projectDir] and parses the output with [GoTestJsonParser]. */
class GoTestRunner(
    private val projectDir: File,
    private val goExecutable: String = "go",
) : CheckRunner {
    override fun run(): CheckResult {
        val process =
            ProcessBuilder(goExecutable, "test", "-json", "./...")
                .directory(projectDir)
                .redirectErrorStream(true)
                .start()
        val output = process.inputStream.bufferedReader().readText()
        process.waitFor()
        return GoTestJsonParser.parse(output)
    }
}
