<script lang="ts">
	import { fade } from 'svelte/transition';

	let {
		isOpen = $bindable(false),
		images = $bindable([]),
		currentIndex = $bindable(0),
		onClose
	} = $props();

	let currentImage = $derived(images[currentIndex]);
	let filmstripContainer = $state<HTMLElement | null>(null);
	let focusBox = $state({ x: 0, y: 0, w: 0, h: 0, visible: false });

	function updateFocusBox() {
		if (!filmstripContainer) return;
		const activeThumb = filmstripContainer.querySelector(`[data-thumb-index="${currentIndex}"]`) as HTMLElement;
		if (activeThumb) {
			focusBox = {
				x: activeThumb.offsetLeft,
				y: activeThumb.offsetTop,
				w: activeThumb.offsetWidth,
				h: activeThumb.offsetHeight,
				visible: true
			};
		} else {
			focusBox.visible = false;
		}
	}

	$effect(() => {
		if (isOpen && filmstripContainer) {
			requestAnimationFrame(() => updateFocusBox());
			const activeThumb = filmstripContainer.querySelector(`[data-thumb-index="${currentIndex}"]`) as HTMLElement;
			activeThumb?.scrollIntoView({ inline: 'center', block: 'nearest', behavior: 'auto' });
		}
	});

	function next() {
		if (currentIndex < images.length - 1) {
			currentIndex++;
		}
	}

	function prev() {
		if (currentIndex > 0) {
			currentIndex--;
		}
	}

	function close() {
		isOpen = false;
		onClose?.();
	}

	async function setFlagAndAdvance(flag: number) {
		if (!currentImage) return;

		const newFlag = currentImage.flag === flag ? 0 : flag;
		const formData = new FormData();
		formData.append('flag', newFlag.toString());

		try {
			const res = await fetch(`/api/image/${currentImage.id}/flag`, { method: 'POST', body: formData });
			if (res.ok) {
				images[currentIndex] = { ...currentImage, flag: newFlag };
			}
			next();
		} catch (e) {
			console.error('Flag error:', e);
		}
	}

	async function setRatingAndAdvance(rating: number) {
		if (!currentImage) return;

		const formData = new FormData();
		formData.append('rating', rating.toString());

		try {
			const res = await fetch(`/api/image/${currentImage.id}`, { method: 'POST', body: formData });
			if (res.ok) {
				images[currentIndex] = { ...currentImage, rating };
			}
			next();
		} catch (e) {
			console.error('Rating error:', e);
		}
	}

	function nextUnflagged() {
		const nextIdx = images.findIndex((img, idx) => idx > currentIndex && !img.flag);
		if (nextIdx !== -1) {
			currentIndex = nextIdx;
		}
	}

	function prevUnflagged() {
		const prevIdx = images.findLastIndex((img, idx) => idx < currentIndex && !img.flag);
		if (prevIdx !== -1) {
			currentIndex = prevIdx;
		}
	}

	function handleKeyDown(e: KeyboardEvent) {
		if (!isOpen) return;

		const tag = (e.target as HTMLElement)?.tagName;
		if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;

		switch (e.key) {
			case 'ArrowRight':
			case 'L':
			case 'l':
				e.preventDefault();
				if (e.shiftKey) {
					nextUnflagged();
				} else {
					next();
				}
				break;
			case 'ArrowLeft':
			case 'h':
				e.preventDefault();
				if (e.shiftKey) {
					prevUnflagged();
				} else {
					prev();
				}
				break;
			case 'Escape':
				e.preventDefault();
				close();
				break;
			case ' ':
			case 'm':
				e.preventDefault();
				setFlagAndAdvance(1);
				break;
			case 'Backspace':
			case 'n':
				e.preventDefault();
				setFlagAndAdvance(-1);
				break;
			case 'u':
				e.preventDefault();
				setFlagAndAdvance(0);
				break;
			case '0': case '1': case '2': case '3': case '4': case '5':
				e.preventDefault();
				setRatingAndAdvance(Number(e.key));
				break;
		}
	}
</script>

<svelte:window onkeydown={handleKeyDown} />

