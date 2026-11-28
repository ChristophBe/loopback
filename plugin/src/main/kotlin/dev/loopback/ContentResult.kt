package dev.loopback

/** Outcome of a content request: the value, or a typed failure the UI can render. */
sealed interface ContentResult<out T> {
    data class Success<T>(
        val value: T,
    ) : ContentResult<T>

    sealed interface Failure : ContentResult<Nothing> {
        val message: String
    }

    data class NotFound(
        override val message: String = "not found",
    ) : Failure

    data class Unauthorized(
        override val message: String = "unauthorized",
    ) : Failure

    /** The server could not be reached. */
    data class Offline(
        override val message: String = "offline",
    ) : Failure

    /** Any other HTTP status or an unreadable response. */
    data class ServerError(
        val status: Int,
        override val message: String,
    ) : Failure
}
