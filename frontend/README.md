# sv

Everything you need to build a Svelte project, powered by [`sv`](https://github.com/sveltejs/cli).

## Creating a project

If you're seeing this, you've probably already done this step. Congrats!

```sh
# create a new project
npx sv create my-app
```

To recreate this project with the same configuration:

```sh
# recreate this project
npx sv@0.14.0 create --template minimal --types ts --add prettier eslint tailwindcss="plugins:typography" mcp="ide:claude-code+setup:remote" --install npm frontend
```

## Developing

Once you've created a project and installed dependencies with `npm install` (or `pnpm install` or `yarn`), start a development server:

```sh
npm run dev

# or start the server and open the app in a new browser tab
npm run dev -- --open
```

## Building

To create a production version of your app:

```sh
npm run build
```

You can preview the production build with `npm run preview`.

> To deploy your app, you may need to install an [adapter](https://svelte.dev/docs/kit/adapters) for your target environment.

## Security dependency override

SvelteKit 2 still requires cookie 0.6. The scoped package override uses cookie
0.7.2 to fix [CVE-2024-47764](https://github.com/jshttp/cookie/security/advisories/GHSA-pxg6-pf52-xh8x)
without a SvelteKit 3 migration. Keep it until the chosen SvelteKit release
depends on a patched cookie version.

Native reconnect browser tests need the secure-preferences JavaScript bridge
(real plugin or a registration-only test shim): the tests emulate its native
methods through Capacitor. A local no-op web stub cannot test token persistence.

## Native security updates

Capacitor core, Android, iOS and CLI must stay on a patched release (currently
8.4.3). After an SDK update, rebuild and redistribute both native apps; updating
the website alone does not patch installed apps.

The native build/sync npm commands require the real secure-preferences plugin
before changing the native projects. A web-only stub is insufficient. If the
registry returns 401, configure credentials in `~/.npmrc` on that Mac using the
[Capawesome registry setup](https://capawesome.io/docs/insiders/getting-started/),
then install `@capawesome-team/capacitor-secure-preferences` and run
`npm run build:mobile`. Resolve/build in Xcode and Android Studio afterward.

Test login, app-restart session persistence, reconnect, saving and deep-link
returns on both platforms before publishing the patched binaries.
