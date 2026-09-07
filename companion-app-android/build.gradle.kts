// Top-level build file. Plugin versions are declared here (applied `false`) and
// pinned exactly per AGENTS.md's dependency-pinning rule - no version ranges.
//
// No separate Kotlin Android plugin: AGP 9's built-in Kotlin support compiles
// Kotlin sources directly (see android.builtInKotlin in gradle.properties).
plugins {
    id("com.android.application") version "9.1.1" apply false
}
