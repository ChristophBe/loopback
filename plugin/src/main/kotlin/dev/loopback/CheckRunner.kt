package dev.loopback

import dev.loopback.api.model.CheckResult

/** Runs a lesson's checks and returns one normalized result, whichever test framework produced it. */
interface CheckRunner {
    fun run(): CheckResult
}
