plugins {
    id("com.android.application")
}

android {
    namespace = "com.frozenfortress.companion"
    compileSdk = 37

    defaultConfig {
        applicationId = "com.frozenfortress.companion"
        minSdk = 26
        targetSdk = 37
        versionCode = 1
        versionName = "0.1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }

    // With AGP 9's built-in Kotlin support (no separate kotlin-android plugin
    // applied), this also sets the Kotlin compiler's jvmTarget - there's no
    // standalone kotlinOptions block anymore.
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.19.0")
    implementation("androidx.appcompat:appcompat:1.8.0")
    implementation("com.google.android.material:material:1.14.0")

    // ML Kit Document Scanner (Play Services-backed - see AGENTS/limitations note in
    // TofuTrustManager.kt sibling docs: unavailable on de-Googled devices).
    implementation("com.google.android.gms:play-services-mlkit-document-scanner:16.0.0")

    // TOFU cert pinning + scan upload use a custom X509TrustManager on top of OkHttp,
    // since OkHttp's own CertificatePinner requires the pin in advance.
    implementation("com.squareup.okhttp3:okhttp:5.5.0")

    // EncryptedSharedPreferences for persisting the TOFU-pinned cert hash per host.
    implementation("androidx.security:security-crypto:1.1.0")

    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.11.0")

    testImplementation("junit:junit:4.13.2")
    androidTestImplementation("androidx.test.ext:junit:1.3.0")
}
