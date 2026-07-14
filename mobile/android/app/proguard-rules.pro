# --- Capacitor / WebView bridge ---
-keep class com.getcapacitor.** { *; }
-keep class com.capacitorjs.** { *; }
-dontwarn com.getcapacitor.**

# --- Misetanibox app ---
-keep class com.misetanibox.app.** { *; }
-keep class com.misetanibox.app.**.** { *; }

# CameraX / ML Kit barcode
-keep class com.google.mlkit.** { *; }
-keep class com.google.android.gms.internal.mlkit_vision_barcode.** { *; }
-dontwarn com.google.mlkit.**
-keep class androidx.camera.** { *; }

# --- gomobile / mihomo native bridge ---
-keep class mobilecore.** { *; }
-keep class go.** { *; }
-keepclassmembers class * {
    native <methods>;
}
-keepclasseswithmembernames class * {
    native <methods>;
}

# --- Android / Kotlin ---
-keepattributes *Annotation*,Signature,InnerClasses,EnclosingMethod
-keep class kotlin.** { *; }
-keep class kotlin.Metadata { *; }
-dontwarn kotlin.**
-dontwarn org.jetbrains.annotations.**

# Keep enums
-keepclassmembers enum * {
    public static **[] values();
    public static ** valueOf(java.lang.String);
}
