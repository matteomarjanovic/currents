# Android release build

The day-to-day build (`gradlew installDebug`) is a **debug** build: debuggable, WebView-inspectable,
unsigned for distribution. Shipping to the Play Store needs a **signed release** build of an **AAB**.
This doc is the checklist.

## 0. Prerequisites on the backend (one-time)

The released app talks to whatever `PUBLIC_APPVIEW_URL` was baked in at build time. Point it at the
**production** appview (not `api-dev`), and make sure that appview runs the mobile code from this branch:

- CORS allows `capacitor://localhost` and `https://localhost`, and the `Authorization` header.
- `MOBILE_REDIRECT_SCHEMES` includes `currents://` (default).
- `GET /oauth/login` is registered (native in-app-browser flow).

(We verified all of this against `api-dev.currents.is`; production must have the same.)

## 1. Production web build

```bash
cd frontend
# .env for release — point at the production appview, NOT api-dev:
#   PUBLIC_APPVIEW_URL=https://api.currents.is   (or whatever prod is)
#   CAPAWESOME_TOKEN=...   (needed to install @capawesome-team/* — see §6)
set -a; . ./.env; set +a
npm ci
npm run build           # vite build -> build/  (ssr=false SPA)
npx cap sync android    # copies build/ into the native project
```

## 2. Create a release signing keystore (one-time, keep it SAFE)

```bash
keytool -genkey -v -keystore currents-release.keystore \
  -alias currents -keyalg RSA -keysize 2048 -validity 10000
```

- Store the `.keystore` file and its passwords in a password manager / secrets vault.
- **If you lose this keystore you can never update the app on the Play Store.** Back it up.
- Do **not** commit it (see §5 gitignore).

## 3. Signing config (already wired — you only create keystore.properties)

The Gradle signing config and `.gitignore` entries are **already in this repo** (`app/build.gradle`
+ `android/.gitignore`). You only need to create the secrets file.

Create `frontend/android/keystore.properties` (gitignored). `storeFile` is resolved relative to
`frontend/android/` (where the keystore typically lives) — so just use its filename, or an absolute path:

```properties
storeFile=currents-release.keystore
storePassword=********
keyAlias=currents
keyPassword=********
```

For reference, this is the config already present in `app/build.gradle` (above `android {`):

```gradle
def keystoreProps = new Properties()
def keystorePropsFile = rootProject.file("keystore.properties")
if (keystorePropsFile.exists()) {
    keystoreProps.load(new FileInputStream(keystorePropsFile))
}
```

and inside `android { ... }` (note `rootProject.file` so the relative `storeFile` resolves against
`android/`, and the signingConfig is only applied when `keystore.properties` exists):

```gradle
    signingConfigs {
        release {
            if (keystorePropsFile.exists()) {
                storeFile rootProject.file(keystoreProps['storeFile'])
                storePassword keystoreProps['storePassword']
                keyAlias keystoreProps['keyAlias']
                keyPassword keystoreProps['keyPassword']
            }
        }
    }
    buildTypes {
        release {
            if (keystorePropsFile.exists()) {
                signingConfig signingConfigs.release
            }
            minifyEnabled false
            proguardFiles getDefaultProguardFile('proguard-android.txt'), 'proguard-rules.pro'
        }
    }
```

> R8 minification is **off** (safest with Capacitor plugins, which use reflection). To shrink the
> AAB later, set `minifyEnabled true` + `shrinkResources true` and switch to
> `proguard-android-optimize.txt` — then runtime-test the release build before shipping.

## 4. Version + build the AAB

Bump per release in `app/build.gradle` (`versionCode` must increase every upload; currently `2`):

```gradle
versionCode 3
versionName "1.0.1"
```

Then:

```bash
cd frontend/android
JAVA_HOME="/Applications/Android Studio.app/Contents/jbr/Contents/Home" ./gradlew bundleRelease
# output: app/build/outputs/bundle/release/app-release.aab   (upload this to Play)
# (or ./gradlew assembleRelease for a signed APK for sideloading)
```

Sanity-check the signed APK installs and the release build still logs in / uploads / shares.

## 5. .gitignore (already partly there)

Add to `frontend/android/.gitignore` (or root) — **never commit secrets**:

```
keystore.properties
*.keystore
*.jks
```

## 6. Licensing / plugins

- `@capawesome-team/capacitor-secure-preferences` is a **paid Capawesome Insiders** plugin. Confirm
  the Insiders license covers **production distribution** (not just dev). `CAPAWESOME_TOKEN` is only
  needed at install/build time, not at runtime.
- All other plugins (`send-intent`, `@capacitor/*`, `@capacitor-community/safe-area`) are MIT/free.

## 7. Play Console

0. **Create a Google Play developer account** (one-time, if you don't have one): go to
   <https://play.google.com/console>, sign in with a Google account, pay the **one-time $25** fee,
   and complete **identity verification** (legal name / address / phone, possibly a government ID;
   takes hours–days). Note Google's policy for **new personal accounts**: you must run a **closed
   test with ≥20 opted-in testers for 14 continuous days** before you can apply for production
   access. An **organization** account skips the 20-tester rule but needs a D-U-N-S number.
1. Create the app in Play Console (package `is.currents.app`).
2. **Data safety** form: declare camera usage and that shared images/links are processed; declare
   auth/account data. Provide a **privacy policy URL** (the app already has `/privacy`).
3. **Content rating** questionnaire (note: it's a UGC image app with moderation — describe the
   moderation/labeling system).
4. Upload the AAB to **Internal testing** first → test on real installs → promote to Closed/Open/Production.
5. `targetSdkVersion` is 36 (current) — meets Play's recent-target requirement.

## Checklist before each release
- [ ] `PUBLIC_APPVIEW_URL` = production, prod backend has mobile CORS + `currents://` + `GET /oauth/login`
- [ ] `npm run build && npx cap sync android`
- [ ] `versionCode` bumped
- [ ] `bundleRelease` produces a **signed** AAB
- [ ] Smoke test the signed build: login, browse, upload (gallery + camera), save/unsave, share image + link
- [ ] Upload to Internal testing
