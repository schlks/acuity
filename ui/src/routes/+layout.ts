import { error } from '@sveltejs/kit';

export const ssr = false;
export const prerender = false;

export async function load({ fetch }) {
	const response = await fetch('/api/root');
	if (!response.ok) {
		throw error(response.status, 'Failed to load root data');
	}

	return await response.json();
}
