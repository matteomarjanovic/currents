export interface ImportJob {
	jobId: string;
	boardName: string;
	status: 'listing' | 'running' | 'done' | 'failed';
	queued: number;
	running: number;
	done: number;
	failed: number;
	expected: number;
	listed: number | null;
	missing: number;
	unsupported: number;
	previouslyDone: number;
	phase: string;
	outcome: 'complete' | 'partial' | 'failed' | '';
	canRetry: boolean;
	listingIncomplete: boolean;
}

export const importCount = (n: number) => n.toLocaleString('en-US');

export function summarizeImport(jobs: ImportJob[] | null, loadError = false) {
	const summary = {
		state: 'loading',
		title: 'Loading your import…',
		description: '',
		hint: '',
		action: '' as '' | 'retry' | 'reconnect' | 'refresh',
		actionLabel: '',
		progress: null as number | null,
		finished: false
	};
	if (jobs === null) {
		if (loadError)
			Object.assign(summary, {
				state: 'load-error',
				title: "Couldn't load this import",
				description: 'Your import may still be running in the background.',
				action: 'refresh',
				actionLabel: 'Reload status'
			});
		return summary;
	}
	if (jobs.length === 0)
		return {
			...summary,
			state: 'preparing',
			title: 'Preparing your import…',
			description: 'Your boards will appear shortly.'
		};
	const saved = jobs.reduce((n, j) => n + j.done, 0);
	const total = jobs.reduce((n, j) => n + j.done + j.failed + j.queued + j.running, 0);
	const processed = jobs.reduce((n, j) => n + j.done + j.failed, 0);
	const active = jobs.filter((j) => j.status === 'listing' || j.status === 'running');
	summary.title = `${importCount(saved)} ${saved === 1 ? 'image' : 'images'} saved`;
	if (active.length > 0) {
		summary.hint = 'You can leave this page. Your import continues in the background.';
		if (!active.some((j) => j.status === 'listing') && total > 0)
			summary.progress = (processed / total) * 100;
		if (active.some((j) => j.phase === 'reconnect')) {
			Object.assign(summary, {
				state: 'reconnect',
				description: 'Reconnect your account to continue this import.',
				hint: 'Your progress is saved.',
				action: 'reconnect',
				actionLabel: 'Reconnect account'
			});
		} else if (active.some((j) => j.phase === 'retrying')) {
			summary.state = 'retrying';
			if (saved === 0) summary.title = 'Retrying Pinterest';
			summary.description =
				"Pinterest is taking a little longer. We're trying again automatically.";
		} else if (active.some((j) => j.phase === 'waiting')) {
			summary.state = 'waiting';
			summary.description = 'Waiting for uploads to resume automatically.';
		} else if (active.some((j) => j.status === 'listing')) {
			summary.state = 'reading';
			if (saved === 0) summary.title = 'Reading your boards';
			summary.description = jobs.some((j) => j.previouslyDone > 0)
				? 'Checking Pinterest for missing images.'
				: 'Finding images to import.';
		} else {
			summary.state = 'saving';
			summary.description = 'Saving your images…';
		}
		return summary;
	}
	summary.finished = true;
	const canRetry = jobs.some((j) => j.canRetry);
	const knownProblems = jobs.some(
		(j) =>
			j.failed > 0 ||
			j.listingIncomplete ||
			j.status === 'failed' ||
			(j.listed !== null && (j.missing > 0 || j.unsupported > 0))
	);
	if (!knownProblems && jobs.some((j) => j.listed === null)) {
		summary.state = 'legacy';
		summary.description = 'This older import can be checked for missing images.';
		if (canRetry) {
			summary.action = 'retry';
			summary.actionLabel = 'Check for missing images';
		}
	} else if (saved === 0 && canRetry) {
		summary.state = 'failed';
		summary.title = "We couldn't import your pins";
		summary.description = 'No images were saved. Please try again.';
		summary.action = 'retry';
		summary.actionLabel = 'Try again';
	} else if (jobs.some((j) => j.outcome !== 'complete')) {
		summary.state = canRetry ? 'partial' : 'unsupported';
		summary.description = canRetry
			? jobs.some((j) => j.failed > 0) && !jobs.some((j) => j.missing > 0 || j.listingIncomplete)
				? "Some images couldn't be saved."
				: 'Some pins are still missing from this import.'
			: "Some pins don't include an image we can import.";
		if (canRetry) {
			summary.action = 'retry';
			summary.actionLabel = 'Retry missing images';
		}
	} else {
		summary.state = 'complete';
		summary.description = saved > 0 ? 'Import complete.' : 'No images were found in these boards.';
	}
	if (summary.action === 'retry') summary.hint = 'Previously imported images will be skipped.';
	return summary;
}

export function importBoardNotes(job: ImportJob): string[] {
	const notes: string[] = [];
	if (job.status === 'listing') {
		return [job.phase === 'retrying' ? 'Trying Pinterest again' : 'Finding images'];
	}
	if (job.listed === null) notes.push('Completeness was not recorded for this older import.');
	else if (job.missing > 0)
		notes.push(`${importCount(job.missing)} pins not returned by Pinterest`);
	if (job.listingIncomplete && job.missing === 0) notes.push("Pinterest's listing stopped early");
	if (job.failed > 0) notes.push(`${importCount(job.failed)} images could not be saved`);
	if (job.unsupported > 0)
		notes.push(`${importCount(job.unsupported)} pins without a supported image`);
	if (job.queued + job.running > 0)
		notes.push(`${importCount(job.queued + job.running)} images waiting`);
	if (job.previouslyDone > 0)
		notes.push(`${importCount(Math.max(0, job.done - job.previouslyDone))} added in this retry`);
	return notes;
}
