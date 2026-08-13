<script lang="ts">
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { invalidateAll } from '$app/navigation';
	import { carousel } from '$lib/stores/carousel.svelte';
	import ImageCard from '$lib/components/ImageCard.svelte';
	import BatchActionDialog from '$lib/components/BatchActionDialog.svelte';
	import Spinner from '$lib/components/Spinner.svelte';

	let searchThreshold = $state(page.url.searchParams.get('threshold') || '0.1');
	let searchTimeout: ReturnType<typeof setTimeout>;
	let isDropdownOpen = $state(false);
	let batchIsOpen = $state(false);
	let selectedImages = $state<any[]>([]);
	let selectedGalleries = $state<string[]>(page.url.searchParams.getAll('galleries'));

	let duplicatesData = $state<any>(null);
	let isLoading = $state(true);
	let abortController: AbortController | null = null;
	let lastUrl = '';
    let isKeyboardMode = $state(false);
    let focusedIndex = $state(0);

	let galleries = $derived(duplicatesData?.Galleries || page.data?.galleries || []);

    let flatImages = $derived.by(() => {
        if (!duplicatesData) return [];
        if (duplicatesData.Images && duplicatesData.Images.length > 0) {
            return duplicatesData.Images;
        }
        if (duplicatesData.Groups) {
            return duplicatesData.Groups.flat();
        }
        return [];
    })

	async function loadDuplicates() {
		if (abortController) {
			abortController.abort();
		}
		const controller = new AbortController();
		abortController = controller;

		isLoading = true;
		try {
			const params = new URLSearchParams();
			params.set('threshold', searchThreshold.toString());

			const imageID = page.url.searchParams.get('imageID');
			if (imageID) {
				params.set('imageID', imageID);
			}

			selectedGalleries.forEach(g => params.append('galleries', g));

			const res = await fetch(`/api/gallery/${page.params.name}/duplicates?${params.toString()}`, { signal: controller.signal });
			if (res.ok) {
				const data = await res.json();
				if (abortController === controller) {
					duplicatesData = data;
				}
			} else {
				if (abortController === controller) {
					duplicatesData = { Groups: [], Galleries: [] };
				}
			}
		} catch (e: any) {
			if (e.name === 'AbortError') return;
			console.error('Failed to load duplicates:', e);
			if (abortController === controller) {
				duplicatesData = { Groups: [], Galleries: [] };
			}
		} finally {
			if (abortController === controller) {
				isLoading = false;
			}
		}
	}

	$effect(() => {
		const currentHref = page.url.href;
		if (currentHref !== lastUrl) {
			lastUrl = currentHref;
			untrack(() => {
				const currentUrl = page.url;
				searchThreshold = currentUrl.searchParams.get('threshold') || '0.1';
				selectedGalleries = currentUrl.searchParams.getAll('galleries');
				selectedImages = [];
				loadDuplicates();
			});
		}
	});

	function toggleSelect(image: any) {
		if (selectedImages.some(i => i.id === image.id)) {
			selectedImages = selectedImages.filter(i => i.id !== image.id);
		} else {
			selectedImages = [...selectedImages, image];
		}
	}

	function toggleGallery(id: number) {
		const idStr = id.toString();
		if (selectedGalleries.includes(idStr)) {
			selectedGalleries = selectedGalleries.filter(g => g !== idStr);
		} else {
			selectedGalleries = [...selectedGalleries, idStr];
		}
		loadDuplicates();
	}

	function handleDebounceSearch() {
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			loadDuplicates();
		}, 400);
	}

	async function runBatchAction(action: 'delete' | 'move' | 'copy', criteria = 'selected') {
		if (criteria === 'selected' && selectedImages.length === 0) return;

		if (action === 'delete') {
			const count = criteria === 'selected' ? `${selectedImages.length} selected` : 'all matching';
			if (!confirm(`Do you really want to delete ${count} images?`)) return;
		}

		const formData = new FormData();
		formData.append('criteria', criteria);
		formData.append('batch_action', action);
		formData.append('source_gallery', page.params.name || '');

		if (criteria === 'selected') {
			const ids = selectedImages.map(img => img.id ?? img.ID).join(',');
			formData.append('image_ids', ids);
		}

		const res = await fetch('/api/gallery/batch', {
			method: 'POST',
			body: formData
		});

		if (res.ok) {
			await invalidateAll();
			selectedImages = [];
			await loadDuplicates();
		} else {
			console.error('Batch Action failed:', await res.text());
		}
	}

        function handleMouseMove(e: MouseEvent) {
                if (Math.abs(e.movementX) > 0 || Math.abs(e.movementY) > 0) {
                        isKeyboardMode = false;
                }
        }

        function navigate(step: number) {
                isKeyboardMode = true;
                const nextIndex = focusedIndex + step;
                if (nextIndex >= 0 && nextIndex < flatImages.length) {
                        const el = document.querySelector(`.duplicates-container [data-index="${nextIndex}"]`);
                        el?.scrollIntoView({ block: 'nearest', behavior: 'smooth'});
                        focusedIndex = nextIndex;
                }
        }

        function navigateVert(direction: 'up' | 'down' ) {
                isKeyboardMode = true;
                const currentEl = document.querySelector(`.duplicates-container [data-index="${focusedIndex}"]`) as HTMLElement;
                if (!currentEl) return;

                const currLeft = currentEl.offsetLeft;
                const currRight = currLeft + currentEl.offsetWidth;
                const currCenter = (currLeft + currRight) / 2;
                const currTop = currentEl.offsetTop;

                const allCards = Array.from(document.querySelectorAll('.duplicates-container [data-index]')) as HTMLElement[];
                let targetRowCards: HTMLElement[] = [];

                if (direction === 'down') {
                        const belowTops = allCards.map(c => c.offsetTop).filter(top => top > currTop + 20);
                        if (belowTops.length === 0) return;
                        const nextRowTop = Math.min(...belowTops);
                        targetRowCards = allCards.filter(c => Math.abs(c.offsetTop - nextRowTop) < 20);
                } else {
                        const aboveTops = allCards.map(c => c.offsetTop).filter(top => top < currTop - 20);
                        if (aboveTops.length === 0) return;
                        const prevRowTop = Math.max(...aboveTops);
                        targetRowCards = allCards.filter(c => Math.abs(c.offsetTop - prevRowTop) < 20);
                }

                let bestCard: HTMLElement | null = null;
                let highestScore = -Infinity;

                for (const card of targetRowCards) {
                        const candLeft = card.offsetLeft;
                        const candRight = candLeft + card.offsetWidth;
                        const candCenter = (candLeft + candRight) / 2;

                        const overlap = Math.max(0, Math.min(currRight, candRight) - Math.max(currLeft, candLeft));
                        const distance = Math.abs(candCenter - currCenter);
                        const score = overlap - distance;

                        if (score > highestScore) {
                                highestScore = score;
                                bestCard = card;
                        }
                }

                if (bestCard) {
                        const attr = bestCard.getAttribute('data-index');
                        const newIndex = attr !== null ? Number(attr) : NaN;
                        if (!isNaN(newIndex)) {
                                bestCard.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
                                focusedIndex = newIndex;
                        }
                }
        }

        function openFocusedInCarousel() {
                if (flatImages.length === 0 || focusedIndex < 0 || focusedIndex >= flatImages.length) return;
                const targetImage = flatImages[focusedIndex];
                if (duplicatesData.Images && duplicatesData.Images.length > 0) {
                        const idx = duplicatesData.Images.indexOf(targetImage);
                        if (idx !== -1) carousel.open(duplicatesData.Images, idx);
                } else if (duplicatesData.Groups) {
                        for (const group of duplicatesData.Groups) {
                                const idx = group.indexOf(targetImage);
                                if (idx !== -1) {
                                        carousel.open(group, idx);
                                        break;
                                }
                        }
                }
        }

	function handleKeyDown(e: KeyboardEvent) {
		const tag = (e.target as HTMLElement)?.tagName;
		if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;

		if (carousel.isOpen || batchIsOpen) return;

		switch (e.key) {
                        case 'ArrowRight':
                        case 'l':
                        case 'd':
                            navigate(1);
                            break;
                        case 'ArrowLeft':
                        case 'h':
                        case 'a':
                            navigate(-1);
                            break;
                        case 'ArrowDown':
                        case 'j':
                        case 's':
                            e.preventDefault();
                            navigateVert('down');
                            break;
                        case 'ArrowUp':
                        case 'k':
                        case 'w':
                            e.preventDefault();
                            navigateVert('up');
                            break;
                        case 'Enter':
                            openFocusedInCarousel();
                            break;
                        case ' ':
                                e.preventDefault();
                                if (flatImages[focusedIndex]) {
                                        toggleSelect(flatImages[focusedIndex]);
                                }
                                break;
			case 'b':
				batchIsOpen = !batchIsOpen;
				break;
			case 'Delete':
			case 'Backspace':
				e.preventDefault();
				if (selectedImages.length > 0) {
					runBatchAction('delete', 'selected');
				}
				break;
		}
	}

	function handleGlobalClick(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (!target || typeof target.closest !== 'function') return;

		if (!target.closest('.batch-dialog, .dropdown-menu, .gallery-filter-dropdown, .btn-icon')) {
			batchIsOpen = false;
			isDropdownOpen = false;
		}
	}
