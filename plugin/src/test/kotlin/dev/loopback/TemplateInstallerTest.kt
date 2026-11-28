package dev.loopback

import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertTrue

class TemplateInstallerTest {
    @Test
    fun copiesGoTemplateIntoEmptyFolder() {
        val dir = Files.createTempDirectory("loopback-go")
        val result = TemplateInstaller.install("go", dir.resolve("project"))
        assertIs<InstallResult.Installed>(result)
        assertTrue(Files.isRegularFile(dir.resolve("project/go.mod")))
        assertTrue(Files.isRegularFile(dir.resolve("project/resp/doc.go")))
    }

    @Test
    fun copiesRustTemplateIntoExistingEmptyFolder() {
        val dir = Files.createTempDirectory("loopback-rust")
        val result = TemplateInstaller.install("rust", dir)
        assertIs<InstallResult.Installed>(result)
        assertTrue(Files.isRegularFile(dir.resolve("Cargo.toml")))
        assertTrue(Files.isRegularFile(dir.resolve("src/lib.rs")))
    }

    @Test
    fun refusesNonEmptyFolderAndWritesNothing() {
        val dir = Files.createTempDirectory("loopback-busy")
        Files.writeString(dir.resolve("notes.txt"), "mine")
        val result = TemplateInstaller.install("go", dir)
        assertIs<InstallResult.Refused>(result)
        assertEquals(listOf("notes.txt"), Files.list(dir).use { s -> s.map { it.fileName.toString() }.toList() })
    }

    @Test
    fun refusesUnknownLanguage() {
        val dir = Files.createTempDirectory("loopback-none")
        assertIs<InstallResult.Refused>(TemplateInstaller.install("cobol", dir))
    }

    @Test
    fun refusesTargetThatIsAFile() {
        val file = Files.createTempFile("loopback", ".txt")
        assertIs<InstallResult.Refused>(TemplateInstaller.install("go", file))
    }
}
