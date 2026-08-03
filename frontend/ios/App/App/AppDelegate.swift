import UIKit
import Capacitor
// The send-intent SPM library product is "SendIntent" but its module/target is "SendIntentPlugin".
import SendIntentPlugin

@UIApplicationMain
class AppDelegate: UIResponder, UIApplicationDelegate {

    var window: UIWindow?

    let shareStore = ShareStore.store

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
        consumePendingShare()
    }

    // Content from the Share Extension. iOS won't reliably let the extension open this app (see
    // ShareViewController.openHostApp), so the share is persisted as a manifest in the App Group
    // container and picked up here on every foreground: immediately when the wake-up open works,
    // otherwise the next time the user opens the app. Deleting before parsing makes it one-shot.
    private func consumePendingShare() {
        let fm = FileManager.default
        guard let dir = fm.containerURL(forSecurityApplicationGroupIdentifier: "group.is.currents.app") else { return }
        let manifest = dir.appendingPathComponent("pending-share.json")
        guard let data = try? Data(contentsOf: manifest) else { return }
        try? fm.removeItem(at: manifest)
        guard let items = try? JSONSerialization.jsonObject(with: data) as? [[String: String]],
            !items.isEmpty else { return }
        NSLog("Currents: consuming pending share with \(items.count) item(s)")
        shareStore.shareItems.removeAll()
        for entry in items {
            var item = JSObject()
            item["title"] = entry["title"] ?? ""
            item["description"] = ""
            item["type"] = entry["type"] ?? ""
            item["url"] = entry["url"] ?? ""
            shareStore.shareItems.append(item)
        }
        shareStore.processed = false
        NotificationCenter.default.post(name: Notification.Name("triggerSendIntent"), object: nil)
    }

    func applicationWillTerminate(_ application: UIApplication) {
        // Called when the application is about to terminate. Save data if appropriate. See also applicationDidEnterBackground:.
    }

    func application(_ app: UIApplication, open url: URL, options: [UIApplication.OpenURLOptionsKey: Any] = [:]) -> Bool {
        // Called when the app was launched with a url. Feel free to add additional processing here,
        // but if you want the App API to support tracking app url opens, make sure to keep this call
        let handled = ApplicationDelegateProxy.shared.application(app, open: url, options: options)

        // TEMP diagnostics (app process → visible in Xcode's console). Remove once share works.
        // Share data no longer arrives via this URL — see consumePendingShare. The OAuth deep
        // link (currents://oauth-callback?token=...) is handled by Capacitor's appUrlOpen
        // (src/lib/app-init.ts) through the proxy call above.
        NSLog("Currents: application open url = \(url.absoluteString)")

        return handled
    }

    func application(_ application: UIApplication, continue userActivity: NSUserActivity, restorationHandler: @escaping ([UIUserActivityRestoring]?) -> Void) -> Bool {
        // Called when the app was launched with an activity, including Universal Links.
        // Feel free to add additional processing here, but if you want the App API to support
        // tracking app url opens, make sure to keep this call
        return ApplicationDelegateProxy.shared.application(application, continue: userActivity, restorationHandler: restorationHandler)
    }

}
