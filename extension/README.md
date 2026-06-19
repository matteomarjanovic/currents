# Save to Currents

Browser extension for [Currents](https://currents.is). Save images to your
collections from any website: right-click an image and choose **Save to
Currents** to pick a collection and save. The toolbar popup shows your login
status. Built with [WXT](https://wxt.dev) + Svelte, Manifest V3.

## Build from source

Prerequisites: Node.js ≥ 20 (built and tested with 24.x) and npm.

```sh
npm install
npm run zip          # Chrome  → .output/save-to-currents-<version>-chrome.zip
npm run zip:firefox  # Firefox → .output/save-to-currents-<version>-firefox.zip
```

Unpacked builds are written to `.output/chrome-mv3/` and `.output/firefox-mv3/`.

## Configuration

No configuration is required: the production build targets the live Currents
services (`https://currents.is` and `https://api.currents.is`) by default, so a
clean `npm install && npm run zip` reproduces the published extension exactly.

Local development overrides the endpoints via `.env.development` (see
`.env.example`); `npm run dev` runs against that config with hot reload.
