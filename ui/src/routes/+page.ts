import type { PageLoad } from './$types';
import { error } from '@sveltejs/kit';

export const load: PageLoad = async ({ url, fetch}) => {
	const query = url.searchParams.get('q');

	if (!query) {
		return { query: null, results: null };
	}

	const res = await fetch(`/api/search?q=${encodeURIComponent(query)}`);

	if (!res.ok) {
		throw error(res.status, 'Failed to query the server for a search');
	}

	const results = await res.json();

	return {
		query,
		results
	};
};
