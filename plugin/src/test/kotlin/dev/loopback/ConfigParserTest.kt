package dev.loopback

import kotlin.test.Test
import kotlin.test.assertEquals

class ConfigParserTest {
    @Test
    fun parsesTheConfigResponse() {
        val json = """{"version":"test","gamification":{"xpPerLesson":50,"xpPerLevel":500,"dailyGoalLessons":1}}"""

        val config = ConfigParser.parse(json)

        assertEquals("test", config.version)
        assertEquals(50, config.gamification.xpPerLesson)
    }
}
