<script lang="ts">
	import { invalidateAll, goto } from '$app/navigation';
	import { page, navigating } from '$app/stores';
	import { carousel } from '$lib/stores/carousel.svelte';
	import ImageCard from '$lib/components/ImageCard.svelte';
	import Spinner from '$lib/components/Spinner.svelte';

	let { data } = $props();
	let searchQuery = $state('');
	let searchThreshold = $state($page.url.searchParams.get('threshold') || '0.9');
	let infiniteScroll = $state(data.settings?.infinite_scroll || false);
	let currentPage = $derived(Number(data.images.current_page || 1));
	let currentFlag = $derived($page.url.searchParams.get('flagFilter') || '');
	let currentSort = $derived($page.url.searchParams.get('sortBy') || 'name');
	let currentOrder = $derived($page.url.searchParams.get('sortOrder') || 'asc');
	let pageButtons = $derived.by(() => {
		let p = [];
		let last = data.images.last_page || 1;
		let curr = currentPage;

		if (last <= 1) return [1];

		p.push(1);

		if (curr > 3) p.push('...');

		if (curr > 2) p.push(curr - 1);
		if (curr !== 1 && curr !== last) p.push(curr);
		if (curr < last - 1) p.push(curr + 1);

		if (curr < last - 2) p.push('...');

		p.push(last);

		return p;
	});
	let galleryImages = $state([]);
	let hasMore = $state(false);
	let currentInfinitePage = $state(1);
	let isLoadingMore = $state(false);
	let searchTimeout;

	$effect(() => {
		galleryImages = data.images.images || [];
		hasMore = data.images.has_page;
		currentInfinitePage = data.images.current_page;
		if (data.settings) {
			infiniteScroll = data.settings.infinite_scroll;
		}
	});

	function viewPort(node, callback) {
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) {
					callback();
				}
			},
			{
				rootMargin: '1500px'
			}
		);
		observer.observe(node);
		return {
			destroy() {
				observer.disconnect();
			}
		};
	}

	function applyFilter(key, value) {
		let currentUrl = new URL(location.href);
		if (currentUrl.searchParams.get(key) === value) {
			currentUrl.searchParams.delete(key);
		} else if (value) {
			currentUrl.searchParams.set(key, value);
		} else {
			currentUrl.searchParams.delete(key);
		}
		currentUrl.searchParams.set('page', 1);

		goto(currentUrl.toString());
		document.querySelector('.main-content').scrollTo({ top: 0, behavior: 'smooth' });
	}

	function changePage(newPage) {
		if (newPage < 1) return;

		goto(`?page=${newPage}`);
		document.querySelector('.main-content').scrollTo({ top: 0, behavior: 'smooth' });
	}

	function handleSearch(event) {
		if (event) event.preventDefault();

		// Nimm den Fokus vom aktuellen Element (z.B. der Suchleiste) weg
		if (document.activeElement instanceof HTMLElement) {
			document.activeElement.blur();
		}

		let currentUrl = new URL(location.href);
		if (searchQuery.trim() !== '') {
			currentUrl.searchParams.set('q', searchQuery);
			currentUrl.searchParams.set('threshold', searchThreshold);
		} else {
			currentUrl.searchParams.delete('q');
			currentUrl.searchParams.delete('threshold');
		}

		currentUrl.searchParams.set('page', 1);

		goto(currentUrl.toString());
		document.querySelector('.main-content').scrollTo({ top: 0, behavior: 'smooth' });
		document.activeElement.blur();
	}

	function handleDebounceSearch() {
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			handleSearch();
		}, 500);
	}

	async function loadMore() {
		console.log('loading more');
		if (isLoadingMore || !hasMore) return;
		isLoadingMore = true;

		let url = new URL(location.href);
		url.searchParams.set('page', currentInfinitePage + 1);

		const res = await fetch(
			`/api/gallery/${data.gallery.name}/images?${url.searchParams.toString()}`
		);
		if (res.ok) {
			const json = await res.json();
			galleryImages.push(...json.images);
			hasMore = json.has_page;
			currentInfinitePage = json.current_page;
		}
		isLoadingMore = false;
	}

	async function rescanGallery() {
		if (!confirm('Scan this gallery for new Images?')) return;
		await fetch(`/api/gallery/${data.gallery.name}/scan`, { method: 'POST' });
		await invalidateAll();
	}

	async function removeFlags() {
		if (!confirm('Remove all flags in this gallery?')) return;
		await fetch(`/api/gallery/${data.gallery.name}/unflag`, { method: 'POST' });
		await invalidateAll();
	}
</script>

