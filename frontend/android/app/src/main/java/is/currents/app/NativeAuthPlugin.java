package is.currents.app;

import android.content.Intent;
import android.net.Uri;

import androidx.activity.result.ActivityResultLauncher;
import androidx.browser.auth.AuthTabIntent;
import androidx.browser.customtabs.CustomTabsClient;

import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

@CapacitorPlugin(name = "NativeAuth")
public class NativeAuthPlugin extends Plugin {
    private ActivityResultLauncher<Intent> launcher;
    private PluginCall pendingCall;

    @Override
    public void load() {
        launcher = AuthTabIntent.registerActivityResultLauncher(getActivity(), this::handleResult);
    }

    @PluginMethod
    public void isSupported(PluginCall call) {
        String provider = CustomTabsClient.getPackageName(getContext(), null);
        JSObject result = new JSObject();
        result.put("value", provider != null && CustomTabsClient.isAuthTabSupported(getContext(), provider));
        call.resolve(result);
    }

    @PluginMethod
    public void open(PluginCall call) {
        String rawUrl = call.getString("url");
        String redirectScheme = call.getString("redirectScheme");
        Uri url = rawUrl == null ? null : Uri.parse(rawUrl);
        if (url == null || !"https".equals(url.getScheme())) {
            call.reject("A valid HTTPS URL is required");
            return;
        }
        if (redirectScheme == null || redirectScheme.isEmpty()) {
            call.reject("redirectScheme is required");
            return;
        }
        if (pendingCall != null) {
            call.reject("An authentication session is already active");
            return;
        }

        pendingCall = call;
        call.setKeepAlive(true);
        new AuthTabIntent.Builder().build().launch(launcher, url, redirectScheme);
    }

    private void handleResult(AuthTabIntent.AuthResult result) {
        if (pendingCall == null) return;
        PluginCall call = pendingCall;
        pendingCall = null;
        call.setKeepAlive(false);

        if (result.resultCode == AuthTabIntent.RESULT_OK && result.resultUri != null) {
            JSObject data = new JSObject();
            data.put("url", result.resultUri.toString());
            call.resolve(data);
        } else {
            call.reject("Authorization was canceled");
        }
    }
}
