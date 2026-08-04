<script lang="ts">
	import { carousel } from '$lib/stores/carousel.svelte';
	import { fade, fly, scale } from 'svelte/transition';
	import Spinner from '$lib/components/Spinner.svelte';
	import Info from '$lib/components/Info.svelte';

	let loadedImages = $state<Record<string, boolean>>({});

	function handleKeydown(e: KeyboardEvent) {
		if (!carousel.isOpen) return;

		switch (e.key) {
			case 'l':
			case 'ArrowRight':
				carousel.next();
				break;
			case 'h':
			case 'ArrowLeft':
				carousel.prev();
				break;
			case 'Escape':
				carousel.close();
				break;
			case '0':
			case '1':
			case '2':
			case '3':
			case '4':
			case '5':
				setRating(parseInt(e.key));
				break;
			case 'i':
				carousel.toggleInfo();
				break;
		}
	}

	async function setRating(rating: nubmer) {
		const img = carousel.currentImage;
		if (!img) return;

		const formData = new FormData();
		formData.append('rating', rating.toString());

		try {
			const res = await fetch(`/api/image/${img.id}`, {
				method: 'POST',
				body: formData
			});
			if (res.ok) {
				img.rating = rating;
			}
		} catch (error) {
			console.error("Rating Error:", error);
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if carousel.isOpen}
	<div class="carousel-modal">
		<div class="carousel-toolbar">
			<span class="carousel-counter">
				{carousel.currentIndex + 1} / {carousel.images.length}
			</span>
			<div class="carousel-actions">
				<button class="btn-icon" class:active={carousel.isInfoOpen} onclick={() => carousel.toggleInfo()}>
					<span class="material-symbols-outlined">info</span>
				</button>
				<button class="btn-icon" onclick={() => carousel.close()}>
					<span class="material-symbols-outlined">close</span>
				</button>
			</div>
		</div>

		{#if carousel.hasPrev}
			<button class="btn-icon prev" onclick={() => carousel.prev()}>
				<span class="material-symbols-outlined">chevron_left</span>
			</button>
		{/if}

		<div class="carousel-content">
			{#each carousel.visibleImages as item (item.image.id)}
				<div 
					class="image-wrapper"
					class:center={item.offset === 0}
					class:left={item.offset === -1}
					class:right={item.offset === 1}
				>
					<img
						class="carousel-image"
						class:loaded={loadedImages[item.image.id]}
						decoding="async"
						src={`/api/image?path=${encodeURIComponent(item.image.filepath)}`}
						alt={item.image.name}
						onload={() => loadedImages[item.image.id] = true}
					/>
					
					{#if !loadedImages[item.image.id]}
						<Spinner />
					{/if}
					
					{#if item.image.flag === 1}
						<div class="badge flag keep">
							<span class="material-symbols-outlined">check_circle</span>
						</div>
					{:else if item.image.flag === -1}
						<div class="badge flag reject">
							<span class="material-symbols-outlined">cancel</span>
						</div>
					{/if}

					{#if item.image.rating > 0}
						<div class="badge rating">
							<span class="material-symbols-outlined icon-filled">star</span>
							{item.image.rating}
						</div>
					{/if}
				</div>
			{/each}
		</div>

		{#if carousel.hasNext}
			<button class="btn-icon next" onclick={() => carousel.next()}>
				<span class="material-symbols-outlined">chevron_right</span>
			</button>
		{/if}
	
		{#if carousel.isInfoOpen && carousel.currentImage}
			<Info image={carousel.currentImage} />
		{/if}
	</div>
{/if}

<style>
	.carousel-modal {
		position: fixed;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		background: rgba(0, 0, 0, 0.6);
		backdrop-filter: blur(24px);
		z-index: 1000;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.carousel-toolbar {
		position: absolute;
		top: 20px;
		left: 0;
		right: 0;
		padding: 20px 32px;
		box-sizing: border-box;
		display: flex;
		justify-content: center;
		align-items: center;
		z-index: 1010;
		pointer-events: none;
	}

	.carousel-content {
		position: relative;
		width: 100%;
		height: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden;
		perspective: 1200px;
	}

	.carousel-actions {
		position: absolute;
		right: 24px;
		display: flex;
		gap: 12px;
		pointer-events: auto;
	}

	.carousel-image {
		max-width: 90vw;
		max-height: 90vh;
		object-fit: contain;
	}

	.image-wrapper {
		position: absolute;
		top: 0; left: 0;
		width: 100%; height: 100%;
		display: flex;
		justify-content: center;
		align-items: center;
		will-change: transform, opacity;
		transition: transform 0.4s cubic-bezier(0.2, 0.8, 0.2, 1), opacity 0.4s;
	}

	.image-wrapper.center {
		transform: translateX(0) scale(1) rotateY(0deg);
		opacity: 1;
		z-index: 2;
	}

	.image-wrapper.left {
		transform: translateX(-75vw) scale(0.3) rotateY(25deg);
		opacity: 0.0;
		z-index: 1;
	}

	.image-wrapper.right {
		transform: translateX(75vw) scale(0.3) rotateY(-25deg);
		opacity: 0.0;
		z-index: 1;
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
		bottom: 32px;
		left: 32px;
	}

	.badge.rating {
		bottom: 32px;
		right: 32px;
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

	.btn-icon.prev {
		position: absolute;
		left: 24px;
	}

	.btn-icon.next {
		position: absolute;
		right: 24px;
	}

	.btn-icon.active::before {
		height: 2px;
		border-radius: 8px;
		background: var(--tertiary);
	}

	.btn-icon.active:hover {
		color: var(--bg-dark);
	}

	.btn-icon.active:hover::before {
		height: 100%;
		border-radius: 8px;
	}
</style>
