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
        // Restart any tasks that were paused (or not yet started) while the application was inactive. If the application was previously in the background, optionally refresh the user interface.
    }

    func applicationWillTerminate(_ application: UIApplication) {
        // Called when the application is about to terminate. Save data if appropriate. See also applicationDidEnterBackground:.
    }

    func application(_ app: UIApplication, open url: URL, options: [UIApplication.OpenURLOptionsKey: Any] = [:]) -> Bool {
        // Called when the app was launched with a url. Feel free to add additional processing here,
        // but if you want the App API to support tracking app url opens, make sure to keep this call
        let handled = ApplicationDelegateProxy.shared.application(app, open: url, options: options)

        // currents://shared?... — content from the Share Extension. Pull the query items into the
        // send-intent plugin's ShareStore and notify it, mirroring its README integration. The
        // OAuth deep link (currents://oauth-callback?token=...) carries no "title" param, so it's
        // naturally skipped here and handled by Capacitor's appUrlOpen (src/lib/app-init.ts).
        if let components = URLComponents(url: url, resolvingAgainstBaseURL: true),
            let params = components.queryItems {
            let titles = params.filter { $0.name == "title" }
            if !titles.isEmpty {
                let descriptions = params.filter { $0.name == "description" }
                let types = params.filter { $0.name == "type" }
                let urls = params.filter { $0.name == "url" }
                shareStore.shareItems.removeAll()
                for index in 0..<titles.count {
                    var item = JSObject()
                    item["title"] = titles[index].value ?? ""
                    item["description"] = index < descriptions.count ? (descriptions[index].value ?? "") : ""
                    item["type"] = index < types.count ? (types[index].value ?? "") : ""
                    item["url"] = index < urls.count ? (urls[index].value ?? "") : ""
                    shareStore.shareItems.append(item)
                }
                shareStore.processed = false
                NotificationCenter.default.post(name: Notification.Name("triggerSendIntent"), object: nil)
            }
        }

        return handled
    }

    func application(_ application: UIApplication, continue userActivity: NSUserActivity, restorationHandler: @escaping ([UIUserActivityRestoring]?) -> Void) -> Bool {
        // Called when the app was launched with an activity, including Universal Links.
        // Feel free to add additional processing here, but if you want the App API to support
        // tracking app url opens, make sure to keep this call
        return ApplicationDelegateProxy.shared.application(application, continue: userActivity, restorationHandler: restorationHandler)
    }

}
