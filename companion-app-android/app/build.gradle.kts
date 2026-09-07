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

    // Stable release signing identity (YETI-62), sourced from environment variables so
    // the keystore itself is never committed - CI decodes it from a GitHub Actions
    // secret into a temp file and points RELEASE_KEYSTORE_PATH at it (see
    // android-release.yml). A local `assembleRelease` without those variables set
    // falls back to the debug keystore below, so the module still builds for anyone
    // who doesn't have (and doesn't need) the release signing material.
    val releaseKeystorePath = System.getenv("RELEASE_KEYSTORE_PATH")
    val releaseKeystorePassword = System.getenv("RELEASE_KEYSTORE_PASSWORD")
    val releaseKeyAlias = System.getenv("RELEASE_KEY_ALIAS")
    val hasReleaseSigningEnv = !releaseKeystorePath.isNullOrBlank() &&
        !releaseKeystorePassword.isNullOrBlank() &&
        !releaseKeyAlias.isNullOrBlank()

    signingConfigs {
        if (hasReleaseSigningEnv) {
            create("release") {
                storeFile = file(releaseKeystorePath!!)
                storePassword = releaseKeystorePassword
                keyAlias = releaseKeyAlias
                // PKCS12 keystores (keytool's default since JDK 9) use one password
                // for both the store and every key in it - see YETI-62 PR notes.
                keyPassword = releaseKeystorePassword
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = if (hasReleaseSigningEnv) {
                signingConfigs.getByName("release")
            } else {
                // No release signing material available locally - fall back to the
                // debug keystore so the APK is still installable, just not with a
                // signature that matches CI-published releases.
                signingConfigs.getByName("debug")
            }
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
