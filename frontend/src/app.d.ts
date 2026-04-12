// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
import type { SaveView } from '$lib/types';

declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		interface PageState {
			save?: SaveView;
		}
		// interface Platform {}
	}
}

export {};
