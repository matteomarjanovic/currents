# Google Play Store Listing — Currents

## App name (≤30 chars)
Currents: Save & Discover

## Short description (≤80 chars)
Save and curate images into collections. Discover more. Own your data.

## Full description (≤4000 chars)
Currents is a beautiful, open-source home for the images you love — and the place to discover your next favorite. Save anything that inspires you, organize it into Collections, and let a personalized feed bring you more of what you're into. Best of all, your data is truly yours: Currents is built on the open AT Protocol, so your saves live on your own account and travel with you. No lock-in, ever.

Think of it as a calmer, more open alternative to the usual image boards — one that respects your attention and your ownership.

WHY CURRENTS

- Own your data. Currents runs on the AT Protocol, the same open network behind a new ecosystem of social apps called Atmosphere. Your saves and collections live on your personal data server, not locked inside one company's silo. Switch apps, keep everything.
- Open source. The whole project is out in the open. No dark patterns, no hidden algorithms working against you — just a tool built to be useful.
- Built for discovery. A smart, personalized feed surfaces images that match your taste, while visual search lets you find more of any style or subject in seconds.

WHAT YOU CAN DO

- Save & curate. Drop any image into a Collection and keep your inspiration organized your way — moodboards, projects, recipes, travel, design references, whatever you're into.
- Save from anywhere. Share an image to Currents straight from your browser or other apps and it lands in the collection you choose.
- Unsorted saves. In a hurry? Save first, sort later. Quick-saves land in your Unsorted shelf on your profile until you're ready to file them away.
- Star your favorites. Pin the collections you open most so they're always one tap away.
- Personalized discovery feed. The more you save, the smarter your feed gets — blending fresh, popular images with picks tuned to the collections you care about.
- Visual search. Search by what you mean, not just keywords. Type a description and Currents finds visually matching images across the network.
- Explore the network. Because Currents is built on an open protocol, you're discovering images saved by people across the whole community — not just one closed app.

SAFETY & CONTROL

Currents includes built-in moderation with sensible defaults. Sensitive content (adult or violent imagery) is blurred until you choose to view it, and AI-generated images can be labeled so you always know what you're looking at. You're in charge: adjust your moderation preferences any time, and they follow you across your devices.

PORTABLE BY DESIGN

Your identity and your saves aren't trapped in this app. Sign in with your AT Protocol account and everything you create in Currents stays connected to you — readable by other apps on the network and yours to take anywhere. That's the whole point: an image-saving app that puts you, not a platform, at the center.

Whether you're building moodboards, collecting design inspiration, planning a project, or just keeping the images that make you happy, Currents gives you a clean, fast, open place to do it.

Save what you love. Discover what's next. Own it all.

Currents is free and open source.

---

# Data safety (Google Play Console form)

> Transcribe into Play Console → **App content → Data safety**. ⚠️ **Verify against your actual
> backend behavior and your `/privacy` policy before submitting** — inaccurate declarations are a
> policy violation. Notes flag the AT-Protocol-specific judgment calls.

**Does the app collect or share user data?** Yes.

### Data collected

| Data type | Collected | Purpose | Required |
|---|---|---|---|
| Personal info → Name (display name) | Yes | App functionality, Account management | Required |
| Personal info → User IDs (AT Proto DID + handle) | Yes | App functionality, Account management | Required |
| Photos and videos → Photos (images you save/upload) | Yes | App functionality | Required |
| App activity → App interactions (saves, collections) | Yes | App functionality, Personalization | Required |
| App activity → Other UGC (search queries) | Yes | App functionality | Optional |

### Data NOT collected
Location, financial, health/fitness, messages, audio, files/docs, calendar, contacts, web-browsing
history, installed apps, **device/advertising IDs**. No ads, no third-party analytics SDKs.
(There's no Firebase/Crashlytics unless you add `google-services.json`, so no crash/diagnostics data.)

### The "shared with third parties" judgment call (read this)
Currents runs on the open **AT Protocol**: the images you save, your collections, and your profile
live on **your own account / Personal Data Server (PDS)** and are **public records on the network**,
readable by other AT Protocol apps (like posting publicly). Currents does **not** sell data or share
it with advertisers/data brokers.

- This public-by-design behavior **must be disclosed in your privacy policy** (`/privacy`).
- Whether to tick **"shared with third parties"** is a real judgment call for decentralized apps:
  user content the user knowingly makes public is generally treated as *collected, not shared* — but
  your PDS provider (e.g. Bluesky) and the network do receive the data. When in doubt, disclose the
  public nature explicitly rather than under-declare.

### Security practices
- **Encrypted in transit:** Yes (all traffic over HTTPS/TLS).
- **Users can request data deletion:** Yes — deleting a save/collection removes the underlying AT
  Protocol record; account management/deletion happens via the user's PDS. Provide a deletion path or
  contact email in the listing.
- Account sign-in is **required** to use the app (AT Protocol OAuth — no password is collected).

### Permissions to justify (Console asks for sensitive ones)
- **Camera** — take a photo to save into a collection.
- Photos/media — selecting images to save (via the system picker).
