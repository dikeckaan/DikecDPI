plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.plugin.compose")
}

// Version comes from the repository's VERSION file, like the desktop builds.
val appVersion = rootProject.file("../VERSION").readText().trim()
val versionParts = appVersion.split(".").map { it.toIntOrNull() ?: 0 }
val appVersionCode = versionParts[0] * 10000 + versionParts.getOrElse(1) { 0 } * 100 + versionParts.getOrElse(2) { 0 }

android {
    namespace = "com.dikec.dpi"
    compileSdk {
        version = release(37) { minorApiLevel = 2 }
    }

    defaultConfig {
        applicationId = "com.dikec.dpi"
        minSdk = 26
        targetSdk = 36
        versionCode = appVersionCode
        versionName = appVersion
    }

    signingConfigs {
        // Release signing comes from the environment (CI secret); without it
        // the release build is signed with the debug key so it still installs.
        create("release") {
            val ks = System.getenv("DIKEC_KEYSTORE")
            if (ks != null && file(ks).exists()) {
                storeFile = file(ks)
                storePassword = System.getenv("DIKEC_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("DIKEC_KEY_ALIAS")
                keyPassword = System.getenv("DIKEC_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = if (System.getenv("DIKEC_KEYSTORE") != null)
                signingConfigs.getByName("release") else signingConfigs.getByName("debug")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    packaging {
        jniLibs { useLegacyPackaging = false }
    }
}

dependencies {
    implementation(files("libs/dikecdpi.aar"))
    implementation(platform("androidx.compose:compose-bom:2026.11.01"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.foundation:foundation")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.activity:activity-compose:1.13.0")
    implementation("androidx.core:core-ktx:1.19.1")
}
