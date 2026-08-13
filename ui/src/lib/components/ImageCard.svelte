<script lang="ts">
	import { decode } from 'blurhash';
	import { formatSize, formatResolution } from '$lib/format';
    import type { GalleryImage } from "$lib/types";

    export interface ImageCardProps {
        image: GalleryImage;
        onclick?: (e: MouseEvent) => void;
        oncontextmenu?: (e: MouseEvent) => void;
        onmouseenter?: (e: MouseEvent) => void;
        isFocused?: boolean;
        isSelected?: boolean;
        dataIndex?: number;
        showMeta?: boolean;
    }

    let {
        image,
        onclick,
        oncontextmenu,
        onmouseenter,
        isFocused = false,
        isSelected = false,
        dataIndex,
        showMeta = false
    }: ImageCardProps = $props();

	let canvasElement = $state<HTMLCanvasElement | null>(null);
	let isLoaded = $state(false);
	let lastPath = $state(image.filepath)

	let idleId: any = null;

	$effect(() => {
		if (image.filepath !== lastPath) {
			lastPath = image.filepath;
			isLoaded = false;
		}
	});

	$effect(() => {
		if (canvasElement && image.blurhash && !isLoaded) {
			const startDecoding = () => {
				if (isLoaded || !canvasElement) return;
				
				try {
					const pixels = decode(image.blurhash, 32, 32);
					const ctx = canvasElement.getContext('2d');
					if (!ctx) return;
					const imageData = ctx.createImageData(32, 32);
					imageData.data.set(pixels);
					ctx.putImageData(imageData, 0, 0);
				} catch (err) {
					console.error("Failed to decode blurhash:", err);
				}
			};

			if (typeof window.requestIdleCallback === 'function') {
				idleId = window.requestIdleCallback(startDecoding, { timeout: 200 });
			} else {
				idleId = setTimeout(startDecoding, 50);
			}
		}

		return () => {
			if (idleId) {
				if (typeof window.cancelIdleCallback === 'function') {
					window.cancelIdleCallback(idleId);
				} else {
					clearTimeout(idleId);
				}
			}
		};
	});
</script>

