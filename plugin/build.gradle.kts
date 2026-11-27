plugins {
    kotlin("jvm") version "2.4.21"
    id("org.jetbrains.intellij.platform") version "2.20.1"
    id("io.gitlab.arturbosch.detekt") version "1.23.8"
    id("org.jlleitschuh.gradle.ktlint") version "12.2.0"
}

group = "dev.loopback"
version = "0.1.0"

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    intellijPlatform {
        intellijIdeaCommunity("2024.3")
        pluginVerifier()
        testFramework(org.jetbrains.intellij.platform.gradle.TestFrameworkType.Platform)
    }
    implementation("com.google.code.gson:gson:2.15.0")
    testImplementation(kotlin("test"))
}

kotlin {
    jvmToolchain(17)
    compilerOptions.freeCompilerArgs.add("-jvm-default=no-compatibility")
}

intellijPlatform {
    pluginConfiguration {
        ideaVersion {
            sinceBuild = "243"
        }
    }
    pluginVerification {
        ides {
            create(org.jetbrains.intellij.platform.gradle.IntelliJPlatformType.IntellijIdeaCommunity, "2024.3")
        }
    }
}

detekt {
    buildUponDefaultConfig = true
    source.setFrom("src/main/kotlin", "src/test/kotlin")
}

ktlint {
    filter {
        exclude { it.file.path.contains("/src/gen/") }
    }
}

tasks.test {
    useJUnit()
}

sourceSets {
    main {
        kotlin.srcDir("src/gen")
    }
}