<div class="gallery-header">
	<h1>{data.gallery.heading || data.gallery.name}</h1>

	<div class="header-actions">
		<button
			class="image-count"
			class:active={infiniteScroll}
			onclick={() => (infiniteScroll = !infiniteScroll)}
			title="Toggle Infinite Scrolling"
		>
			{data.gallery.count} images
		</button>

		<div class="divider"></div>

		<button class="btn-icon" title="Edit Gallery">
			<span class="material-symbols-outlined">edit</span>
		</button>
		<button class="btn-icon" title="Rescan Gallery" onclick={rescanGallery}>
			<span class="material-symbols-outlined">refresh</span>
		</button>

		<div class="divider"></div>

		<button class="btn-icon" title="Remove flags" onclick={removeFlags}>
			<span class="material-symbols-outlined">block</span>
		</button>
		<button class="btn-icon" title="Batch Actions">
			<span class="material-symbols-outlined">more_vert</span>
		</button>
	</div>

	<div class="search-container">
		<form class="search-form" onsubmit={handleSearch}>
			<div class="input-wrapper" class:active={searchQuery.trim() !== ''}>
				<span class="material-symbols-outlined search-icon">search</span>
				<input type="search" placeholder="Search within gallery..." bind:value={searchQuery} />
			</div>

			<div class="threshold-wrapper" title="Search Precision" class:active={searchThreshold != 0.9}>
				<span class="material-symbols-outlined">tune</span>
				<input
					type="number"
					min="0.1"
					max="1.0"
					step="0.05"
					bind:value={searchThreshold}
					onchange={handleDebounceSearch}
				/>
			</div>
			<button type="submit" style="display: none;"></button>
		</form>

		<div class="filter-icons">
			<button
				class="filter-pill"
				class:active={currentFlag === '1'}
				onclick={() => applyFilter('flagFilter', '1')}
				title="Only Kept"
			>
				<span class="material-symbols-outlined">favorite</span>
			</button>
			<button
				class="filter-pill"
				class:active={currentFlag === '-1'}
				onclick={() => applyFilter('flagFilter', '-1')}
				title="Only Rejected"
			>
				<span class="material-symbols-outlined">block</span>
			</button>

			<div class="divider"></div>

			<button
				class="filter-pill"
				class:active={currentSort === 'rating'}
				onclick={() => applyFilter('sortBy', 'rating')}
				title="Sort By Rating"
			>
				<span class="material-symbols-outlined">star</span>
			</button>
			<button
				class="filter-pill"
				class:active={currentSort === 'date'}
				onclick={() => applyFilter('sortBy', 'date')}
				title="Sort By Date"
			>
				<span class="material-symbols-outlined">calendar_today</span>
			</button>
			<button
				class="filter-pill"
				class:active={currentSort === 'name'}
				onclick={() => applyFilter('sortBy', 'name')}
				title="Sort By Name"
			>
				<span class="material-symbols-outlined">nature</span>
			</button>
			<button
				class="filter-pill"
				class:active={currentSort === 'name_lex'}
				onclick={() => applyFilter('sortBy', 'name_lex')}
				title="Sort By Alphabet"
			>
				<span class="material-symbols-outlined">sort_by_alpha</span>
			</button>
			<button
				class="filter-pill"
				class:active={currentSort === 'resolution'}
				onclick={() => applyFilter('sortBy', 'resolution')}
				title="Sort By Resolution"
			>
				<span class="material-symbols-outlined">photo_size_select_large</span>
			</button>
			<button
				class="filter-pill"
				class:active={currentSort === 'aspect_ratio'}
				onclick={() => applyFilter('sortBy', 'aspect_ratio')}
				title="Sort By Aspect Ratio"
			>
				<span class="material-symbols-outlined">image_aspect_ratio</span>
			</button>

			<div class="divider"></div>

			<button
				class="filter-pill"
				class:active={currentOrder === 'asc'}
				onclick={() => applyFilter('sortOrder', 'asc')}
				title="Sort Order asc"
			>
				<span class="material-symbols-outlined">arrow_upward</span>
			</button>
			<button
				class="filter-pill"
				class:active={currentOrder === 'desc'}
				onclick={() => applyFilter('sortOrder', 'desc')}
				title="Sort Order desc"
			>
				<span class="material-symbols-outlined">arrow_downward</span>
			</button>
		</div>
	</div>
</div>

