import Capacitor
import Photos
import UIKit

@UIApplicationMain
class AppDelegate: UIResponder, UIApplicationDelegate {

    var window: UIWindow?

    func application(_ application: UIApplication, didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        // Override point for customization after application launch.
        return true
    }

    func applicationWillResignActive(_ application: UIApplication) {
        // Sent when the application is about to move from active to inactive state. This can occur for certain types of temporary interruptions (such as an incoming phone call or SMS message) or when the user quits the application and it begins the transition to the background state.
        // Use this method to pause ongoing tasks, disable timers, and invalidate graphics rendering callbacks. Games should use this method to pause the game.
    }

    func applicationDidEnterBackground(_ application: UIApplication) {
        // Use this method to release shared resources, save user data, invalidate timers, and store enough application state information to restore your application to its current state in case it is terminated later.
        // If your application supports background execution, this method is called instead of applicationWillTerminate: when the user quits.
    }

    func applicationWillEnterForeground(_ application: UIApplication) {
        // Called as part of the transition from the background to the active state; here you can undo many of the changes made on entering the background.
    }

    func applicationDidBecomeActive(_ application: UIApplication) {
        // Restart any tasks that were paused (or not yet started) while the application was inactive. If the application was previously in the background, optionally refresh the user interface.
    }

    func applicationWillTerminate(_ application: UIApplication) {
        // Called when the application is about to terminate. Save data if appropriate. See also applicationDidEnterBackground:.
    }

    func application(_ app: UIApplication, open url: URL, options: [UIApplication.OpenURLOptionsKey: Any] = [:]) -> Bool {
        // The OAuth deep link (currents://oauth-callback?token=...) is handled by Capacitor's
        // appUrlOpen listener (src/lib/app-init.ts) through this proxy call. Shares never reach
        // the app anymore — they're handled natively inside the Share Extension.
        return ApplicationDelegateProxy.shared.application(app, open: url, options: options)
    }

    func application(_ application: UIApplication, continue userActivity: NSUserActivity, restorationHandler: @escaping ([UIUserActivityRestoring]?) -> Void) -> Bool {
        // Called when the app was launched with an activity, including Universal Links.
        // Feel free to add additional processing here, but if you want the App API to support
        // tracking app url opens, make sure to keep this call
        return ApplicationDelegateProxy.shared.application(application, continue: userActivity, restorationHandler: restorationHandler)
    }

}

// MARK: - SharedAuth plugin

// Mirrors the session token (+ appview base URL) into the App Group container so the Share
// Extension can call the appview directly — it can't read the app's keychain entry. Registered
// on the bridge by MainViewController below; driven from src/lib/auth-storage.ts.
@objc(SharedAuthPlugin)
public class SharedAuthPlugin: CAPPlugin, CAPBridgedPlugin {
    public let identifier = "SharedAuthPlugin"
    public let jsName = "SharedAuth"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "set", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "clear", returnType: CAPPluginReturnPromise)
    ]

    private static var fileURL: URL? {
        FileManager.default
            .containerURL(forSecurityApplicationGroupIdentifier: "group.is.currents.app")?
            .appendingPathComponent("auth.json")
    }

    @objc func set(_ call: CAPPluginCall) {
        guard let token = call.getString("token"), !token.isEmpty, let url = Self.fileURL else {
            call.reject("token is required")
            return
        }
        var payload = ["token": token]
        if let apiUrl = call.getString("apiUrl"), !apiUrl.isEmpty { payload["apiUrl"] = apiUrl }
        do {
            let data = try JSONSerialization.data(withJSONObject: payload)
            try data.write(to: url, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
            call.resolve()
        } catch {
            call.reject("could not write shared auth: \(error.localizedDescription)")
        }
    }

    @objc func clear(_ call: CAPPluginCall) {
        if let url = Self.fileURL { try? FileManager.default.removeItem(at: url) }
        call.resolve()
    }
}

// MARK: - Native save actions plugin

