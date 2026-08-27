<script lang="ts">
	import { carousel } from '$lib/stores/carousel.svelte';
	import Spinner from '$lib/components/Spinner.svelte';
	import Info from '$lib/components/Info.svelte';
        import { toast } from "$lib/stores/toast.svelte";

	let loadedImages = $state<Record<string, boolean>>({});
	let filmstripContainer = $state<HTMLElement | null>(null);
	let focusBox = $state({ x: 0, y: 0, w: 0, h: 0, visible: false });
	let zoom = $state(1);
	let pan = $state({ x: 0, y: 0 });
	let isDragging = $state(false);
	let dragStart = $state({ x: 0, y: 0 });
	
	$effect(() => {
		if (carousel.isOpen && filmstripContainer) {
			requestAnimationFrame(() => updateFocusBox());
			const activeThumb = filmstripContainer.querySelector(`[data-thumb-index="${carousel.currentIndex}"]`) as HTMLElement;
			activeThumb?.scrollIntoView({ inline: 'center', block: 'nearest', behavior: 'auto' });
		}
	});

	$effect(() => {
        carousel.currentIndex;
        resetZoom();
	})

	function resetZoom() {
		zoom = 1;
		pan = { x: 0, y: 0 };
		isDragging = false;
	}

	function updateFocusBox() {
		if (!filmstripContainer) return;
		const activeThumb = filmstripContainer.querySelector(`[data-thumb-index="${carousel.currentIndex}"]`) as HTMLElement;
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

	function handleWheel(e: WheelEvent) {
		if (!carousel.isOpen) return;
		e.preventDefault();

		const zoomFactor = -e.deltaY * 0.002;
		const newZoom = Math.min(Math.max(1, zoom + zoomFactor), 5);

		if (newZoom === 1) {
			pan = { x: 0, y: 0};
		}
		zoom = newZoom;
	}

	function handleDoubleClick() {
		if (zoom > 1) {
			resetZoom();
		} else {
			zoom = 2.5;
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
		dragStart = { x: e.clientX - pan.x, y: e.clientY - pan.y};
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
			console.error("Rating Error:", error);
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
			console.error("Flag Error:", error);
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
			console.error("Löschen fehlgeschlagen", e);
		}
	}

	function handleGlobalClick(e: MouseEvent) {
		if (!carousel.isOpen) return;
		const target = e.target as HTMLElement;
		if (!target || typeof target.closest !== 'function') return ;

		if (carousel.isInfoOpen && !target.closest('.info-popup, .btn-icon')) {
			carousel.isInfoOpen = false;
		}
	}

        async function copyImage() {
                try {
                        const img = carousel.currentImage
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

                                const pngBlob = await new Promise<Blob>((resolve) => canvas.toBlob((b) => resolve(b!), 'image/png'));
                                URL.revokeObjectURL(img.src);
                                return pngBlob;
                        })(img);

                        await navigator.clipboard.write([
                                new ClipboardItem({ 'image/png': imageBlobPromise })
                        ]);
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
					role="presentation"
					class:center={item.offset === 0}
					class:left={item.offset === -1}
					class:right={item.offset === 1}
					onwheel={item.offset === 0 ? handleWheel : undefined}
					ondblclick={item.offset === 0 ? handleDoubleClick : undefined}
					onmousedown={item.offset === 0 ? handleMouseDown : undefined}
				>
					<img
						class="carousel-image"
						class:loaded={loadedImages[item.image.id]}
						style={item.offset === 0 ? `transform: translate3d(${pan.x}px, ${pan.y}px, 0) scale(${zoom}); cursor: ${zoom > 1 ? (isDragging ? 'grabbing' : 'grab') : 'default'};` : ''}
						draggable="false"
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
	
		<!-- Bottom Filmstrip with Floating Focus Frame -->
		<footer class="carousel-filmstrip" bind:this={filmstripContainer}>
			{#if focusBox.visible}
				<div
					class="floating-focus"
					style="transform: translate3d({focusBox.x}px, {focusBox.y}px, 0); width: {focusBox.w}px; height: {focusBox.h}px;"
				></div>
			{/if}

			{#each carousel.images as img, i (img.filepath || i)}
				<button
					class="filmstrip-thumb"
					class:active={carousel.currentIndex === i}
					data-thumb-index={i}
					onclick={() => carousel.currentIndex = i}
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
		top: 0; left: 0;
		width: 100%; height: 100%;
		display: flex;
		justify-content: center;
		align-items: center;
		backface-visibility: hidden;
		transform: translateZ(0);
		will-change: transform, opacity;
		transition: transform 0.28s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.28s ease;
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
		border-radius: 0;
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

	.carousel-filmstrip {
		position: absolute;
		bottom: 0;
		left: 0;
		right: 0;
		height: 80px;
		background: rgba(15, 15, 15, 0.95);
		border-top: 1px solid var(--border);
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 0 16px;
		overflow-x: auto;
		scrollbar-width: thin;
		z-index: 1010;
	}

	.floating-focus {
		position: absolute;
		top: 0;
		left: 0;
		pointer-events: none;
		border-radius: 6px;
		box-shadow: inset 0 0 0 3px var(--info), 0 0 15px color-mix(in srgb, var(--info) 45%, transparent);
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
		box-shadow: inset 0 0 0 3px var(--primary), 0 0 15px color-mix(in srgb, var(--primary) 45%, transparent);
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
</style>
