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
    fun resourceName(language: String) = "/templates/template-$language.zip"

    fun install(
        language: String,
        target: Path,
    ): InstallResult {
        val stream = TemplateInstaller::class.java.getResourceAsStream(resourceName(language))
        val problem = if (stream == null) "No template bundled for language '$language'." else targetProblem(target)
        return if (stream == null || problem != null) {
            InstallResult.Refused(problem.orEmpty())
        } else {
            try {
                InstallResult.Installed(stream.use { extract(it, target) })
            } catch (e: IOException) {
                InstallResult.Refused("Could not copy the template: ${e.message}")
            }
        }
    }

    private fun targetProblem(target: Path): String? =
        when {
            Files.exists(target) && !Files.isDirectory(target) -> "$target is not a folder."
            Files.isDirectory(target) && Files.list(target).use { it.findAny().isPresent } ->
                "$target is not empty. Choose an empty folder."
            else -> null
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