<div
	class="image-card"
	class:focused={isFocused}
	data-index={dataIndex}
	{onmouseenter}
	onclick={onclick}
	{oncontextmenu}
	style="flex-grow: {image.aspect_ratio || 1.5}; flex-basis: calc(var(--grid-base, 250px) * {image.aspect_ratio || 1.5})"
	tabindex="0">

	<canvas
		bind:this={canvasElement}
		width="32"
		height="32"
		class="blurhash-canvas"
	></canvas>

	<img
		src="/api/image?path={encodeURIComponent(image.filepath)}&thumb=true"
		alt=""
		loading="lazy"
		decoding="async"
		class:loaded={isLoaded}
		onload={() => isLoaded = true}
	/>

	{#if image.rating > 0}
		<div class="badge rating">
			<span class="material-symbols-outlined icon-filled">star</span>
			{image.rating}
		</div>
	{/if}

	{#if image.flag === 1}
		<div class="badge flag keep">
			<span class="material-symbols-outlined">check_circle</span>
		</div>
	{:else if image.flag === -1}
		<div class="badge flag reject">
			<span class="material-symbols-outlined">cancel</span>
		</div>
	{/if}

	{#if isSelected}
		<div class="selection-overlay">
			<span class="material-symbols-outlined check-icon">select_check_box</span>
		</div>
	{/if}

	{#if showMeta && (image.size || image.resolution)}
		<div class="meta-badge">
			{#if image.resolution}<span>{formatResolution(image.resolution, image.aspect_ratio)}</span>{/if}
			{#if image.resolution && image.size}<span class="meta-dot">•</span>{/if}
			{#if image.size}<span>{formatSize(image.size)}</span>{/if}
		</div>
	{/if}

	{#if image.distance !== undefined && image.distance !== null}
		<div class="badge similarity" title="Distance: {image.distance.toFixed(3)}">
			<span class="material-symbols-outlined">auto_awesome</span>
			{Math.round((1 - image.distance) * 100)}%
		</div>
	{/if}
</div>

<style>
	:global(.image-card) {
		scroll-margin-top: 200px;
		scroll-margin-bottom: 200px;
	}

	.image-card {
		position: relative;
		border-radius: 8px;
		clip-path: inset(0 round 8px);
		background-color: var(--bg-light);
		cursor: pointer;
		height: var(--grid-base, 250px);
		--badge-offset: 12px;
	}

	.image-card::before {
		content: '';
		position: absolute;
		inset: 0;
		background: color-mix(in srgb, var(--primary) 15%, transparent);
		z-index: 2;
		pointer-events: none;
		clip-path: circle(0% at 0% 100%);
		transition: clip-path 0.4s cubic-bezier(0.4, 0, 0.2, 1);
		transform: translateZ(0);
		will-change: clip-path;
	}

	.image-card::after {
		content: '';
		position: absolute;
		inset: 0;
		border: 4px solid var(--primary);
		border-radius: 8px;
		box-sizing: border-box;
		z-index: 3;
		pointer-events: none;
		clip-path: circle(0% at 0% 100%);
		transition: clip-path 0.4s cubic-bezier(0.4, 0, 0.2, 1);
		transform: translateZ(0);
		will-change: clip-path;
	}

	.image-card:hover::before,
	.image-card:hover::after,
	.image-card.focused::before,
	.image-card.focused::after {
		clip-path: circle(250% at 0% 100%);
	}

	.image-card img {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
		z-index: 1;
		opacity: 0;
		transition: opacity 0.4s ease, transform 0.2s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.image-card img.loaded {
		opacity: 1;
	}

	.blurhash-canvas {
		position: absolute;
		inset: 0;
		display: block;
		width: 100% !important;
		height: 100% !important;
		object-fit: cover;
		z-index: 0;
	}

	.badge {
		position: absolute;
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 16px;
		border-radius: 20px;
		font-weight: 600;
		font-size: 1.2rem;
		box-shadow: 0 4px 12px rgba(0,0,0,0.5);
		z-index: 10;
	}

	.badge.flag {
		top: var(--badge-offset);
		left: var(--badge-offset);
	}

	.selection-overlay {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.5);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 2;
        pointer-events: none;
    }

    .selection-overlay .check-icon {
        font-size: 2.3rem;
        color: color-mix(in srgb, var(--tertiary) 70%, var(--bg));
        font-variation-settings: 'FILL' 1;
        filter: drop-shadow(0 4px 10px rgba(0, 0, 0, 0.6));
        animation: popIn 0.15s ease-out;
    }

    @keyframes popIn {
        from {
            transform: scale(0.7);
            opacity: 0;
        }
        to {
            transform: scale(1);
            opacity: 1;
        }
    }

	.badge.rating {
		top: var(--badge-offset);
		right: var(--badge-offset);
		background: var(--bg-dark);
		color: var(--info);
	}

	.badge.keep {
		background: var(--bg-dark);
		color: var(--success);
	}

	.badge.reject {
		background: var(--bg-dark);
		color: var(--danger);
	}

	.icon-filled {
		font-variation-settings: 'FILL' 1;
	}

	.meta-badge {
		position: absolute;
		top: 8px;
		left: 8px;
		margin-right: 8px;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 5px;
		padding: 3px 8px;
		background: rgba(0, 0, 0, 0.85);
		border-radius: 6px;
		font-size: 0.72rem;
		color: var(--text);
		font-weight: 500;
		letter-spacing: 0.02em;
		pointer-events: none;
		z-index: 2;
		border: 1px solid rgba(255, 255, 255, 0.1);
	}

	.meta-dot {
		opacity: 0.5;
	}

	.badge.similarity {
		bottom: var(--badge-offset);
		left: 50%;
		transform: translateX(-50%);
		background: color-mix(in srgb, var(--bg-dark) 70%, transparent);
		color: var(--info);
		font-size: 0.9rem;
		padding: 4px 10px;
		gap: 4px;
	}

	.badge.similarity .material-symbols-outlined {
		font-size: 1rem;
	}
</style>
