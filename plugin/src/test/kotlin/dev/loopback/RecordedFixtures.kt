package dev.loopback

import java.io.File
import kotlin.test.assertTrue

/** The recorded toolchain outputs in `api/schemas/fixtures`: one folder per supported version, see its README. */
object RecordedFixtures {
    private val root = File("../api/schemas/fixtures")

    /** The folder of every recorded version of [toolchain] (`go` or `cargo`). */
    fun versions(toolchain: String): List<File> {
        val dirs = File(root, toolchain).listFiles { f -> f.isDirectory }.orEmpty().sortedBy { it.name }
        assertTrue(dirs.size >= 2, "expected at least two $toolchain versions, found ${dirs.map { it.name }}")
        return dirs
    }

    /** Runs [check] once per recorded version of [toolchain] and names the failing version. */
    fun forEachVersion(
        toolchain: String,
        check: (File) -> Unit,
    ) {
        for (dir in versions(toolchain)) {
            try {
                check(dir)
            } catch (e: Throwable) {
                throw AssertionError("$toolchain ${dir.name}: ${e.message}", e)
            }
        }
    }
}