{#if $navigating}
	<div class="loader-container">
		<Spinner />
	</div>
{/if}

<div class="gallery-grid medium" class:loading={$navigating}>
	<hr />
	{#each galleryImages as image, index (image.filepath)}
		<ImageCard {image} onclick={() => carousel.open(galleryImages, index)}/>
	{/each}
</div>

{#if infiniteScroll && hasMore}
	<div use:viewPort={loadMore} class="infinite-trigger">
		{#if isLoadingMore}
			<div class="loader-container">
				<Spinner />
			</div>
		{/if}
	</div>
{/if}

{#if !infiniteScroll}
	<div class="pagination">
		{#if currentPage > 1}
			<button class="filter-pill" onclick={() => changePage(currentPage - 1)}>
				<span class="material-symbols-outlined">arrow_back_ios</span>
			</button>
		{/if}
		<div class="page-numbers">
			{#each pageButtons as btn}
				{#if btn === '...'}
					<span class="page-dots">...</span>
				{:else}
					<button
						class="filter-pill"
						class:active={currentPage === btn}
						onclick={() => changePage(btn)}
					>
						{btn}
					</button>
				{/if}
			{/each}
		</div>
		{#if currentPage < data.images.last_page}
			<button class="filter-pill" onclick={() => changePage(currentPage + 1)}>
				<span class="material-symbols-outlined">arrow_forward_ios</span>
			</button>
		{/if}
	</div>
{/if}

<style>
	span {
		font-size: 0.9rem;
	}

	.gallery-header {
		display: grid;
		grid-template-columns: 1fr auto;
		grid-template-areas:
			'title search'
			'actions search';
		padding: 32px 24px 24px 24px;
		row-gap: 8px;
	}

	.gallery-header h1 {
		grid-area: title;
		font-size: 4rem;
		line-height: 1.1;
		letter-spacing: -1px;
		transform: skewX(-4deg);
		transform-origin: bottom left;
		text-transform: uppercase;
		text-decoration: underline;
		text-decoration-color: var(--secondary);
		text-decoration-thickness: 4px;
		text-underline-offset: 6px;
		padding: 0px 0px 24px 0px;
	}

	.image-count {
		background: transparent;
		border: none;
		position: relative;
		color: var(--text-muted);
		font-size: 0.9rem;
		cursor: pointer;
		padding: 4px 12px 4px 12px;
		z-index: 1;
		transition: color 0.3s ease;
	}

	.image-count::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		background: var(--primary);
		border-radius: 0px;
		z-index: -1;
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.image-count:hover {
		border-radius: 8px;
		color: var(--bg-dark);
	}

	.image-count:hover::before {
		height: 100%;
		border-radius: 8px;
	}

	.image-count.active::before {
		background: var(--tertiary);
	}

	.image-count.active:hover {
		border-radius: 8px;
		color: var(--bg-dark);
	}

	.btn-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		position: relative;
		background: transparent;
		border: 1px;
		color: var(--text-muted);
		padding: 8px;
		border: none;
		border-radius: 8px;
		cursor: pointer;
		transition:
			background-color 0.2s,
			color 0.2s,
			box-shadow 0.2s,
			border-color 0.2s;
	}

	.btn-icon::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		background: var(--primary);
		border-radius: 0px;
		z-index: -1;
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.btn-icon:hover::before {
		height: 100%;
		border-radius: 8px;
	}

	.search-container {
		grid-area: search;
		align-self: center;
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 16px;
	}

	.input-wrapper {
		flex: 1;
		position: relative;
		display: flex;
		align-items: center;
		background: transparent;
		color: var(--text-muted);
		border-radius: 8px;
		z-index: 1;
		transition: color 0.3s ease;
	}

	.input-wrapper::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		background: var(--primary);
		border-radius: 0px;
		z-index: -1;
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.input-wrapper:hover,
	.input-wrapper:focus-within {
		color: var(--bg-dark);
	}

	.input-wrapper:hover::before,
	.input-wrapper:focus-within::before {
		height: 100%;
		border-radius: 8px;
		color: var(--bg-dark);
	}

	.input-wrapper.active {
		color: var(--text);
	}

	.input-wrapper.active::before {
		height: 2px;
		border-radius: 0px;
		background: var(--tertiary);
	}

	.input-wrapper.active:hover,
	.input-wrapper.active:focus-within {
		color: var(--bg-dark);
	}

	.input-wrapper.active:hover::before,
	.input-wrapper.active:focus-within::before {
		height: 100%;
		border-radius: 8px;
		background: var(--tertiary);
	}

	.search-icon {
		position: absolute;
		left: 16px;
		color: inherit;
		font-size: 1.2rem;
		pointer-events: none;
	}

	.search-form {
		width: 100%;
		max-width: 700px;
		display: flex;
		gap: 16px;
	}

	.search-form input[type='search'] {
		width: 100%;
		padding: 14px 20px 14px 48px;
		font-size: 1rem;
		background-color: transparent;
		color: inherit;
		border: none;
		outline: none;
	}

	.input-wrapper:hover input::placeholder,
	.input-wrapper:focus-within input::placeholder {
		color: color-mix(in srgb, var(--bg-dark) 60%, transparent);
	}

	.threshold-wrapper {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 0 25px;
		position: relative;
		background: transparent;
		color: var(--text-muted);
		border-radius: 8px;
		z-index: 1;
		transition: color 0.3s ease;
	}

	.threshold-wrapper::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		background: var(--primary);
		border-radius: 0px;
		z-index: -1;
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.threshold-wrapper:hover,
	.threshold-wrapper:focus-within {
		color: var(--bg-dark);
	}

	.threshold-wrapper:hover::before,
	.threshold-wrapper:focus-within::before {
		height: 100%;
		border-radius: 8px;
	}

	.threshold-wrapper.active {
		color: var(--text);
	}

	.threshold-wrapper.active::before {
		height: 2px;
		border-radius: 0px;
		background: var(--tertiary);
	}

	.threshold-wrapper.active:hover,
	.threshold-wrapper.active:focus-within {
		color: var(--bg-dark);
	}

	.threshold-wrapper.active:hover::before,
	.threshold-wrapper.active:focus-within::before {
		height: 100%;
		border-radius: 8px;
		background: var(--tertiary);
	}

	.threshold-wrapper input[type='number'] {
		width: 50px;
		padding-right: 4px;
		background: transparent;
		border: none;
		color: inherit;
		font-size: 1rem;
		font-family: inherit;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		outline: none;
		text-align: right;
	}

	.threshold-wrapper input[type='number']::-webkit-inner-spin-button,
	.threshold-wrapper input[type='number']::-webkit-outer-spin-button {
		-webkit-appearance: none;
		margin: 0;
	}
	.threshold-wrapper input[type='number'] {
		-moz-appearance: textfield;
	}

	.header-actions {
		grid-area: actions;
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.divider {
		width: 1px;
		height: 24px;
		background-color: var(--bg-light);
		margin: 0 4px;
	}

	.gallery-grid {
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
		padding: 0 24px 24px;
	}

	.gallery-grid.small {
		--grid-base: 150px;
	}

	.gallery-grid.medium {
		--grid-base: 250px;
	}

	.gallery-grid.large {
		--grid-base: 400px;
	}

	.gallery-grid::after {
		content: '';
		flex-grow: 999;
	}

	.pagination {
		display: flex;
		justify-content: center;
		align-items: center;
		gap: 24px;
		padding: 40px 24px 80px 24px;
	}

	.page-numbers {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.page-btn {
		background: transparent;
		border: 1px solid transparent;
		color: var(--text-muted);
		font-family: inherit;
		font-size: 1rem;
		width: 40px;
		height: 40px;
		border-radius: 8px;
		cursor: pointer;
		transition: all 0.2s;
	}

	.page-btn:hover {
		background-color: var(--bg-light);
		color: var(--text);
	}

	.page-btn.active {
		background-color: color-mix(in srgb, var(--info) 20%, transparent);
		color: var(--info);
		border-color: color-mix(in srgb, var(--info) 40%, transparent);
	}

	.page-dots {
		color: var(--text-muted);
		padding: 0 4px;
		letter-spacing: 2px;
	}

	.filter-icons {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.filter-pill {
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		background: transparent;
		color: var(--text-muted);
		border: none;
		border-radius: 8px;
		width: 44px;
		height: 44px;
		cursor: pointer;
		z-index: 1;
		transition: color 0.3s ease;
	}

	.filter-pill::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		background: var(--primary);
		border-radius: 0px;
		z-index: -1;
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.filter-pill:hover {
		color: var(--bg-dark);
	}

	.filter-pill:hover::before {
		height: 100%;
		border-radius: 8px;
	}

	.filter-pill.active::before {
		height: 2px;
		border-radius: 8px;
		background: var(--tertiary);
	}

	.filter-pill.active:hover {
		color: var(--bg-dark);
	}

	.filter-pill.active:hover::before {
		height: 100%;
		border-radius: 8px;
	}

	.loader-container {
		display: flex;
		justify-content: center;
		align-items: center;
		padding: 40px;
		width: 100%;
	}

	.gallery-grid {
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
		padding: 0 24px 24px;
		transition: opacity 0.3s ease;
	}

	.gallery-grid.loading {
		pointer-events: none;
	}

	.gallery-grid.loading :global(img) {
		opacity: 0;
	}

	.infinite-trigger {
		width: 100%;
		height: 100px;
		display: flex;
		justify-content: center;
		align-items: center;
	}
</style>
