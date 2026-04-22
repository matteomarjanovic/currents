<script lang="ts">
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import {
		FieldGroup,
		Field,
		FieldLabel,
		FieldDescription,
		FieldSeparator
	} from '$lib/components/ui/field/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { cn } from '$lib/utils.js';
	import type { HTMLAttributes } from 'svelte/elements';

	let {
		class: className,
		returnTo,
		...restProps
	}: HTMLAttributes<HTMLDivElement> & { returnTo?: string } = $props();

	const id = $props.id();
	const loginAction = `${PUBLIC_APPVIEW_URL}/oauth/login`;

	type Actor = { did: string; handle: string; displayName?: string; avatar?: string };

	let handle = $state('');
	let suggestions: Actor[] = $state([]);
	let showSuggestions = $state(false);
	let activeIndex = $state(-1);
	let debounceTimer: ReturnType<typeof setTimeout>;

	async function fetchSuggestions(q: string) {
		if (q.length < 2) {
			suggestions = [];
			showSuggestions = false;
			return;
		}
		try {
			const res = await fetch(
				`https://public.api.bsky.app/xrpc/app.bsky.actor.searchActorsTypeahead?q=${encodeURIComponent(q)}&limit=6`
			);
			if (res.ok) {
				const data = await res.json();
				suggestions = data.actors ?? [];
				showSuggestions = suggestions.length > 0;
				activeIndex = -1;
			}
		} catch {
			// silently ignore network errors
		}
	}

	function onInput() {
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => fetchSuggestions(handle), 250);
	}

	function onKeydown(e: KeyboardEvent) {
		if (!showSuggestions) return;
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			activeIndex = Math.min(activeIndex + 1, suggestions.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			activeIndex = Math.max(activeIndex - 1, -1);
		} else if (e.key === 'Enter' && activeIndex >= 0) {
			e.preventDefault();
			selectSuggestion(suggestions[activeIndex]);
		} else if (e.key === 'Escape') {
			showSuggestions = false;
		}
	}

	function selectSuggestion(actor: Actor) {
		handle = actor.handle;
		showSuggestions = false;
		activeIndex = -1;
	}

	function onBlur() {
		setTimeout(() => {
			showSuggestions = false;
		}, 150);
	}
</script>

