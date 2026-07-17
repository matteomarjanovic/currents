import { STANDARD_SITE_DID, STANDARD_SITE_PUBLICATION_RKEY } from '$lib/standard-site';

export const prerender = true;

// Standard.site verification: must resolve to the publication record's at:// URI.
// https://standard.site/docs/verification
export function GET() {
	if (!STANDARD_SITE_PUBLICATION_RKEY) return new Response(null, { status: 404 });

	const uri = `at://${STANDARD_SITE_DID}/site.standard.publication/${STANDARD_SITE_PUBLICATION_RKEY}`;
	return new Response(uri, { headers: { 'content-type': 'text/plain' } });
}
