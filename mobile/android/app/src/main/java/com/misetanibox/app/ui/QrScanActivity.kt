package com.misetanibox.app.ui

import android.Manifest
import android.app.Activity
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Bundle
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.core.content.ContextCompat
import com.misetanibox.app.R
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Built-in QR / barcode scanner (CameraX + ML Kit).
 * Returns scanned text via [EXTRA_RESULT].
 */
class QrScanActivity : AppCompatActivity() {

    private val done = AtomicBoolean(false)
    private val analysisExecutor = Executors.newSingleThreadExecutor()
    private var cameraProvider: ProcessCameraProvider? = null

    private val permissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) startCamera()
        else {
            Toast.makeText(this, "Нужен доступ к камере", Toast.LENGTH_SHORT).show()
            setResult(Activity.RESULT_CANCELED)
            finish()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_qr_scan)
        findViewById<TextView>(R.id.qr_cancel).setOnClickListener {
            setResult(Activity.RESULT_CANCELED)
            finish()
        }
        if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA)
            == PackageManager.PERMISSION_GRANTED
        ) {
            startCamera()
        } else {
            permissionLauncher.launch(Manifest.permission.CAMERA)
        }
    }

    private fun startCamera() {
        val previewView = findViewById<PreviewView>(R.id.qr_preview)
        val future = ProcessCameraProvider.getInstance(this)
        future.addListener({
            try {
                val provider = future.get()
                cameraProvider = provider
                bindCamera(provider, previewView)
            } catch (e: Exception) {
                Toast.makeText(this, "Камера: ${e.message}", Toast.LENGTH_LONG).show()
                setResult(Activity.RESULT_CANCELED)
                finish()
            }
        }, ContextCompat.getMainExecutor(this))
    }

    private fun bindCamera(provider: ProcessCameraProvider, previewView: PreviewView) {
        provider.unbindAll()
        val preview = Preview.Builder().build().also {
            it.surfaceProvider = previewView.surfaceProvider
        }
        val options = BarcodeScannerOptions.Builder()
            .setBarcodeFormats(
                Barcode.FORMAT_QR_CODE,
                Barcode.FORMAT_AZTEC,
                Barcode.FORMAT_DATA_MATRIX,
            )
            .build()
        val scanner = BarcodeScanning.getClient(options)
        val analysis = ImageAnalysis.Builder()
            .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
            .build()
        analysis.setAnalyzer(analysisExecutor) { imageProxy ->
            if (done.get()) {
                imageProxy.close()
                return@setAnalyzer
            }
            val media = imageProxy.image
            if (media == null) {
                imageProxy.close()
                return@setAnalyzer
            }
            val image = InputImage.fromMediaImage(media, imageProxy.imageInfo.rotationDegrees)
            scanner.process(image)
                .addOnSuccessListener { barcodes ->
                    val text = barcodes.firstOrNull { !it.rawValue.isNullOrBlank() }?.rawValue
                    if (!text.isNullOrBlank() && done.compareAndSet(false, true)) {
                        runOnUiThread {
                            val data = Intent().putExtra(EXTRA_RESULT, text)
                            setResult(Activity.RESULT_OK, data)
                            finish()
                        }
                    }
                }
                .addOnCompleteListener {
                    imageProxy.close()
                }
        }
        try {
            provider.bindToLifecycle(
                this,
                CameraSelector.DEFAULT_BACK_CAMERA,
                preview,
                analysis,
            )
        } catch (_: Exception) {
            // Some devices: try front as last resort
            try {
                provider.bindToLifecycle(
                    this,
                    CameraSelector.DEFAULT_FRONT_CAMERA,
                    preview,
                    analysis,
                )
            } catch (e2: Exception) {
                Toast.makeText(this, "Не удалось открыть камеру", Toast.LENGTH_LONG).show()
                finish()
            }
        }
    }

    override fun onDestroy() {
        try {
            cameraProvider?.unbindAll()
        } catch (_: Exception) {
        }
        analysisExecutor.shutdown()
        super.onDestroy()
    }

    companion object {
        const val EXTRA_RESULT = "qr_result"
    }
}
