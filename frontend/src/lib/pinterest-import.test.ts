import { describe, expect, it } from 'vitest';
import { importBoardNotes, summarizeImport, type ImportJob } from './pinterest-import';

const complete: ImportJob = {
	jobId: 'board',
	boardName: 'Art',
	status: 'done',
	phase: 'saving',
	queued: 0,
	running: 0,
	done: 767,
	failed: 0,
	expected: 769,
	listed: 769,
	missing: 0,
	unsupported: 0,
	previouslyDone: 0,
	outcome: 'complete',
	canRetry: false,
	listingIncomplete: false
};

describe('the import has one overall result and recovery action', () => {
	it.each([
		['complete', {}, 'complete', '767 images saved', ''],
		[
			'partial listing',
			{ expected: 849, missing: 80, outcome: 'partial', canRetry: true },
			'partial',
			'767 images saved',
			'Retry missing images'
		],
		[
			'uploads failed',
			{ failed: 3, outcome: 'partial', canRetry: true },
			'partial',
			'767 images saved',
			'Retry missing images'
		],
		[
			'only unsupported',
			{ unsupported: 3, outcome: 'partial' },
			'unsupported',
			'767 images saved',
			''
		],
		[
			'none saved',
			{ done: 0, failed: 3, outcome: 'failed', canRetry: true },
			'failed',
			"We couldn't import your pins",
			'Try again'
		],
		[
			'legacy section',
			{ listed: null, expected: 0, canRetry: true },
			'legacy',
			'767 images saved',
			'Check for missing images'
		],
		[
			'legacy approximate gap',
			{ listed: null, expected: 1000, missing: 233, outcome: 'partial', canRetry: true },
			'legacy',
			'767 images saved',
			'Check for missing images'
		],
		['saving', { status: 'running', queued: 100, outcome: '' }, 'saving', '767 images saved', ''],
		['reading', { status: 'listing', done: 0, outcome: '' }, 'reading', 'Reading your boards', ''],
		[
			'Pinterest retry',
			{ status: 'listing', phase: 'retrying', done: 0, outcome: '' },
			'retrying',
			'Retrying Pinterest',
			''
		],
		[
			'waiting',
			{ status: 'running', phase: 'waiting', outcome: '' },
			'waiting',
			'767 images saved',
			''
		],
		[
			'reconnect',
			{ status: 'running', phase: 'reconnect', outcome: '' },
			'reconnect',
			'767 images saved',
			'Reconnect account'
		]
	] as const)('%s', (_name, patch, state, title, actionLabel) => {
		const result = summarizeImport([{ ...complete, ...patch }]);
		expect(result.state).toBe(state);
		expect(result.title).toBe(title);
		expect(result.actionLabel).toBe(actionLabel);
		if (result.finished) expect(result.progress).toBeNull();
	});

	it('prioritizes a required reconnection over automatic waiting on another board', () => {
		const result = summarizeImport([
			{ ...complete, status: 'running', phase: 'waiting', outcome: '' },
			{ ...complete, status: 'running', phase: 'reconnect', outcome: '' }
		]);
		expect(result.action).toBe('reconnect');
		expect(result.hint).toBe('Your progress is saved.');
	});

	it('does not claim a failed request stopped the background import', () => {
		expect(summarizeImport(null, true)).toMatchObject({
			state: 'load-error',
			action: 'refresh',
			actionLabel: 'Reload status'
		});
		expect(summarizeImport(null, true).description).toContain('may still be running');
		expect(summarizeImport(null).action).toBe('');
		expect(summarizeImport([]).state).toBe('preparing');
	});

	it('uses upload counts for progress and hides it until listing is finished', () => {
		expect(
			summarizeImport([
				{
					...complete,
					status: 'running',
					done: 40,
					failed: 10,
					queued: 50,
					expected: 2000,
					outcome: ''
				}
			]).progress
		).toBe(50);
		expect(summarizeImport([{ ...complete, status: 'listing', outcome: '' }]).progress).toBeNull();
	});

	it('keeps raw-pin accounting out of the headline and detail rows free of actions', () => {
		const partial = {
			...complete,
			expected: 849,
			missing: 80,
			failed: 2,
			unsupported: 3,
			outcome: 'partial' as const,
			canRetry: true
		};
		expect(summarizeImport([partial]).description).toBe(
			'Some pins are still missing from this import.'
		);
		expect(importBoardNotes(partial)).toEqual([
			'80 pins not returned by Pinterest',
			'2 images could not be saved',
			'3 pins without a supported image'
		]);
		expect(importBoardNotes({ ...complete, expected: 0, listed: null })).toEqual([
			'Completeness was not recorded for this older import.'
		]);
		expect(importBoardNotes({ ...complete, previouslyDone: 700 })).toEqual([
			'67 added in this retry'
		]);
	});
});
