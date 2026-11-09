<script lang="ts">
	import { carousel } from '$lib/stores/carousel.svelte';
	import Spinner from '$lib/components/Spinner.svelte';
	import Info from '$lib/components/Info.svelte';
	import { toast } from '$lib/stores/toast.svelte';
	import Filmstrip from './Filmstrip.svelte';

	let loadedImages = $state<Record<string, boolean>>({});
	let zoom = $state(1);
	let pan = $state({ x: 0, y: 0 });
	let isDragging = $state(false);
	let dragStart = $state({ x: 0, y: 0 });
	let fullResFor = $state<string | null>(null);
	let fullResUrl = $state('');

	function releaseFullRes() {
		if (fullResUrl) URL.revokeObjectURL(fullResUrl);
		fullResUrl = '';
		fullResFor = null;
	}

	async function loadFullRes() {
		const image = carousel.currentImage;
		if (!image || fullResFor === image.filepath) return;
		const path = image.filepath;
		const res = await fetch(`/api/image?path=${encodeURIComponent(path)}`);
		const blob = await res.blob();
		if (carousel.currentImage?.filepath !== path) return;
		if (fullResUrl) URL.revokeObjectURL(fullResUrl);
		fullResUrl = URL.createObjectURL(blob);
		fullResFor = path;
	}

	$effect(() => {
		carousel.currentIndex;
		resetZoom();
		releaseFullRes();
	});

	$effect(() => {
		if (!carousel.isOpen) releaseFullRes();
	});

	function resetZoom() {
		zoom = 1;
		pan = { x: 0, y: 0 };
		isDragging = false;
	}

	function handleWheel(e: WheelEvent) {
		if (!carousel.isOpen) return;
		e.preventDefault();

		const zoomFactor = -e.deltaY * 0.002;
		const newZoom = Math.min(Math.max(1, zoom + zoomFactor), 5);

		if (newZoom === 1) {
			pan = { x: 0, y: 0 };
		}
		zoom = newZoom;
		if (newZoom > 1) loadFullRes();
	}

	function handleDoubleClick() {
		if (zoom > 1) {
			resetZoom();
		} else {
			zoom = 2.5;
			loadFullRes();
		}
	}

	function handleMouseMove(e: MouseEvent) {
		if (!isDragging || zoom <= 1) return;
		pan = {
			x: e.clientX - dragStart.x,
			y: e.clientY - dragStart.y
		};
	}

	function handleMouseUp() {
		isDragging = false;
	}

	function handleMouseDown(e: MouseEvent) {
		if (zoom <= 1 || e.button !== 0) return;
		isDragging = true;
		dragStart = { x: e.clientX - pan.x, y: e.clientY - pan.y };
		e.preventDefault();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (!carousel.isOpen) return;

		switch (e.key) {
			case 'l':
			case 'ArrowRight':
			case 'd':
				carousel.next();
				break;
			case 'h':
			case 'ArrowLeft':
			case 'a':
				carousel.prev();
				break;
			case 'Escape':
				zoom > 1 ? resetZoom() : carousel.close();
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
			case 'n':
				setFlag(-1);
				break;
			case 'm':
				setFlag(1);
				break;
			case 'u':
				setFlag(0);
				break;
			case 'Delete':
				deleteImage();
				break;
			case 'c':
				e.preventDefault();
				if (e.ctrlKey) {
					copyImage();
				}
				break;
		}
	}

	async function setRating(rating: number) {
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
			console.error('Rating Error:', error);
		}
	}

	async function setFlag(flag: number) {
		const img = carousel.currentImage;
		if (!img || !img.id) return;

		const formData = new FormData();
		if (img.flag === flag) flag = 0;
		formData.append('flag', flag.toString());

		try {
			const res = await fetch(`/api/image/${img.id}/flag`, {
				method: 'POST',
				body: formData
			});
			if (res.ok) {
				img.flag = flag;
			}
		} catch (error) {
			console.error('Flag Error:', error);
		}
	}

	async function deleteImage() {
		if (!confirm('remove image irreversibly?')) return;
		try {
			const img = carousel.currentImage;
			if (!img) return;
			await fetch(`/api/image/${img.id}/delete`, { method: 'POST' });
			carousel.images.splice(carousel.currentIndex, 1);

			if (carousel.images.length === 0) {
				carousel.close();
			} else if (carousel.currentIndex >= carousel.images.length) {
				carousel.currentIndex = carousel.images.length - 1;
			}
		} catch (e) {
			console.error('Löschen fehlgeschlagen', e);
		}
	}

	function handleGlobalClick(e: MouseEvent) {
		if (!carousel.isOpen) return;
		const target = e.target as HTMLElement;
		if (!target || typeof target.closest !== 'function') return;

		if (carousel.isInfoOpen && !target.closest('.info-popup, .btn-icon')) {
			carousel.isInfoOpen = false;
		}
	}

	async function copyImage() {
		try {
			const img = carousel.currentImage;
			if (!img) return;
			const imageBlobPromise = (async (image) => {
				const res = await fetch(`/api/image?path=${encodeURIComponent(image.filepath)}`);
				const blob = await res.blob();

				if (blob.type === 'image/png') return blob;

				const img = new Image();
				img.src = URL.createObjectURL(blob);
				await new Promise((resolve) => (img.onload = resolve));

				const canvas = document.createElement('canvas');
				canvas.width = img.naturalWidth;
				canvas.height = img.naturalHeight;
				const ctx = canvas.getContext('2d');
				ctx?.drawImage(img, 0, 0);

				const pngBlob = await new Promise<Blob>((resolve) =>
					canvas.toBlob((b) => resolve(b!), 'image/png')
				);
				URL.revokeObjectURL(img.src);
				return pngBlob;
			})(img);

			await navigator.clipboard.write([new ClipboardItem({ 'image/png': imageBlobPromise })]);
			toast.success('Image copied to clipboard');
		} catch (err) {
			console.error('Failed to copy image', err);
			toast.error('Failed to copy image');
		}
	}