{#if isOpen && currentImage}
	<div class="culling-overlay" transition:fade={{ duration: 150 }}>
		<!-- Header -->
		<header class="culling-header">
			<div class="header-left">
				<div class="mode-tag">
					<span class="material-symbols-outlined icon-filled">bolt</span>
					<span>Culling Mode</span>
				</div>
				<span class="counter">{currentIndex + 1} / {images.length}</span>
			</div>

			<div class="header-shortcuts">
				<span class="shortcut-hint"><kbd>Space</kbd>/<kbd>m</kbd> Keep</span>
				<span class="shortcut-hint"><kbd>⌫</kbd>/<kbd>n</kbd> Reject</span>
				<span class="shortcut-hint"><kbd>0-5</kbd> Star</span>
				<span class="shortcut-hint"><kbd>←</kbd><kbd>→</kbd> Navigate</span>
			</div>

			<button class="close-btn" onclick={close} title="Close Culling Mode (Esc / c)">
				<span class="material-symbols-outlined">close</span>
			</button>
		</header>

		<!-- Main Image View -->
		<main class="culling-main">
			<div class="image-wrapper">
				<img
					src="/api/image?path={encodeURIComponent(currentImage.filepath)}"
					alt=""
					class="main-image"
				/>

				<!-- Flag Badge Overlay -->
				{#if currentImage.flag === 1}
					<div class="status-badge keep">
						<span class="material-symbols-outlined icon-filled">check_circle</span>
						<span>Keep</span>
					</div>
				{:else if currentImage.flag === -1}
					<div class="status-badge reject">
						<span class="material-symbols-outlined icon-filled">cancel</span>
						<span>Reject</span>
					</div>
				{/if}

				<!-- Rating Overlay -->
				{#if currentImage.rating && currentImage.rating > 0}
					<div class="rating-badge">
						<span class="material-symbols-outlined icon-filled star-icon">star</span>
						<span>{currentImage.rating}</span>
					</div>
				{/if}
			</div>
		</main>

		<!-- Bottom Filmstrip -->
		<footer class="culling-filmstrip" bind:this={filmstripContainer}>
			{#if focusBox.visible}
				<div
					class="floating-focus"
					style="transform: translate3d({focusBox.x}px, {focusBox.y}px, 0); width: {focusBox.w}px; height: {focusBox.h}px;"
				></div>
			{/if}

			{#each images as img, i (img.filepath || i)}
				<button
					class="filmstrip-thumb"
					class:active={currentIndex === i}
					data-thumb-index={i}
					onclick={() => currentIndex = i}
				>
					<img src="/api/image?path={encodeURIComponent(img.filepath)}&thumb=true" alt="" loading="lazy" />
					
					{#if img.flag === 1}
						<span class="thumb-flag keep">
							<span class="material-symbols-outlined icon-filled">check_circle</span>
						</span>
					{:else if img.flag === -1}
						<span class="thumb-flag reject">
							<span class="material-symbols-outlined icon-filled">cancel</span>
						</span>
					{/if}

					{#if img.rating && img.rating > 0}
						<span class="thumb-rating">
							<span class="material-symbols-outlined icon-filled">star</span>{img.rating}
						</span>
					{/if}
				</button>
			{/each}
		</footer>
	</div>
{/if}

<style>
	.culling-overlay {
		position: fixed;
		inset: 0;
		background: rgba(10, 10, 12, 0.98);
		z-index: 2000;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		user-select: none;
	}

	.culling-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 24px;
		background: rgba(0, 0, 0, 0.4);
		border-bottom: 1px solid var(--border);
		z-index: 10;
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: 16px;
	}

	.mode-tag {
		display: flex;
		align-items: center;
		gap: 6px;
		color: var(--info);
		font-weight: 700;
		font-size: 0.95rem;
		background: color-mix(in srgb, var(--info) 12%, transparent);
		padding: 4px 10px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--info) 30%, transparent);
	}

	.counter {
		color: var(--text-muted);
		font-size: 0.9rem;
		font-variant-numeric: tabular-nums;
	}

	.header-shortcuts {
		display: flex;
		align-items: center;
		gap: 14px;
	}

	.shortcut-hint {
		color: var(--text-muted);
		font-size: 0.8rem;
		display: flex;
		align-items: center;
		gap: 4px;
	}

	kbd {
		background: var(--bg-dark);
		border: 1px solid var(--border);
		padding: 1px 5px;
		border-radius: 4px;
		font-family: monospace;
		font-size: 0.75rem;
		color: var(--text);
	}

	.close-btn {
		background: transparent;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 6px;
		border-radius: 6px;
		transition: all 0.2s ease;
	}

	.close-btn:hover {
		color: var(--text);
		background: rgba(255, 255, 255, 0.1);
	}

	.culling-main {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 16px;
		overflow: hidden;
		position: relative;
	}

	.image-wrapper {
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		max-width: 100%;
		max-height: calc(100vh - 170px);
	}

	.main-image {
		max-width: 100%;
		max-height: calc(100vh - 170px);
		object-fit: contain;
		border-radius: 8px;
		box-shadow: 0 10px 40px rgba(0, 0, 0, 0.6);
	}

	.status-badge {
		position: absolute;
		top: 16px;
		left: 16px;
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 14px;
		border-radius: 8px;
		font-weight: 600;
		font-size: 0.95rem;
		box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
		animation: popIn 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
	}

	.status-badge.keep {
		background: var(--success);
		color: var(--bg-dark);
	}

	.status-badge.reject {
		background: var(--danger);
		color: var(--text);
	}

	.rating-badge {
		position: absolute;
		bottom: 16px;
		right: 16px;
		display: flex;
		align-items: center;
		gap: 4px;
		background: rgba(0, 0, 0, 0.85);
		color: var(--warning);
		padding: 6px 12px;
		border-radius: 8px;
		font-weight: 700;
		font-size: 1rem;
	}

	.star-icon {
		color: var(--warning);
		font-size: 1.2rem;
	}

	/* Filmstrip */
	.culling-filmstrip {
		position: relative;
		height: 80px;
		background: rgba(0, 0, 0, 0.5);
		border-top: 1px solid var(--border);
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 0 16px;
		overflow-x: auto;
		scrollbar-width: thin;
	}

	.floating-focus {
		position: absolute;
		top: 0;
		left: 0;
		pointer-events: none;
		border-radius: 6px;
		box-shadow: inset 0 0 0 3px var(--primary), 0 0 15px color-mix(in srgb, var(--primary) 45%, transparent);
		z-index: 10;
		transition:
			transform 0.2s cubic-bezier(0.2, 0, 0, 1),
			width 0.2s cubic-bezier(0.2, 0, 0, 1),
			height 0.2s cubic-bezier(0.2, 0, 0, 1);
	}

	.filmstrip-thumb {
		position: relative;
		height: 60px;
		min-width: 60px;
		flex-shrink: 0;
		border-radius: 6px;
		overflow: hidden;
		border: 2px solid transparent;
		background: var(--bg-dark);
		cursor: pointer;
		padding: 0;
		opacity: 0.6;
		transition: opacity 0.2s ease, transform 0.2s ease;
	}

	.filmstrip-thumb:hover {
		opacity: 0.9;
		transform: scale(1.04);
	}

	.filmstrip-thumb.active {
		opacity: 1;
	}

	.filmstrip-thumb img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}

	.thumb-flag {
		position: absolute;
		top: 2px;
		right: 2px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.thumb-flag span {
		font-size: 14px;
	}

	.thumb-flag.keep span {
		color: var(--success);
	}

	.thumb-flag.reject span {
		color: var(--danger);
	}

	.thumb-rating {
		position: absolute;
		bottom: 2px;
		left: 2px;
		background: rgba(0, 0, 0, 0.7);
		color: var(--warning);
		font-size: 10px;
		font-weight: bold;
		padding: 1px 3px;
		border-radius: 3px;
		display: flex;
		align-items: center;
		gap: 2px;
	}

	.thumb-rating span {
		font-size: 10px;
	}

	@keyframes popIn {
		0% { transform: scale(0.8); opacity: 0; }
		100% { transform: scale(1); opacity: 1; }
	}
</style>
