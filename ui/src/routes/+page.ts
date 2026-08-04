import type { PageLoad } from './$types';

export const load: PageLoad = async ({ url, fetch}) => {
	const query = url.searchParams.get('q');

	if (!query) {
		return { query: null, results: null };
	}

	try {
		const res = await fetch(`http://localhost:3000/api/search?q=${encodeURIComponent(query)}`);

		if (!res.ok) {
			throw new Error(`Go server responded with status: ${res.status}`);
		}

		const results = await res.json();

		return {
			query,
			results
		};
	} catch (error) {
		console.error("Search failed:", error);
		return {
			query,
			results: [],
			error: "Failed to connect to the search service."
		};
	}
};
