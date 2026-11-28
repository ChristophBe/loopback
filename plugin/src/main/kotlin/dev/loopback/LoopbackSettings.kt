package dev.loopback

import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage
import com.intellij.openapi.components.service

const val DEFAULT_API_BASE_URL = "http://localhost:8080/api/v1"

/** Application-level plugin settings, stored in loopback.xml. */
@Service(Service.Level.APP)
@State(name = "LoopbackSettings", storages = [Storage("loopback.xml")])
class LoopbackSettings : PersistentStateComponent<LoopbackSettings.State> {
    data class State(
        var apiBaseUrl: String = DEFAULT_API_BASE_URL,
    )

    private var state = State()

    var apiBaseUrl: String
        get() = state.apiBaseUrl.ifBlank { DEFAULT_API_BASE_URL }
        set(value) {
            state.apiBaseUrl = value
        }

    override fun getState(): State = state

    override fun loadState(state: State) {
        this.state = state
    }

    companion object {
        fun getInstance(): LoopbackSettings = service()
    }
}
