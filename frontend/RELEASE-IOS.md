# iOS release build

The day-to-day build (`cap run ios`, or ⌘R in Xcode) is a **debug** build on the Simulator or a
development device. Shipping to the App Store needs a **signed release archive** uploaded to App
Store Connect. This doc is the checklist. It assumes the `ios/` project from this repo (Capacitor 8,
Swift Package Manager — no CocoaPods) and an **Apple Developer Program** membership.

The native project layout:

- App target `is.currents.app`, URL scheme `currents://` (OAuth deep link), App Group
  `group.is.currents.app`.
- **ShareExtension** target `is.currents.app.ShareExtension` (the iOS share sheet → app). It writes
  shared files into the App Group container and opens the app via `currents://shared?...`, which
  `AppDelegate` parses into the `send-intent` plugin's `ShareStore`.

## 0. Prerequisites on the backend (one-time)

Identical to Android — the released app talks to whatever `PUBLIC_APPVIEW_URL` was baked in at build
time. Point it at the **production** appview and make sure that appview:

- CORS allows `capacitor://localhost` and `https://localhost`, and the `Authorization` header.
- `MOBILE_REDIRECT_SCHEMES` includes `currents://` (default).
- `GET /oauth/login` is registered (native in-app-browser flow).

## 1. Production web build

```bash
cd frontend
# Release points at the production appview (.env.production → https://api.currents.is).
# CAPAWESOME_TOKEN is needed to install @capawesome-team/* (see §7); it's in frontend/.env.
npm ci
npm run build:mobile     # CAPACITOR=1 vite build (mode=production) + cap sync
# (or: CAPACITOR=1 vite build --mode production && npx cap sync ios)
```

`cap sync ios` copies `build/` into `ios/App/App/public` and regenerates the SPM package list. It
does **not** touch the ShareExtension target, Info.plist, or entitlements.

## 2. Toolchain (one-time)

- **Xcode 26+** installed and selected: `sudo xcode-select -s /Applications/Xcode.app/Contents/Developer`.
- The iOS **Simulator runtime**: `xcodebuild -downloadPlatform iOS` (Xcode ships runtimes separately).
- CocoaPods is **not** required (Capacitor 8 uses SPM).

Open the project: `npx cap open ios` (opens `ios/App/App.xcodeproj` in Xcode).

## 3. Signing & capabilities (one-time, both targets)

Automatic signing is simplest. In Xcode → **Signing & Capabilities**, for **both** the `App` and
`ShareExtension` targets:

1. Set **Team** to your Apple Developer team (this also sets `DEVELOPMENT_TEAM` in the project).
2. Confirm **Automatically manage signing** is on. Xcode registers the two App IDs
   (`is.currents.app`, `is.currents.app.ShareExtension`) on the developer portal.
3. The **App Groups** capability is already declared via the `.entitlements` files
   (`App/App.entitlements`, `ShareExtension/ShareExtension.entitlements`), both containing
   `group.is.currents.app`. The **first** signed build prompts Xcode to register the App Group on the
   portal and add it to both provisioning profiles — accept it. (On the Simulator the App Group works
   without any of this; it only matters for device/TestFlight/App Store builds.)

> If a build fails with "doesn't support the App Groups capability" or a provisioning error, it's
> almost always step 3 not yet registered for one of the two App IDs — open each target's Signing tab
> once with a device/Any iOS Device destination selected so Xcode provisions them.

## 4. Versioning

Bump per release on **both** targets (they're set in build settings, not Info.plist):

- `MARKETING_VERSION` (e.g. `1.0.1`) — the user-facing version.
- `CURRENT_PROJECT_VERSION` (the build number) — **must strictly increase** for every App Store
  Connect upload.

Keep the App and ShareExtension versions in sync. From the CLI you can set them with `agvtool` or
edit the target build settings in Xcode.

## 5. Export compliance

`ITSAppUsesNonExemptEncryption` is set to `false` in `ios/App/App/App/Info.plist` — the app only uses
standard HTTPS/OAuth, which is exempt. This skips the per-build encryption questionnaire on upload.

## 6. Archive + upload

**Via Xcode (recommended first time):**

1. Select **Any iOS Device (arm64)** as the destination.
2. **Product → Archive**.
3. In the Organizer: **Distribute App → App Store Connect → Upload**.

**Via CLI:**

```bash
cd frontend/ios/App
xcodebuild -project App.xcodeproj -scheme App -configuration Release \
  -destination 'generic/platform=iOS' -archivePath build/Currents.xcarchive archive
xcodebuild -exportArchive -archivePath build/Currents.xcarchive \
  -exportOptionsPlist ExportOptions.plist -exportPath build/export
# then upload build/export/*.ipa via Transporter, or:
xcrun altool --upload-app -f build/export/Currents.ipa -t ios \
  --apiKey <KEY_ID> --apiIssuer <ISSUER_ID>
```

Sanity-check the archive installs and the release build still logs in / uploads / shares.

## 7. Licensing / plugins

- `@capawesome-team/capacitor-secure-preferences` is a **paid Capawesome Insiders** plugin (Keychain
  storage on iOS). Confirm the Insiders license covers **production distribution**. `CAPAWESOME_TOKEN`
  is only needed at install/build time, not at runtime.
- All other plugins (`send-intent`, `@capacitor/*`, `@capacitor-community/safe-area`) are MIT/free.

## 8. App Store Connect

1. Create the app in App Store Connect (bundle `is.currents.app`).
2. **App Privacy**: declare camera usage (`NSCameraUsageDescription`), photo library access
   (`NSPhotoLibraryUsageDescription`), that shared images/links are processed, and auth/account data.
   Provide the **privacy policy URL** (the app already has `/privacy`).
3. **Age rating** questionnaire — it's a UGC image app with moderation; describe the moderation /
   labeling system.
4. Upload the build (§6), assign it to **TestFlight** internal testing first → test on real installs →
   submit for App Store review.
5. The ShareExtension ships **inside** the app bundle — no separate App Store entry; it just needs to
   be embedded (it is, via the "Embed Foundation Extensions" build phase) and signed with the same
   team.

## Checklist before each release

- [ ] `PUBLIC_APPVIEW_URL` = production; prod backend has mobile CORS + `currents://` + `GET /oauth/login`
- [ ] `npm run build:mobile` (production mode) — verify `build/` baked `https://api.currents.is`
- [ ] `MARKETING_VERSION` + `CURRENT_PROJECT_VERSION` bumped on **both** targets
- [ ] Both targets signed with the team; App Group registered on the portal
- [ ] Archive uploads cleanly (no export-compliance / provisioning prompts)
- [ ] Smoke test: login, browse, upload (gallery + camera), save/unsave, **share image + link from
      another app's share sheet**
- [ ] Assign to TestFlight internal testing
