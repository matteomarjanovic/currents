<script lang="ts">
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { resolve } from '$app/paths';
	import LogoMerged from '$lib/assets/logo.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { FieldGroup, Field, FieldDescription } from '$lib/components/ui/field/index.js';

	const loginAction = `${PUBLIC_APPVIEW_URL}/oauth/login`;
</script>

<div class="flex min-h-svh flex-col items-center justify-center gap-6 bg-muted p-6 md:p-10">
	<div class="flex w-full max-w-sm flex-col gap-6">
		<a href={resolve('/')} class="flex h-5 items-center gap-2 self-center font-medium">
			<LogoMerged />
		</a>
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
								<Button variant="outline" type="submit" class="w-full justify-start">
									<svg
										fill="currentColor"
										viewBox="77.86 113.9 129.15 129.15"
										xmlns="http://www.w3.org/2000/svg"
									>
										<path
											d="M148.846 144.562C148.846 159.75 161.158 172.062 176.346 172.062H207.012V185.865H176.346C161.158 185.865 148.846 198.177 148.846 213.365V243.045H136.029V213.365C136.029 198.177 123.717 185.865 108.529 185.865H77.8633V172.062H108.529C123.717 172.062 136.029 159.75 136.029 144.562V113.896H148.846V144.562Z"
										></path>
									</svg>
									Register with Eurosky
									<span
										class="ml-auto rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
									>
										Recommended
									</span>
								</Button>
							</form>
							<form method="POST" action={loginAction}>
								<input type="hidden" name="username" value="https://bsky.social" />
								<Button variant="outline" type="submit" class="w-full justify-start">
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
				By clicking continue, you agree to our <a href="##">Terms of Service</a>
				and <a href="##">Privacy Policy</a>.
			</FieldDescription>
		</div>
	</div>
</div>
