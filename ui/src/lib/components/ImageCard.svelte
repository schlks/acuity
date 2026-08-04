<script>
	import { decode } from 'blurhash';

	let { image, onclick } = $props();

	let canvasElement = $state(null);
	let isLoaded = $state(false);

	$effect(() => {
		if (canvasElement && image.blurhash && !isLoaded) {
			const pixels = decode(image.blurhash, 32, 32)
			const ctx = canvasElement.getContext('2d');
			const imageData = ctx.createImageData(32, 32);
			imageData.data.set(pixels);
			ctx.putImageData(imageData, 0, 0);
		}
	});
</script>

<div
	class="image-card"
	onclick={onclick}
	style="flex-grow: {image.aspect_ratio}; flex-basis: calc(var(--grid-base, 250px) * {image.aspect_ratio})"
	tabindex="0">

	<canvas
		bind:this={canvasElement}
		width="32"
		height="32"
		class="blurhash-canvas"
	></canvas>

	<img
		src="/api/image?path={encodeURIComponent(image.filepath)}"
		alt=""
		loading="lazy"
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
		<div class="badge keep">
			<span class="material-symbols-outlined">check_circle</span>
		</div>
	{:else if image.flag === -1}
		<div class="badge reject">
			<span class="material-symbols-outlined">cancel</span>
		</div>
	{/if}
</div>

<style>
	.image-card::before {
		content: '';
		position: absolute;
		top: 0; left: 0; right: 0; bottom: 0;
		background: transparent;
		z-index: 2; /* Höher als das Bild (z-index: 1) */
		pointer-events: none;
		clip-path: circle(0% at 0% 100%);
		transition: background 0.4s cubic-bezier(0.4, 0, 0.2, 1), clip-path 0.4s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.image-card {
		position: relative;
		border-radius: 8px;
		background-color: var(--bg-light);
		overflow: hidden;
		cursor: pointer;
		height: var(--grid-base, 250px);
	}

	.image-card::after {
		content: '';
		position: absolute;
		top: 0; left: 0; right: 0; bottom: 0;

		border: 3px solid var(--primary);
		border-radius: 8px;
		box-sizing: border-box;
		z-index: 3; /* Höher als das Bild und der Tint */
		pointer-events: none;

		clip-path: circle(0% at 0% 100%);
		transition: clip-path 0.4s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.image-card:hover::before {
		clip-path: circle(150% at 0% 100%);
		background: color-mix(in srgb, var(--primary) 15%, transparent);
	}

	.image-card:hover::after {
		clip-path: circle(150% at 0% 100%);
	}

	.image-card img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
		transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.2s;
	}

	.blurhash-canvas {
		position: absolute;
		top: 0;
		left: 0;
		display: block;
		width: 100% !important;
		height: 100% !important;
		object-fit: cover;
		z-index: 0;
	}

	.image-card {
		position: relative;
		border-radius: 8px;
		background-color: var(--bg-light);
		overflow: hidden;
		cursor: pointer;
		height: var(--grid-base, 250px);
	}

	.image-card img {
		position: absolute;
		top: 0;
		left: 0;
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
		top: 12px;
		left: 12px;
	}

	.badge.rating {
		top: 12px;
		right: 12px;
		background: var(--bg-dark);
		color: var(--info);
	}

	.badge.keep {
		background: var(--success);
		color: var(--bg-dark)
	}

	.badge.reject {
		background: var(--danger);
		color: var(--text);
	}

	.icon-filled {
		font-variation-settings: 'FILL' 1;
	}
</style>
