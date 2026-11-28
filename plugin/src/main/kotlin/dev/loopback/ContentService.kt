package dev.loopback

import com.google.gson.Gson
import com.google.gson.JsonParseException
import com.google.gson.reflect.TypeToken
import dev.loopback.api.model.Course
import dev.loopback.api.model.Exercise
import dev.loopback.api.model.ExerciseType
import dev.loopback.api.model.Lesson
import java.io.IOException
import java.net.URI
import java.net.URLEncoder
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.Duration

/** Supplies the bearer token for API calls; null sends the request unauthenticated. */
fun interface TokenProvider {
    fun token(): String?
}

/** Reads courses, lessons and exercises from the loopback API using the generated models. */
class ContentService(
    private val baseUrl: () -> String,
    private val tokens: TokenProvider,
    private val http: HttpClient = HttpClient.newHttpClient(),
) {
    private val gson = Gson()

    fun listCourses(): ContentResult<List<Course>> = get<List<Course>>("/courses")

    fun listLessons(courseId: Int): ContentResult<List<Lesson>> = get<List<Lesson>>("/courses/$courseId/lessons")

    fun getLesson(id: Int): ContentResult<Lesson> = get<Lesson>("/lessons/$id")

    fun listExercises(
        courseId: Int? = null,
        type: ExerciseType? = null,
    ): ContentResult<List<Exercise>> {
        val query =
            listOfNotNull(
                courseId?.let { "courseId=$it" },
                type?.let { "type=" + URLEncoder.encode(it.value, Charsets.UTF_8) },
            ).joinToString("&")
        val path = if (query.isEmpty()) "/exercises" else "/exercises?$query"
        return get<List<Exercise>>(path)
    }

    private inline fun <reified T> get(path: String): ContentResult<T> = get(path, object : TypeToken<T>() {})

    private fun <T> get(
        path: String,
        type: TypeToken<T>,
    ): ContentResult<T> {
        val response = send(path)
        val failure = response as? ContentResult.Failure
        val http = response as? ContentResult.Success
        return when {
            failure != null -> failure
            http != null -> map(http.value, type)
            else -> ContentResult.Offline()
        }
    }

    private fun send(path: String): ContentResult<HttpResponse<String>> =
        try {
            val request =
                HttpRequest
                    .newBuilder(URI.create(baseUrl().trimEnd('/') + path))
                    .timeout(TIMEOUT)
                    .header("Accept", "application/json")
                    .apply { tokens.token()?.let { header("Authorization", "Bearer $it") } }
                    .GET()
                    .build()
            ContentResult.Success(http.send(request, HttpResponse.BodyHandlers.ofString()))
        } catch (e: IllegalArgumentException) {
            ContentResult.Offline("invalid base url: ${e.message}")
        } catch (e: IOException) {
            ContentResult.Offline(e.message ?: "offline")
        } catch (e: InterruptedException) {
            Thread.currentThread().interrupt()
            ContentResult.Offline(e.message ?: "interrupted")
        }

    private fun <T> map(
        response: HttpResponse<String>,
        type: TypeToken<T>,
    ): ContentResult<T> =
        when (val status = response.statusCode()) {
            HTTP_OK -> parse(response.body(), type)
            HTTP_UNAUTHORIZED -> ContentResult.Unauthorized(errorMessage(response.body(), "unauthorized"))
            HTTP_NOT_FOUND -> ContentResult.NotFound(errorMessage(response.body(), "not found"))
            else -> ContentResult.ServerError(status, errorMessage(response.body(), "HTTP $status"))
        }

    private fun <T> parse(
        body: String,
        type: TypeToken<T>,
    ): ContentResult<T> =
        try {
            val value: T? = gson.fromJson(body, type.type)
            if (value == null) {
                ContentResult.ServerError(HTTP_OK, "empty response")
            } else {
                ContentResult.Success(value)
            }
        } catch (e: JsonParseException) {
            ContentResult.ServerError(HTTP_OK, "unreadable response: ${e.message}")
        }

    private fun errorMessage(
        body: String,
        fallback: String,
    ): String =
        try {
            gson.fromJson(body, dev.loopback.api.model.Error::class.java)?.errorDescription ?: fallback
        } catch (_: JsonParseException) {
            fallback
        }

    private companion object {
        const val HTTP_OK = 200
        const val HTTP_UNAUTHORIZED = 401
        const val HTTP_NOT_FOUND = 404
        val TIMEOUT: Duration = Duration.ofSeconds(10)
    }
}