// Browser download/clipboard APIs do not hand image files off correctly from a WebView. Keep
// those operations native so Download writes to Photos and Copy places real image data on the
// platform clipboard. Link sharing also stays native so it always opens the system share sheet.
@objc(NativeSaveActionsPlugin)
public class NativeSaveActionsPlugin: CAPPlugin, CAPBridgedPlugin {
    public let identifier = "NativeSaveActionsPlugin"
    public let jsName = "NativeSaveActions"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "download", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "copyImage", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "copyText", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "shareLink", returnType: CAPPluginReturnPromise)
    ]

    private struct DownloadedImage {
        let data: Data
        let fileExtension: String
    }

    @objc func download(_ call: CAPPluginCall) {
        fetchImage(call) { result in
            switch result {
            case .failure(let error):
                call.reject("Could not download image: \(error.localizedDescription)")
            case .success(let image):
                let fileURL = FileManager.default.temporaryDirectory
                    .appendingPathComponent(UUID().uuidString)
                    .appendingPathExtension(image.fileExtension)
                do {
                    try image.data.write(to: fileURL, options: .atomic)
                } catch {
                    call.reject("Could not prepare image: \(error.localizedDescription)")
                    return
                }
                PHPhotoLibrary.requestAuthorization(for: .addOnly) { status in
                    guard status == .authorized || status == .limited else {
                        try? FileManager.default.removeItem(at: fileURL)
                        call.reject("Photo library permission was denied")
                        return
                    }
                    PHPhotoLibrary.shared().performChanges({
                        PHAssetChangeRequest.creationRequestForAssetFromImage(atFileURL: fileURL)
                    }) { saved, error in
                        try? FileManager.default.removeItem(at: fileURL)
                        if saved {
                            call.resolve()
                        } else {
                            call.reject("Could not save image: \(error?.localizedDescription ?? "Unknown error")")
                        }
                    }
                }
            }
        }
    }

    @objc func copyImage(_ call: CAPPluginCall) {
        fetchImage(call) { result in
            switch result {
            case .failure(let error):
                call.reject("Could not download image: \(error.localizedDescription)")
            case .success(let downloaded):
                guard let image = UIImage(data: downloaded.data) else {
                    call.reject("Downloaded file is not a supported image")
                    return
                }
                DispatchQueue.main.async {
                    UIPasteboard.general.image = image
                    call.resolve()
                }
            }
        }
    }

    @objc func copyText(_ call: CAPPluginCall) {
        guard let text = call.getString("text") else {
            call.reject("text is required")
            return
        }
        DispatchQueue.main.async {
            UIPasteboard.general.string = text
            call.resolve()
        }
    }

    @objc func shareLink(_ call: CAPPluginCall) {
        guard let value = call.getString("url"), let url = URL(string: value),
              url.scheme == "http" || url.scheme == "https" else {
            call.reject("A valid URL is required")
            return
        }
        DispatchQueue.main.async {
            guard let presenter = self.bridge?.viewController else {
                call.reject("Could not open share sheet")
                return
            }
            let sheet = UIActivityViewController(activityItems: [url], applicationActivities: nil)
            if let popover = sheet.popoverPresentationController {
                popover.sourceView = presenter.view
                popover.sourceRect = CGRect(x: presenter.view.bounds.midX, y: presenter.view.bounds.maxY, width: 0, height: 0)
                popover.permittedArrowDirections = []
            }
            sheet.completionWithItemsHandler = { _, _, _, _ in call.resolve() }
            presenter.present(sheet, animated: true)
        }
    }

    private func fetchImage(
        _ call: CAPPluginCall,
        completion: @escaping (Result<DownloadedImage, Error>) -> Void
    ) {
        guard let value = call.getString("url"), let url = URL(string: value),
              url.scheme == "http" || url.scheme == "https" else {
            completion(.failure(NativeSaveActionError.invalidURL))
            return
        }
        URLSession.shared.dataTask(with: url) { data, response, error in
            if let error {
                completion(.failure(error))
                return
            }
            guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode),
                  let data, !data.isEmpty else {
                completion(.failure(NativeSaveActionError.downloadFailed))
                return
            }
            let mime = response?.mimeType?.lowercased() ?? ""
            guard mime.hasPrefix("image/") else {
                completion(.failure(NativeSaveActionError.notImage))
                return
            }
            completion(.success(DownloadedImage(data: data, fileExtension: Self.extensionForMime(mime))))
        }.resume()
    }

    private static func extensionForMime(_ mime: String) -> String {
        switch mime {
        case "image/jpeg": return "jpg"
        case "image/png": return "png"
        case "image/gif": return "gif"
        case "image/webp": return "webp"
        case "image/heic", "image/heif": return "heic"
        case "image/avif": return "avif"
        default: return "jpg"
        }
    }

    private enum NativeSaveActionError: LocalizedError {
        case invalidURL
        case downloadFailed
        case notImage

        var errorDescription: String? {
            switch self {
            case .invalidURL: return "A valid image URL is required"
            case .downloadFailed: return "The image request failed"
            case .notImage: return "The response is not an image"
            }
        }
    }
}

// The storyboard's root view controller (see Base.lproj/Main.storyboard) — the documented spot
// to register locally defined Capacitor plugins with the bridge.
@objc(MainViewController)
class MainViewController: CAPBridgeViewController {
    override open func capacitorDidLoad() {
        bridge?.registerPluginInstance(SharedAuthPlugin())
        bridge?.registerPluginInstance(NativeSaveActionsPlugin())
        // Enable the native edge-swipe-back gesture. It drives the WKWebView's back/forward
        // list, which SvelteKit's client-side navigations populate via the History API.
        webView?.allowsBackForwardNavigationGestures = true
    }
}
