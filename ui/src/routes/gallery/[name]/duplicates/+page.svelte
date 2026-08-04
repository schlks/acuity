<script lang="ts">
	import type { PageData } from './$types';
	import ImageCard from '$lib/components/ImageCard.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	let { data }: { data: PageData } = $props();
	let searchThreshold = $state($page.url.searchParams.get('threshold') || '0.1');
	let searchTimeout: ReturnType<typeof setTimeout>;
	let isDropdownOpen = $state(false);

	function toggleGallery(id: number) {
		let url = new URL(location.href);
		let currentGalleries = url.searchParams.getAll('galleries');
		let idStr = id.toString();

		if (currentGalleries.includes(idStr)) {
			currentGalleries = currentGalleries.filter(g => g !== idStr);
		} else {
			currentGalleries.push(idStr);
		}

		url.searchParams.delete('galleries');
		currentGalleries.forEach(g => url.searchParams.append('galleries', g));

		goto(url.toString(), { keepFocus: true, noScroll: true});
	}

	function handleDebounceSearch() {
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			let url = new URL(location.href);
			url.searchParams.set('threshold', searchThreshold.toString());

			goto(url.toString(), { keepFocus: true, noScroll: true });
		}, 500);
	}
</script>

<div class="duplicates-container">
	<div class="header-section">
		{#if $page.url.searchParams.has('imageID')}
			<h1>Similar Images</h1>
		{:else}
			<h1>Duplicates</h1>
		{/if}

		<form method="GET" class="control-bar">
			{#if $page.url.searchParams.has('imageID')}
				<input type="hidden" name="imageID" value={$page.url.searchParams.get('imageID')} />
			{/if}

			<div class="threshold-wrapper" title="Search Precision" class:active={searchThreshold != 0.1}>
				<span class="material-symbols-outlined">tune</span>
				<input
					type="number"
					min="0.01"
					max="1.0"
					step="0.01"
					bind:value={searchThreshold}
					onchange={handleDebounceSearch}
				/>
			</div>
			{#if data.duplicatesData.Galleries}
				<div class="dropdown-wrapper">
					<button type="button" class="dropdown-btn" onclick={() => isDropdownOpen = !isDropdownOpen}>
						<span>Galleries ({Object.keys(data.duplicatesData.SelectedGalleries || {}).length})</span>
						<span class="material-symbols-outlined">expand_more</span>
					</button>
					
					{#if isDropdownOpen}
						<div class="dropdown-menu">
							{#each data.duplicatesData.Galleries as gallery}
								<label class="gallery-checkbox">
									<input
										type="checkbox"
										checked={data.duplicatesData.SelectedGalleries && data.duplicatesData.SelectedGalleries[gallery.id]}
										onchange={() => toggleGallery(gallery.id)}
									/>
									<span>{gallery.name}</span>
								</label>
							{/each}
						</div>
					{/if}
				</div>
			{/if}
		</form>
	</div>
	
	<hr />

	{#if data.duplicatesData.Images && data.duplicatesData.Images.length > 0}
		<div class="duplicates-group">
			<div class="group-header">
				<h3>Duplicates for selected Image</h3>
				<span class="badge">{data.duplicatesData.Images.length} Images</span>
			</div>
			<div class="gallery-grid">
				{#each data.duplicatesData.Images as image}
					<ImageCard {image} />
				{/each}
			</div>
		</div>
	{:else if data.duplicatesData.Groups && data.duplicatesData.Groups.length > 0}
		{#each data.duplicatesData.Groups as group, index}
			<div class="duplicates-group">
				<div class="group-header">
					<h3>Set {index + 1}</h3>
					<span class="badge">{group.length} Images</span>
				</div>
				<div class="gallery-grid">
					{#each group as image}
						<ImageCard {image} />
					{/each}
				</div>
			</div>
		{/each}
	{:else}
		<div class="empty-state">
			<span class="material-symbols-outlined icon-large">check_circle</span>
			<p>No Duplicates found within the current threshold.</p>
		</div>
	{/if}
</div>

<style>
	.duplicates-container {
		padding: 24px;
		margin: 0 auto;
		width: 100%;
		flex: 1;
	}
	
	.header-section {
		display: flex;
		justify-content: space-between;
		align-items: center;
		border-bottom: 1px solid var(--border);
		padding-bottom: 24px 24px 0px 24px;
	}

	.header-section h1 {
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
	
	h3 {
		text-decoration: underline;
		text-decoration-color: var(--secondary);
		text-decoration-thickness: 2px;
		text-underline-offset: 3px;

	}

	.control-bar {
		padding: 12px 24px;
		border-radius: 12px;
		border: 1px solid var(--border);
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

	.threshold-wrapper input[type="number"] {
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

	.threshold-wrapper input[type="number"]::-webkit-inner-spin-button,
	.threshold-wrapper input[type="number"]::-webkit-outer-spin-button {
		-webkit-appearance: none;
		margin: 0;
	}
	.threshold-wrapper input[type="number"] {
		-moz-appearance: textfield;
	}

	.label-text {
		font-size: 0.95rem;
		color: var(--text-muted);
	}

	input[type="number"] {
		background-color: var(--bg);
		color: var(--text);
		border: 1px solid var(--bg);
		padding: 8px 12px;
		border-radius: 8px;
		font-family: inherit;
		font-size: 0.95rem;
		width: 80px;
		outline: none;
		transition: border-color 0.2s;
	}

	input[type=number]:focus {
		border-color: var(--primary);
	}

	button.btn-primary {
		position: relative;
		z-index: 1;
		font-family: inherit;
		font-size: 0.95rem;
		border-radius: 8px;
		padding: 8px 24px;
		cursor: pointer;
		border: none;
		background: transparent;
		color: var(--text);
		transition: color 0.3s;
	}

	button.btn-primary::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		border-radius: 0px;
		background: var(--primary);
		z-index: -1;
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
	}

	button.btn-primary:hover {
		color: var(--bg-dark);
	}

	button.btn-primary:hover::before {
		height: 100%;
		border-radius: 8px;
	}

	.duplicates-group {
		margin-bottom: 24px;
		background: var(--bg);
		padding: 24px;
		border: 1px solid var(--border);
		border-radius: 12px;
	}

	.group-header {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 20px;
	}

	.group-header h3 {
		margin: 0;
		color: var(--text);
		font-weight: 500;
	}

	.badge {
		background: var(--bg-dark);
		color: var(--text-muted);
		padding: 4px 12px;
		border-radius: 20px;
		font-size: 0.85rem;
		font-weight: 600;
		text-decoration: underline;
		text-decoration-color: var(--info);
		text-decoration-thickness: 2px;
		text-underline-offset: 3px;
	}

	.gallery-grid {
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
	}

	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 64px 0;
		color: var(--text-muted);
	}

	.icon-large {
		font-size: 4rem;
		margin-bottom: 16px;
		color: var(--info);
		opacity: 0.8;
	}

	hr {
		padding: 0px 0px 12px 0px;
	}

	.dropdown-wrapper {
		position: relative;
	}

	.dropdown-btn {
		display: flex;
		align-items: center;
		justify-content: space-between;
		min-width: 180px;
		padding: 8px 12px;
		background: var(--bg-light);
		border: 1px solid var(--border);
		border-radius: 8px;
		color: var(--text);
		cursor: pointer;
		font-size: 0.9rem;
	}

	.gallery-checkbox {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px;
		border-radius: 4px;
		cursor: pointer;
		font-size: 0.9rem;
		transition: background 0.2s;
	}

	.gallery-checkbox:hover {
		background: var(--bg);
	}
</style>
