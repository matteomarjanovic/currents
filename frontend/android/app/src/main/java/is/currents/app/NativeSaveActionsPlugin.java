package is.currents.app;

import android.Manifest;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.ContentResolver;
import android.content.ContentValues;
import android.content.Context;
import android.content.Intent;
import android.media.MediaScannerConnection;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.provider.MediaStore;
import android.webkit.MimeTypeMap;

import androidx.core.content.FileProvider;

import com.getcapacitor.PermissionState;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import com.getcapacitor.annotation.Permission;
import com.getcapacitor.annotation.PermissionCallback;

import java.io.BufferedInputStream;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.net.URLConnection;
import java.util.Locale;
import java.util.UUID;

@CapacitorPlugin(
    name = "NativeSaveActions",
    permissions = @Permission(strings = Manifest.permission.WRITE_EXTERNAL_STORAGE, alias = "storage")
)
public class NativeSaveActionsPlugin extends Plugin {
    @PluginMethod
    public void download(PluginCall call) {
        if (Build.VERSION.SDK_INT <= Build.VERSION_CODES.P && getPermissionState("storage") != PermissionState.GRANTED) {
            requestPermissionForAlias("storage", call, "downloadPermissionCallback");
            return;
        }
        saveToGallery(call);
    }

    @PermissionCallback
    public void downloadPermissionCallback(PluginCall call) {
        if (getPermissionState("storage") == PermissionState.GRANTED) {
            saveToGallery(call);
        } else {
            call.reject("Photo storage permission was denied");
        }
    }

    @PluginMethod
    public void copyImage(PluginCall call) {
        execute(() -> {
            try {
                DownloadedImage image = fetchImage(call);
                File directory = new File(getContext().getCacheDir(), "shared-images");
                if (!directory.exists() && !directory.mkdirs()) throw new IOException("Could not create cache directory");
                File file = new File(directory, UUID.randomUUID() + "." + image.extension);
                try (OutputStream output = new FileOutputStream(file)) {
                    output.write(image.data);
                }
                Uri uri = FileProvider.getUriForFile(
                    getContext(),
                    getContext().getPackageName() + ".fileprovider",
                    file
                );
                getBridge().executeOnMainThread(() -> {
                    ClipboardManager clipboard = (ClipboardManager) getContext().getSystemService(Context.CLIPBOARD_SERVICE);
                    clipboard.setPrimaryClip(ClipData.newUri(getContext().getContentResolver(), "Currents image", uri));
                    call.resolve();
                });
            } catch (Exception error) {
                call.reject("Could not copy image", error);
            }
        });
    }

    @PluginMethod
    public void copyText(PluginCall call) {
        String text = call.getString("text");
        if (text == null) {
            call.reject("text is required");
            return;
        }
        getBridge().executeOnMainThread(() -> {
            ClipboardManager clipboard = (ClipboardManager) getContext().getSystemService(Context.CLIPBOARD_SERVICE);
            clipboard.setPrimaryClip(ClipData.newPlainText("Currents link", text));
            call.resolve();
        });
    }

    @PluginMethod
    public void shareLink(PluginCall call) {
        String url = call.getString("url");
        if (!isHttpUrl(url)) {
            call.reject("A valid URL is required");
            return;
        }
        Intent send = new Intent(Intent.ACTION_SEND);
        send.setType("text/plain");
        send.putExtra(Intent.EXTRA_TEXT, url);
        Intent chooser = Intent.createChooser(send, null);
        getBridge().executeOnMainThread(() -> {
            getActivity().startActivity(chooser);
            call.resolve();
        });
    }

