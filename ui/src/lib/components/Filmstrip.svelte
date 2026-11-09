<script lang="ts">
	import type { GalleryImage } from '$lib/types';

	let {
		images,
		currentIndex = $bindable(0)
	}: {
		images: GalleryImage[];
		currentIndex: number;
	} = $props();

	let container = $state<HTMLElement | null>(null);
	let focusBox = $state({ x: 0, y: 0, w: 0, h: 0, visible: false});

	function activeThumb() {
		return container?.querySelector(`[data-thumb-index="${currentIndex}"]`) as HTMLElement | null;
	}

	function updateFocusBox() {
		const thumb = activeThumb();
		if (thumb) {
			focusBox = {
				x: thumb.offsetLeft,
				y: thumb.offsetTop,
				w: thumb.offsetWidth,
				h: thumb.offsetHeight,
				visible: true
			};
		} else {
			focusBox.visible = false;
		}
	}

	$effect(() => {
		images.length;
		currentIndex;
		if (!container) return;
		requestAnimationFrame(() => updateFocusBox());
		activeThumb()?.scrollIntoView({ inline: 'center', block: 'nearest', behavior: 'auto'});
	});
</script>

<div class="filmstrip" bind:this={container}>
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
			onclick={() => (currentIndex = i)}
		>
			<img
				src="/api/image?path={encodeURIComponent(img.filepath)}&thumb=true"
				alt=""
				loading="lazy"
			/>

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
</div>

<style>
	.filmstrip {
		position: relative;
		height: 100%;
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
		border-radius: var(--radius-sm);
		box-shadow:
			inset 0 0 0 3px var(--primary),
			0 0 15px color-mix(in srgb, var(--primary) 45%, transparent);
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
		transition:
			opacity 0.2s ease,
			transform 0.2s ease;
	}

	.filmstrip-thumb:hover {
		opacity: 0.9;
		box-shadow:
			inset 0 0 0 3px var(--primary),
			0 0 15px color-mix(in srgb, var(--primary) 45%, transparent);
		transform: scale(1.04);
	}

	.filmstrip-thumb.active {
		opacity: 1;
	}

	.filmstrip-thumb img {
		width: auto;
		height: 100%;
		object-fit: contain;
		display: block;
		margin: 0 auto;
	}

	.thumb-flag {
		position: absolute;
		top: 2px;
		right: 2px;
		padding: 2px 2px 3px 3px;
		background: rgba(0, 0, 0, 0.7);
		border-radius: 3px;
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
		color: var(--info);
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
</style>
