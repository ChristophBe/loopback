package dev.loopback

import java.io.IOException
import java.io.InputStream
import java.nio.file.Files
import java.nio.file.Path
import java.util.zip.ZipInputStream

sealed interface InstallResult {
    data class Installed(
        val files: Int,
    ) : InstallResult

    data class Refused(
        val message: String,
    ) : InstallResult
}

/** Copies the template repo bundled in the plugin resources into a learner project folder. */
object TemplateInstaller {
    fun resourceName(language: String) = "/templates/template-${language.filter { it.isLetterOrDigit() }}.zip"

    fun install(
        language: String,
        target: Path,
    ): InstallResult {
        val stream = TemplateInstaller::class.java.getResourceAsStream(resourceName(language))
        return stream.use {
            val problem = if (it == null) "No template bundled for language '$language'." else targetProblem(target)
            if (it == null || problem != null) {
                InstallResult.Refused(problem.orEmpty())
            } else {
                extractSafely(it, target)
            }
        }
    }

    private fun extractSafely(
        input: InputStream,
        target: Path,
    ): InstallResult =
        try {
            InstallResult.Installed(extract(input, target))
        } catch (e: IOException) {
            InstallResult.Refused("Could not copy the template: ${e.message}")
        }

    private fun targetProblem(target: Path): String? =
        try {
            when {
                Files.exists(target) && !Files.isDirectory(target) -> "$target is not a folder."
                Files.isDirectory(target) && Files.list(target).use { it.findAny().isPresent } ->
                    "$target is not empty. Choose an empty folder."
                else -> null
            }
        } catch (e: IOException) {
            "Cannot read $target: ${e.message}"
        }

    private fun extract(
        input: InputStream,
        target: Path,
    ): Int {
        Files.createDirectories(target)
        val root = target.toAbsolutePath().normalize()
        var count = 0
        val zip = ZipInputStream(input)
        for (entry in generateSequence { zip.nextEntry }) {
            val dest = root.resolve(entry.name).normalize()
            if (!dest.startsWith(root)) throw IOException("Unsafe entry ${entry.name}")
            if (entry.isDirectory) {
                Files.createDirectories(dest)
            } else {
                Files.createDirectories(dest.parent)
                Files.copy(zip, dest)
                count++
            }
        }
        return count
    }
}