</script>

<svelte:window onclick={handleGlobalClick} onkeydown={handleKeyDown} onmousemove={handleMouseMove} />

<div class="duplicates-container">
	<div class="header-section">
		{#if page.url.searchParams.has('imageID')}
			<h1>Similar Images</h1>
		{:else}
			<h1>Duplicates</h1>
		{/if}

		<form method="GET" class="control-bar" onsubmit={(e) => e.preventDefault()}>
			<div style="position: relative;">
				<button
					type="button"
					class="btn-icon"
					class:active={batchIsOpen}
					title="Batch Actions (b)"
					onclick={() => batchIsOpen = !batchIsOpen}
				>
					<span class="material-symbols-outlined">more_vert</span>
				</button>

				{#if batchIsOpen}
					<BatchActionDialog
						{selectedImages}
						onClose={() => batchIsOpen = false}
					/>
				{/if}
			</div>

			{#if selectedImages.length > 0}
				<span class="selected-count">{selectedImages.length}</span>
			{/if}

			<div class="threshold-wrapper" title="Search Precision" class:active={searchThreshold != '0.1'}>
				<span class="material-symbols-outlined">tune</span>
				<input
					type="number"
					min="0.01"
					max="1.0"
					step="0.01"
					bind:value={searchThreshold}
					oninput={handleDebounceSearch}
				/>
			</div>

			{#if galleries.length > 0}
				<div class="dropdown-wrapper">
					<button type="button" class="dropdown-btn" onclick={() => isDropdownOpen = !isDropdownOpen}>
						<span>Galleries ({selectedGalleries.length})</span>
						<span class="material-symbols-outlined">expand_more</span>
					</button>
					
					{#if isDropdownOpen}
						<div class="dropdown-menu">
							{#each galleries as gallery}
								<label class="gallery-checkbox">
									<input
										type="checkbox"
										checked={selectedGalleries.includes(gallery.id?.toString()) || (duplicatesData?.SelectedGalleries && duplicatesData.SelectedGalleries[gallery.id])}
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
	
	{#if isLoading}
		<div class="loading-state">
			<Spinner />
			<p>Searching for duplicates...</p>
		</div>
	{:else if duplicatesData?.Images && duplicatesData.Images.length > 0}
		<div class="duplicates-group">
			<div class="group-header">
				<h3>Duplicates for selected Image</h3>
				<span class="badge">{duplicatesData.Images.length} Images</span>
			</div>
			<div class="gallery-grid">
				{#each duplicatesData.Images as image, imgIndex}
					<ImageCard 
						{image}
						showMeta={true}
						isSelected={selectedImages.some(i => i.id === image.id)}
						dataIndex={imgIndex}
						isFocused={isKeyboardMode && focusedIndex === imgIndex}
						onmouseenter={() => {
							if (!isKeyboardMode) focusedIndex = imgIndex;
						}}
						onclick={(e) => {
							if (e.shiftKey) {
								toggleSelect(image);
							} else {
								carousel.open(duplicatesData.Images, imgIndex);
							}
						}}
					/>
				{/each}
                <div class="flex-spacer"></div>
			</div>
		</div>
	{:else if duplicatesData?.Groups && duplicatesData.Groups.length > 0}
		{#each duplicatesData.Groups as group, index}
			<div class="duplicates-group">
				<div class="group-header">
					<h3>Set {index + 1}</h3>
					<span class="badge">{group.length} Images</span>
				</div>
				<div class="gallery-grid">
					{#each group as image, imgIndex}
						{@const globalIndex = flatImages.indexOf(image)}
						<ImageCard 
							{image}
							isSelected={selectedImages.some(i => i.id === image.id)}
							dataIndex={globalIndex}
							isFocused={isKeyboardMode && focusedIndex === globalIndex}
							onmouseenter={() => {
								if (!isKeyboardMode) focusedIndex = globalIndex;
							}}
							onclick={(e: { shiftKey: any; }) => {
								if (e.shiftKey) {
									toggleSelect(image);
								} else {
									carousel.open(group, imgIndex);
								}
							}}
							showMeta={true}
						/>
					{/each}
                    <div class="flex-spacer"></div>
				</div>
			</div>
		{/each}
	{:else}
		<div class="empty-state">
			<span class="material-symbols-outlined icon-large">task_alt</span>
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
		padding-bottom: 24px;
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
		padding: 0 0 24px 0;
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
		display: flex;
		align-items: center;
		gap: 16px;
	}

	.selected-count {
		position: relative;
		color: var(--text-muted);
		font-size: 0.9rem;
		padding: 4px 12px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 28px;
		cursor: default;
		user-select: none;
		z-index: 1;
	}

	.selected-count::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		background: var(--info);
		z-index: -1;
	}

	.threshold-wrapper {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 0 25px;
		height: 40px;
		box-sizing: border-box;
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
		border-radius: 0;
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
		border-radius: 0;
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
        appearance: textfield;
		-moz-appearance: textfield;
	}

	.btn-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		position: relative;
		background: transparent;
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

	.btn-icon:hover {
		color: var(--bg-dark);
	}

	.btn-icon::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		background: var(--primary);
		border-radius: 0;
		z-index: -1;
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.btn-icon:hover::before {
		height: 100%;
		border-radius: 8px;
	}

	.btn-icon.active::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		background: var(--tertiary);
		border-radius: 0;
		z-index: -1;
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.btn-icon.active:hover::before {
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

    .flex-spacer {
        flex-grow: 99999;
        flex-basis: 0;
        height: 0;
    }

	.loading-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 80px 0;
		gap: 16px;
		color: var(--text-muted);
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

	.dropdown-wrapper {
		position: relative;
	}

	.dropdown-btn {
		display: flex;
		align-items: center;
		justify-content: space-between;
		min-width: 180px;
		height: 40px;
		box-sizing: border-box;
		padding: 8px 12px;
		background: var(--bg-light);
		border: 1px solid var(--border);
		border-radius: 8px;
		color: var(--text);
		cursor: pointer;
		font-size: 0.9rem;
	}

	.dropdown-menu {
		position: absolute;
		top: calc(100% + 4px);
		right: 0;
		background: var(--bg-dark);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 8px;
		min-width: 200px;
		z-index: 100;
		box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
		max-height: 250px;
		overflow-y: auto;
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