    private void saveToGallery(PluginCall call) {
        execute(() -> {
            Uri inserted = null;
            try {
                DownloadedImage image = fetchImage(call);
                String fileName = fileName(call, image.extension);
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    ContentResolver resolver = getContext().getContentResolver();
                    ContentValues values = new ContentValues();
                    values.put(MediaStore.Images.Media.DISPLAY_NAME, fileName);
                    values.put(MediaStore.Images.Media.MIME_TYPE, image.mimeType);
                    values.put(
                        MediaStore.Images.Media.RELATIVE_PATH,
                        Environment.DIRECTORY_PICTURES + File.separator + "Currents"
                    );
                    values.put(MediaStore.Images.Media.IS_PENDING, 1);
                    inserted = resolver.insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, values);
                    if (inserted == null) throw new IOException("Could not create gallery item");
                    try (OutputStream output = resolver.openOutputStream(inserted)) {
                        if (output == null) throw new IOException("Could not open gallery item");
                        output.write(image.data);
                    }
                    values.clear();
                    values.put(MediaStore.Images.Media.IS_PENDING, 0);
                    resolver.update(inserted, values, null, null);
                } else {
                    File directory = new File(
                        Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_PICTURES),
                        "Currents"
                    );
                    if (!directory.exists() && !directory.mkdirs()) throw new IOException("Could not create Pictures directory");
                    File file = new File(directory, fileName);
                    try (OutputStream output = new FileOutputStream(file)) {
                        output.write(image.data);
                    }
                    MediaScannerConnection.scanFile(
                        getContext(),
                        new String[] { file.getAbsolutePath() },
                        new String[] { image.mimeType },
                        null
                    );
                }
                call.resolve();
            } catch (Exception error) {
                if (inserted != null) getContext().getContentResolver().delete(inserted, null, null);
                call.reject("Could not save image", error);
            }
        });
    }

    private DownloadedImage fetchImage(PluginCall call) throws IOException {
        String urlString = call.getString("url");
        if (!isHttpUrl(urlString)) throw new IOException("A valid image URL is required");
        HttpURLConnection connection = (HttpURLConnection) new URL(urlString).openConnection();
        connection.setConnectTimeout(30_000);
        connection.setReadTimeout(30_000);
        connection.setRequestProperty("Accept", "image/*");
        try {
            int status = connection.getResponseCode();
            if (status < 200 || status >= 300) throw new IOException("Image request failed with status " + status);
            try (BufferedInputStream input = new BufferedInputStream(connection.getInputStream())) {
                input.mark(32);
                String guessedMime = URLConnection.guessContentTypeFromStream(input);
                input.reset();
                String mimeType = normalizeMime(connection.getContentType(), guessedMime);
                ByteArrayOutputStream output = new ByteArrayOutputStream();
                byte[] buffer = new byte[16_384];
                int read;
                while ((read = input.read(buffer)) != -1) output.write(buffer, 0, read);
                return new DownloadedImage(output.toByteArray(), mimeType, extensionForMime(mimeType));
            }
        } finally {
            connection.disconnect();
        }
    }

    private static String normalizeMime(String header, String guessed) throws IOException {
        String mime = header == null ? "" : header.split(";", 2)[0].trim().toLowerCase(Locale.ROOT);
        if (!mime.startsWith("image/") && guessed != null) mime = guessed.toLowerCase(Locale.ROOT);
        if (!mime.startsWith("image/")) throw new IOException("Response is not an image");
        return mime;
    }

    private static String extensionForMime(String mimeType) {
        if ("image/jpeg".equals(mimeType)) return "jpg";
        String extension = MimeTypeMap.getSingleton().getExtensionFromMimeType(mimeType);
        return extension == null || extension.isEmpty() ? "jpg" : extension;
    }

    private static String fileName(PluginCall call, String extension) {
        String base = call.getString("fileName", "currents-image").replaceAll("[^A-Za-z0-9._-]", "-");
        if (base.isEmpty()) base = "currents-image";
        return base + "." + extension;
    }

    private static boolean isHttpUrl(String value) {
        if (value == null) return false;
        Uri uri = Uri.parse(value);
        return "http".equalsIgnoreCase(uri.getScheme()) || "https".equalsIgnoreCase(uri.getScheme());
    }

    private static final class DownloadedImage {
        final byte[] data;
        final String mimeType;
        final String extension;

        DownloadedImage(byte[] data, String mimeType, String extension) {
            this.data = data;
            this.mimeType = mimeType;
            this.extension = extension;
        }
    }
}
