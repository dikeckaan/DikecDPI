// Test helper only (not shipped): an app that is NOT excluded from the VPN,
// used to check from inside the emulator that blocked sites open through it.
plugins { id("com.android.application") }

android {
    namespace = "com.dikec.dpi.probe"
    compileSdk { version = release(37) { minorApiLevel = 2 } }
    defaultConfig {
        applicationId = "com.dikec.dpi.probe"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "1"
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}