<div class={cn('flex flex-col gap-6', className)} {...restProps}>
	<Card.Root class="overflow-visible">
		<Card.Header class="text-center">
			<Card.Title class="text-xl">Welcome back to Currents</Card.Title>
			<Card.Description>Login with your Atmosphere account</Card.Description>
		</Card.Header>
		<Card.Content>
			<FieldGroup>
				<Field>
					<form method="POST" action={loginAction}>
						<input type="hidden" name="username" value="https://bsky.social" />
						{#if returnTo}<input type="hidden" name="return_to" value={returnTo} />{/if}
						<Button variant="outline" type="submit" class="w-full">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								viewBox="0 0 576 512"
								fill="currentColor"
								class="size-4"
							>
								><!--!Font Awesome Free v7.2.0 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license/free Copyright 2026 Fonticons, Inc.--><path
									d="M407.8 294.7c-3.3-.4-6.7-.8-10-1.3 3.4 .4 6.7 .9 10 1.3zM288 227.1C261.9 176.4 190.9 81.9 124.9 35.3 61.6-9.4 37.5-1.7 21.6 5.5 3.3 13.8 0 41.9 0 58.4S9.1 194 15 213.9c19.5 65.7 89.1 87.9 153.2 80.7 3.3-.5 6.6-.9 10-1.4-3.3 .5-6.6 1-10 1.4-93.9 14-177.3 48.2-67.9 169.9 120.3 124.6 164.8-26.7 187.7-103.4 22.9 76.7 49.2 222.5 185.6 103.4 102.4-103.4 28.1-156-65.8-169.9-3.3-.4-6.7-.8-10-1.3 3.4 .4 6.7 .9 10 1.3 64.1 7.1 133.6-15.1 153.2-80.7 5.9-19.9 15-138.9 15-155.5s-3.3-44.7-21.6-52.9c-15.8-7.1-40-14.9-103.2 29.8-66.1 46.6-137.1 141.1-163.2 191.8z"
								/></svg
							>
							Login with Bluesky
						</Button>
					</form>
					<form method="POST" action={loginAction}>
						<input type="hidden" name="username" value="https://eurosky.social" />
						{#if returnTo}<input type="hidden" name="return_to" value={returnTo} />{/if}
						<Button variant="outline" type="submit" class="w-full">
							<svg
								fill="currentColor"
								viewBox="77.86 113.9 129.15 129.15"
								xmlns="http://www.w3.org/2000/svg"
							>
								<path
									d="M148.846 144.562C148.846 159.75 161.158 172.062 176.346 172.062H207.012V185.865H176.346C161.158 185.865 148.846 198.177 148.846 213.365V243.045H136.029V213.365C136.029 198.177 123.717 185.865 108.529 185.865H77.8633V172.062H108.529C123.717 172.062 136.029 159.75 136.029 144.562V113.896H148.846V144.562Z"
								></path>
							</svg>
							Login with Eurosky
						</Button>
					</form>
					<!-- <form method="POST" action={loginAction}>
						<input type="hidden" name="username" value="https://blacksky.community" />
						<Button variant="outline" type="submit" class="w-full">
							<svg fill="currentColor" viewBox="-0.5 1 286 243" xmlns="http://www.w3.org/2000/svg">
								<g>
									<path
										d="M148.846 144.562C148.846 159.75 161.158 172.062 176.346 172.062H207.012V185.865H176.346C161.158 185.865 148.846 198.177 148.846 213.365V243.045H136.029V213.365C136.029 198.177 123.717 185.865 108.529 185.865H77.8633V172.062H108.529C123.717 172.062 136.029 159.75 136.029 144.562V113.896H148.846V144.562Z"
									></path>
									<path
										d="M170.946 31.8766C160.207 42.616 160.207 60.0281 170.946 70.7675L192.631 92.4516L182.871 102.212L161.186 80.5275C150.447 69.7881 133.035 69.7881 122.296 80.5275L101.309 101.514L92.2456 92.4509L113.232 71.4642C123.972 60.7248 123.972 43.3128 113.232 32.5733L91.5488 10.8899L101.309 1.12988L122.993 22.814C133.732 33.5533 151.144 33.5534 161.884 22.814L183.568 1.12988L192.631 10.1925L170.946 31.8766Z"
									></path>
									<path
										d="M79.0525 75.3259C75.1216 89.9962 83.8276 105.076 98.498 109.006L128.119 116.943L124.547 130.275L94.9267 122.338C80.2564 118.407 65.1772 127.113 61.2463 141.784L53.5643 170.453L41.1837 167.136L48.8654 138.467C52.7963 123.797 44.0902 108.718 29.4199 104.787L-0.201172 96.8497L3.37124 83.5173L32.9923 91.4542C47.6626 95.3851 62.7419 86.679 66.6728 72.0088L74.6098 42.3877L86.9895 45.7048L79.0525 75.3259Z"
									></path>
									<path
										d="M218.413 71.4229C222.344 86.093 237.423 94.7992 252.094 90.8683L281.715 82.9313L285.287 96.2628L255.666 104.2C240.995 108.131 232.29 123.21 236.22 137.88L243.902 166.55L231.522 169.867L223.841 141.198C219.91 126.528 204.831 117.822 190.16 121.753L160.539 129.69L156.967 116.357L186.588 108.42C201.258 104.49 209.964 89.4103 206.033 74.74L198.096 45.1189L210.476 41.8018L218.413 71.4229Z"
									></path>
								</g>
							</svg>
							Login with Blacksky
						</Button>
					</form> -->
				</Field>
				<FieldSeparator class="*:data-[slot=field-separator-content]:bg-card">
					Or use your custom PDS
				</FieldSeparator>
				<form method="POST" action={loginAction}>
					{#if returnTo}<input type="hidden" name="return_to" value={returnTo} />{/if}
					<FieldGroup>
						<Field>
							<FieldLabel for="handle-{id}">Your handle</FieldLabel>
							<div class="relative">
								<Input
									id="handle-{id}"
									name="username"
									type="text"
									placeholder="handle.bsky.social"
									required
									autocomplete="off"
									bind:value={handle}
									oninput={onInput}
									onkeydown={onKeydown}
									onblur={onBlur}
								/>
								{#if showSuggestions}
									<ul
										class="absolute left-0 right-0 top-full z-50 mt-1 overflow-hidden rounded-xl border bg-popover text-popover-foreground shadow-md"
									>
										{#each suggestions as actor, i (actor.did)}
											<li>
												<button
													type="button"
													class={cn(
														'flex w-full items-center gap-2 px-3 py-2 text-sm transition-colors',
														i === activeIndex
															? 'bg-accent text-accent-foreground'
															: 'hover:bg-accent hover:text-accent-foreground'
													)}
													onmousedown={() => selectSuggestion(actor)}
												>
													{#if actor.avatar}
														<img src={actor.avatar} alt="" class="size-5 shrink-0 rounded-full" />
													{:else}
														<div class="size-5 shrink-0 rounded-full bg-muted"></div>
													{/if}
													<span class="font-medium">@{actor.handle}</span>
													{#if actor.displayName}
														<span class="truncate text-muted-foreground">{actor.displayName}</span>
													{/if}
												</button>
											</li>
										{/each}
									</ul>
								{/if}
							</div>
						</Field>
						<Field>
							<Button type="submit" class="w-full">Login</Button>
							<FieldDescription class="text-center">
								Don't have an Atmosphere account? <a href={resolve('/register')}>Sign up</a>
							</FieldDescription>
						</Field>
					</FieldGroup>
				</form>
			</FieldGroup>
		</Card.Content>
	</Card.Root>
	<FieldDescription class="px-6 text-center">
		By clicking continue, you agree to our <a href={resolve('/terms')}>Terms of Service</a>
		and <a href={resolve('/privacy')}>Privacy Policy</a>.
	</FieldDescription>
</div>
