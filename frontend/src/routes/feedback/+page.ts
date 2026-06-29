import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = () => {
	redirect(307, 'https://userinput.app/#/s/did:plc:jaur46k6ijyfvl4lojza7eic/3mob6zfay3e2d');
};
