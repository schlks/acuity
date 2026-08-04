export async function load({ fetch, params, url }) {
	const page = url.searchParams.get('page') || '1';
	const flagFilter = url.searchParams.get('flagFilter') || '';
	const sortBy = url.searchParams.get('sortBy') || '';
	const sortOrder = url.searchParams.get('sortOrder') || '';
	const q = url.searchParams.get('q') || '';
	const threshold = url.searchParams.get('threshold') || '0.9';

	let imagesRes;

	if (q) {
		const formData = new FormData();
		formData.append('q', q);
		formData.append('threshold', threshold);
		imagesRes = await fetch(`/api/gallery/${params.name}/search`, {
			method: 'POST',
			body: formData
		})
	} else {
		const query = new URLSearchParams();
		query.set('page', page);
		if (flagFilter) query.set('flagFilter', flagFilter);
		if (sortBy) query.set('sortBy', sortBy);
		if (sortOrder) query.set('sortOrder', sortOrder);

		imagesRes = await fetch(`/api/gallery/${params.name}/images?${query.toString()}`);
	}

	const [galleryRes, settingsRes] = await Promise.all([
		fetch(`/api/gallery/${params.name}`),
		fetch(`/api/settings`)
	]);

	const galleryData = await galleryRes.json();
	const imagesData = await imagesRes.json();
	const settingsData = await settingsRes.json();

	return {
		gallery: galleryData,
		images: imagesData,
		settings: settingsData
	};
}
