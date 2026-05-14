<script lang="ts">
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { resolve } from '$app/paths';
	import LogoMerged from '$lib/assets/logo.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { FieldGroup, Field, FieldDescription } from '$lib/components/ui/field/index.js';
	import RegisterFormMobile from '$lib/components/register-form-mobile.svelte';
	import { isNative } from '$lib/platform';

	const loginAction = `${PUBLIC_APPVIEW_URL}/oauth/login`;
</script>

<div class="flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
	<div class="flex w-full max-w-sm flex-col gap-6">
		<a href={resolve('/')} class="flex h-5 items-center gap-2 self-center font-medium">
			<LogoMerged />
		</a>
		{#if isNative()}
			<RegisterFormMobile />
		{:else}
			<div class="flex flex-col gap-6">
				<Card.Root>
					<Card.Header class="text-center">
						<Card.Title class="text-xl">Register your Atmosphere account</Card.Title>
						<Card.Description>
							One identity for Currents and the wider AT&nbsp;Protocol network.
						</Card.Description>
					</Card.Header>
					<Card.Content>
						<FieldGroup>
							<Field>
								<form method="POST" action={loginAction}>
									<input type="hidden" name="username" value="https://eurosky.social" />
									<Button variant="outline" type="submit" class="w-full relative">
										<span
											class="rounded-full bg-secondary px-2 py-0.5 text-xs font-medium text-primary absolute -top-4"
										>
											Recommended
										</span>
										<svg viewBox="0 0 1000 1000" class="h-8 w-8 shrink-0" fill="currentColor" aria-hidden="true"><path d="M990.9671,522.1991c-3.0947,0-6.1689,0-9.2229.0001-456.4914.0202-459.525,3.0539-459.5452,459.5452-.0001,3.0539-.0001,6.1282-.0001,9.2228h-44.3979c0-3.0947,0-6.1689-.0001-9.2229-.0202-456.4913-3.0538-459.525-459.5452-459.5452-3.0539-.0001-6.1282-.0001-9.2229-.0001v-44.3981c3.0947,0,6.1689,0,9.2229-.0001,456.4914-.0202,459.525-3.0534,459.5452-459.5452.0001-3.0539.0001-6.1282.0001-9.2229h44.3979c0,3.0947,0,6.1689.0001,9.2228.0202,456.4917,3.0538,459.525,459.5452,459.5452,3.0539.0001,6.1282.0001,9.2229.0001v44.3981Z"></path></svg>
										Register with Eurosky
									</Button>
								</form>
								<form method="POST" action={loginAction}>
									<input type="hidden" name="username" value="https://bsky.social" />
									<Button variant="outline" type="submit" class="w-full">
										<svg
											xmlns="http://www.w3.org/2000/svg"
											viewBox="0 0 576 512"
											fill="currentColor"
											class="size-4"
										>
											<path
												d="M407.8 294.7c-3.3-.4-6.7-.8-10-1.3 3.4 .4 6.7 .9 10 1.3zM288 227.1C261.9 176.4 190.9 81.9 124.9 35.3 61.6-9.4 37.5-1.7 21.6 5.5 3.3 13.8 0 41.9 0 58.4S9.1 194 15 213.9c19.5 65.7 89.1 87.9 153.2 80.7 3.3-.5 6.6-.9 10-1.4-3.3 .5-6.6 1-10 1.4-93.9 14-177.3 48.2-67.9 169.9 120.3 124.6 164.8-26.7 187.7-103.4 22.9 76.7 49.2 222.5 185.6 103.4 102.4-103.4 28.1-156-65.8-169.9-3.3-.4-6.7-.8-10-1.3 3.4 .4 6.7 .9 10 1.3 64.1 7.1 133.6-15.1 153.2-80.7 5.9-19.9 15-138.9 15-155.5s-3.3-44.7-21.6-52.9c-15.8-7.1-40-14.9-103.2 29.8-66.1 46.6-137.1 141.1-163.2 191.8z"
											/>
										</svg>
										Register with Bluesky
									</Button>
								</form>
								<form method="POST" action={loginAction}>
									<input type="hidden" name="username" value="https://blacksky.app" />
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
										Register with Blacksky
									</Button>
								</form>
							</Field>
							<Field>
								<FieldDescription class="text-center">
									Already have an account? <a href={resolve('/login')}>Log in</a>
								</FieldDescription>
							</Field>
						</FieldGroup>
					</Card.Content>
				</Card.Root>
				<div class="px-6 text-center text-sm text-muted-foreground">
					<a href="https://atmosphereaccount.com" target="_blank">
						<Button variant="link" class="w-full">What is an Atmosphere account?</Button>
					</a>
				</div>
				<FieldDescription class="px-6 text-center">
					By clicking continue, you agree to our <a href={resolve('/terms')}>Terms of Service</a>
					and <a href={resolve('/privacy')}>Privacy Policy</a>.
				</FieldDescription>
			</div>
		{/if}
	</div>
</div>
