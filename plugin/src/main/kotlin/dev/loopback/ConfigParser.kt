package dev.loopback

import com.google.gson.Gson
import dev.loopback.api.model.Config

object ConfigParser {
    private val gson = Gson()

    fun parse(json: String): Config = gson.fromJson(json, Config::class.java)
}
