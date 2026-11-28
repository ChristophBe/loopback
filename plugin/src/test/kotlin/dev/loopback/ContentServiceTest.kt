package dev.loopback

import com.sun.net.httpserver.HttpServer
import dev.loopback.api.model.Course
import dev.loopback.api.model.ExerciseType
import dev.loopback.api.model.Lesson
import java.net.InetSocketAddress
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertNull

class ContentServiceTest {
    private lateinit var server: HttpServer
    private val seen = mutableListOf<Pair<String, String?>>()
    private var status = 200
    private var body = "[]"
    private var token: String? = "tok"

    private val lessonJson =
        """{"id":7,"courseId":1,"slug":"s","order":1,"title":"T","minutes":5,"xp":10,"body":"b",
        |"ownedFiles":["a.go"],"starterFiles":{"a.go":"x"},"testFiles":{},"hints":[],"failureExplanations":{}}
        """.trimMargin()

    @BeforeTest
    fun start() {
        server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0)
        server.createContext("/") { ex ->
            seen += ex.requestURI.toString() to ex.requestHeaders.getFirst("Authorization")
            val bytes = body.toByteArray()
            ex.responseHeaders.add("Content-Type", "application/json")
            ex.sendResponseHeaders(status, bytes.size.toLong())
            ex.responseBody.use { it.write(bytes) }
        }
        server.start()
    }

    @AfterTest
    fun stop() = server.stop(0)

    private fun baseUrl() = "http://127.0.0.1:${server.address.port}/api/v1"

    private fun service(base: String = baseUrl()) = ContentService({ base }, { token })

    @Test
    fun `default base url points at the local backend`() {
        assertEquals("http://localhost:8080/api/v1", DEFAULT_API_BASE_URL)
        assertEquals("http://localhost:8080/api/v1", LoopbackSettings.State().apiBaseUrl)
    }

    @Test
    fun `lists courses with the bearer token`() {
        body = """[{"id":1,"language":"go","project":"p","title":"Go"}]"""
        val result = assertIs<ContentResult.Success<List<Course>>>(service().listCourses())
        assertEquals("Go", result.value.single().title)
        assertEquals("/api/v1/courses" to "Bearer tok", seen.single())
    }

    @Test
    fun `omits the authorization header without a token`() {
        token = null
        service().listCourses()
        assertNull(seen.single().second)
    }

    @Test
    fun `gets a lesson and lists lessons of a course`() {
        body = lessonJson
        assertEquals(7, assertIs<ContentResult.Success<Lesson>>(service().getLesson(7)).value.id)
        body = "[$lessonJson]"
        assertEquals(1, assertIs<ContentResult.Success<List<Lesson>>>(service().listLessons(1)).value.size)
        assertEquals(listOf("/api/v1/lessons/7", "/api/v1/courses/1/lessons"), seen.map { it.first })
    }

    @Test
    fun `passes exercise filters`() {
        service().listExercises(courseId = 2, type = ExerciseType.values().first())
        assertEquals("/api/v1/exercises?courseId=2&type=${ExerciseType.values().first().value}", seen.single().first)
    }

    @Test
    fun `maps 404 to not found`() {
        status = 404
        body = """{"error":"not_found","errorDescription":"no such lesson"}"""
        assertEquals(ContentResult.NotFound("no such lesson"), service().getLesson(9))
    }

    @Test
    fun `maps 401 to unauthorized`() {
        status = 401
        body = """{"error":"unauthorized"}"""
        assertIs<ContentResult.Unauthorized>(service().listCourses())
    }

    @Test
    fun `maps other statuses and bad bodies to server error`() {
        status = 500
        body = "oops"
        assertEquals(500, assertIs<ContentResult.ServerError>(service().listCourses()).status)
        status = 200
        body = "not json"
        assertIs<ContentResult.ServerError>(service().listCourses())
        body = "null"
        assertIs<ContentResult.ServerError>(service().listCourses())
    }

    @Test
    fun `maps an unreachable server and an invalid url to offline`() {
        val base = baseUrl()
        server.stop(0)
        assertIs<ContentResult.Offline>(service(base).listCourses())
        assertIs<ContentResult.Offline>(service("not a url").listCourses())
    }
}
