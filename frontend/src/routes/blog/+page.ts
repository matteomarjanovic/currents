interface PostMetadata {
	title: string;
	description: string;
	date: string;
}

export const load = () => {
	const modules = import.meta.glob<{ metadata: PostMetadata }>('/src/routes/blog/*/+page.{svx,md}', {
		eager: true
	});

	const posts = Object.entries(modules)
		.map(([path, mod]) => ({ slug: path.split('/').at(-2)!, ...mod.metadata }))
		.sort((a, b) => b.date.localeCompare(a.date));

	return { posts };
};
