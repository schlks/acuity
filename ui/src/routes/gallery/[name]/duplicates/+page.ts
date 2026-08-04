import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url, params }) => {
	const res = await fetch(`/api/gallery/${params.name}/duplicates${url.search}`);

	if (!res.ok) {
		throw new Error(`Failed to load duplicates: ${res.status}`);
	}

	return {
		duplicatesData: await res.json()
	};
};
