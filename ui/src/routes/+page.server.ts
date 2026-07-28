import type { PageServerLoad } from './$tpyes';

export const load: PageServerLoad = async ({ url, fetch}) => {
	const query = url.searchParams.get('q');

	if (!query) {
		return { query: null, results: null };
	}

	try {
		const res = await fetch(`htpp://localhost:3000/api/search?q=${encodeURIComponent(query)}`);

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
