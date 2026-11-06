import { defineEnvVars } from '@sveltejs/kit/env';

export const variables = defineEnvVars({
	APPVIEW_INTERNAL_URL: { schema: (value) => value ?? '' },
	PUBLIC_APPVIEW_URL: { public: true, static: true, schema: (value) => value ?? '' },
	PUBLIC_WEB_APPVIEW_URL: { public: true, static: true, schema: (value) => value ?? '' },
	PUBLIC_POLAR_PRODUCT_MONTHLY: { public: true, static: true, schema: (value) => value ?? '' },
	PUBLIC_POLAR_PRODUCT_YEARLY: { public: true, static: true, schema: (value) => value ?? '' },
	PUBLIC_UMAMI_SCRIPT_URL: { public: true, static: true, schema: (value) => value ?? '' },
	PUBLIC_UMAMI_WEBSITE_ID: { public: true, static: true, schema: (value) => value ?? '' },
	PUBLIC_INTERACTIVE_WIDGET: {
		public: true,
		static: true,
		schema: (value) => value ?? 'resizes-visual'
	}
});
