import { expect, test, vi } from 'vitest';
import { dismissTopOverlay, onBackButton } from './back-button';

test('dismisses the most recently registered overlay first', () => {
	const first = vi.fn();
	const second = vi.fn();
	const removeFirst = onBackButton(first);
	const removeSecond = onBackButton(second);

	expect(dismissTopOverlay()).toBe(true);
	expect(second).toHaveBeenCalledOnce();
	expect(first).not.toHaveBeenCalled();

	removeSecond();
	expect(dismissTopOverlay()).toBe(true);
	expect(first).toHaveBeenCalledOnce();

	removeFirst();
});
