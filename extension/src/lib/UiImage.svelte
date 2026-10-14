<script lang="ts">
	interface Props {
		src: string;
		alt?: string;
		class?: string;
		loading?: 'lazy' | 'eager';
		onFail?: () => void;
	}

	let { src, alt = '', class: className = '', loading = 'lazy', onFail }: Props = $props();
	let dataUrl = $state('');
	let failed = $state(false);
	let triedProxy = false;

	$effect(() => {
		if (!src) return;
		dataUrl = '';
		failed = false;
		triedProxy = false;
	});

	function fail() {
		failed = true;
		onFail?.();
	}

	async function onError() {
		if (triedProxy) return fail();
		triedProxy = true;
		const failedSrc = src;
		try {
			const result = await browser.runtime.sendMessage({ type: 'FETCH_UI_IMAGE', url: src });
			if (src === failedSrc && result.dataUrl) dataUrl = result.dataUrl;
			else if (src === failedSrc) fail();
		} catch {
			if (src === failedSrc) fail();
		}
	}
</script>

{#if failed}
	<div class="{className} bg-muted"></div>
{:else}
	<img src={dataUrl || src} {alt} {loading} class={className} onerror={onError} />
{/if}
