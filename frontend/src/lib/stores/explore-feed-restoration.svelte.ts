// Keep the closing image over Explore until its scroll position is ready to paint.
export const exploreFeedRestoration = $state({ holdingOverlay: false });

let pageReady: (() => void) | undefined;

export function waitForExploreFeedReturn(): Promise<void> {
	finishExploreFeedReturn();
	return new Promise((resolve) => {
		pageReady = resolve;
	});
}

export function finishExploreFeedReturn() {
	pageReady?.();
	pageReady = undefined;
}