</script>

<svelte:window
	onclick={handleGlobalClick}
	onkeydown={handleKeydown}
	onmousemove={handleMouseMove}
	onmouseup={handleMouseUp}
/>

{#if carousel.isOpen}
	<div class="carousel-modal">
		<div class="carousel-toolbar">
			<span class="carousel-counter">
				{carousel.currentIndex + 1} / {carousel.images.length}
			</span>
			<div class="carousel-actions">
				<button
					class="btn-icon"
					class:active={carousel.isInfoOpen}
					onclick={() => carousel.toggleInfo()}
				>
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
					role="presentation"
					class:center={item.offset === 0}
					class:left={item.offset === -1}
					class:right={item.offset === 1}
					onwheel={item.offset === 0 ? handleWheel : undefined}
					ondblclick={item.offset === 0 ? handleDoubleClick : undefined}
					onmousedown={item.offset === 0 ? handleMouseDown : undefined}
				>
					<div class="image-frame">
						<img
							class="carousel-image"
							class:loaded={loadedImages[item.image.id]}
							style={item.offset === 0
								? `transform: translate3d(${pan.x}px, ${pan.y}px, 0) scale(${zoom}); cursor: ${zoom > 1 ? (isDragging ? 'grabbing' : 'grab') : 'default'};`
								: ''}
							draggable="false"
							decoding="async"
							src={fullResFor === item.image.filepath
								? fullResUrl
								: carousel.displaySrcs[item.image.filepath] || ''}
							alt={item.image.name}
							onload={() => (loadedImages[item.image.id] = true)}
						/>

						{#if loadedImages[item.image.id]}
							{#if item.image.flag === 1}
								<div class="badge flag keep">
									<span class="material-symbols-outlined">check_circle</span>
									<span>Keep</span>
								</div>
							{:else if item.image.flag === -1}
								<div class="badge flag reject">
									<span class="material-symbols-outlined">cancel</span>
									<span>Reject</span>
								</div>
							{/if}

							{#if item.image.rating > 0}
								<div class="badge rating">
									<span class="material-symbols-outlined icon-filled star-icon">star</span>
									<span>{item.image.rating}</span>
								</div>
							{/if}
						{/if}
					</div>

					{#if !loadedImages[item.image.id]}
						<Spinner />
					{/if}
				</div>
			{/each}
		</div>

		{#if carousel.hasNext}
			<button class="btn-icon next" onclick={() => carousel.next()}>
				<span class="material-symbols-outlined">chevron_right</span>
			</button>
		{/if}

		<footer class="carousel-filmstrip">
			<Filmstrip images={carousel.images} bind:currentIndex={carousel.currentIndex} />
		</footer>

		{#if carousel.isInfoOpen && carousel.currentImage}
			<Info image={carousel.currentImage} onDelete={deleteImage} onClose={() => carousel.close()} />
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
		background: rgba(9, 9, 9, 0.96);
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
		height: calc(100% - 80px);
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
		max-height: calc(90vh - 80px);
		object-fit: contain;
		-webkit-user-drag: none;
	}

	.image-wrapper {
		position: absolute;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		display: flex;
		justify-content: center;
		align-items: center;
		backface-visibility: hidden;
		transform: translateZ(0);
		will-change: transform, opacity;
		transition:
			transform 0.28s cubic-bezier(0.16, 1, 0.3, 1),
			opacity 0.28s ease;
	}

	.image-wrapper.center {
		transform: translate3d(0, 0, 0);
		opacity: 1;
		z-index: 2;
	}

	.image-wrapper.left {
		transform: translate3d(-35px, 0, 0);
		opacity: 0;
		z-index: 1;
	}

	.image-wrapper.right {
		transform: translate3d(35px, 0, 0);
		opacity: 0;
		z-index: 1;
	}

	.image-frame {
		position: relative;
		display: flex;
	}

	.badge {
		position: absolute;
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

	.badge.flag {
		top: 16px;
		left: 16px;
	}

	.badge.rating {
		bottom: 16px;
		right: 16px;
		gap: 4px;
		padding: 6px 12px;
		background: rgba(0, 0, 0, 0.85);
		color: var(--info);
		font-weight: 700;
		font-size: 1rem;
		box-shadow: none;
		animation: none;
	}

	.star-icon {
		color: var(--info);
		font-size: 1.2rem;
	}

	.badge.keep {
		background: var(--success);
		color: var(--bg-dark);
	}

	.badge.reject {
		background: var(--danger);
		color: var(--text);
	}

	@keyframes popIn {
		0% {
			transform: scale(0.8);
			opacity: 0;
		}
		100% {
			transform: scale(1);
			opacity: 1;
		}
	}

	.btn-icon {
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		background: transparent;
		border: none;
		color: var(--text-muted);
		padding: 8px;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition:
			background-color var(--duration-base) ease,
			color var(--duration-base) ease,
			box-shadow var(--duration-base) ease,
			border-color var(--duration-base) ease;
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
		transition: all var(--duration-slow) var(--ease-standard);
	}

	.btn-icon:hover::before {
		height: 100%;
		border-radius: var(--radius-md);
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
		border-radius: var(--radius-md);
		background: var(--tertiary);
	}

	.btn-icon.active:hover {
		color: var(--bg-dark);
	}

	.btn-icon.active:hover::before {
		height: 100%;
		border-radius: var(--radius-md);
	}

	.carousel-filmstrip {
		position: absolute;
		bottom: 0;
		left: 0;
		right: 0;
		height: 80px;
		background: rgba(15, 15, 15, 0.95);
		border-top: 1px solid var(--border);
		z-index: 1010;
	}
</style>
