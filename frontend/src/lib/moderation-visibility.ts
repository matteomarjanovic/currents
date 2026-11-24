import type { LabelView } from './types';

export type Visibility = 'show' | 'blur' | 'hide';

const BLUESKY_LABELER = 'did:plc:ar7c4by46qjdydhdevvrndac';
const RESTRICTIVENESS: Record<Visibility, number> = { show: 0, blur: 1, hide: 2 };

export function resolveVisibility(
	visibilityFor: (value: string) => Visibility,
	signedIn: boolean,
	labels: LabelView[] = [],
	localValues: string[] = [],
	now = Date.now()
): Visibility {
	let visibility: Visibility = 'show';
	for (const val of localValues) {
		const next = visibilityFor(val);
		if (RESTRICTIVENESS[next] > RESTRICTIVENESS[visibility]) visibility = next;
	}
	for (const { src, val, exp } of labels) {
		if (exp && Date.parse(exp) <= now) continue;
		let next = visibilityFor(val);
		if (src === BLUESKY_LABELER) {
			if (val.startsWith('currents-')) continue;
			if (val === '!warn') next = 'blur';
			if (val === '!no-unauthenticated' && !signedIn) next = 'hide';
		}
		if (RESTRICTIVENESS[next] > RESTRICTIVENESS[visibility]) visibility = next;
	}
	return visibility;
}
