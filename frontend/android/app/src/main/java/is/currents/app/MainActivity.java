package is.currents.app;

import android.os.Bundle;

import androidx.activity.EdgeToEdge;

import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        // Required by @capacitor-community/safe-area so it can dispatch the real window
        // insets to the webview. Without it, targetSdk 36 still forces edge-to-edge on
        // Android 15+, but the system bars aren't set up transparently and the plugin
        // can't feed env(safe-area-inset-*), so content draws under the status bar.
        EdgeToEdge.enable(this);
    }
}
