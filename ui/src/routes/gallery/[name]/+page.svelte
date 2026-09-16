<script lang="ts">
	import { invalidateAll, goto } from '$app/navigation';
	import { page, navigating } from '$app/state';
	import { carousel } from '$lib/stores/carousel.svelte';
	import { tick } from 'svelte';
	import { fade } from 'svelte/transition';
	import ImageCard from '$lib/components/ImageCard.svelte';
	import Spinner from '$lib/components/Spinner.svelte';
	import BatchActionDialog from '$lib/components/BatchActionDialog.svelte';
	import CullingMode from '$lib/components/CullingMode.svelte';
	import ContextMenu from '$lib/components/ContextMenu.svelte';
	import { toast } from '$lib/stores/toast.svelte';
	import { modal } from '$lib/stores/modal.svelte';
    import type { GalleryImage } from '$lib/types/GalleryImage';
    import type { ActionReturn } from "svelte/action";

	let { data } = $props();
	let searchQuery = $state(page.url.searchParams.get('q') || '');
	let searchThreshold = $state(page.url.searchParams.get('threshold') || '0.9');
	let infiniteScroll = $state(data.settings?.infinite_scroll || false);
	let currentPage = $derived(Number(data.images.current_page || 1));
	let currentFlag = $derived(page.url.searchParams.get('flagFilter') || '');
	let currentSort = $derived(page.url.searchParams.get('sortBy') || 'name');
	let currentOrder = $derived(page.url.searchParams.get('sortOrder') || 'desc');
	let hasActiveFilters = $derived(
		Boolean(
			page.url.searchParams.get('q') ||
			page.url.searchParams.get('flagFilter') ||
			page.url.searchParams.get('sortBy') ||
			page.url.searchParams.get('folderFilter') ||
			page.url.searchParams.get('sortOrder')
		)
	);

	function resetFilters() {
		searchQuery = '';
		searchThreshold = '0.9';
		if (droppedImagePreview) {
			URL.revokeObjectURL(droppedImagePreview);
			droppedImagePreview = null;
		}
		goto(page.url.pathname);
	}
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
	let galleryImages = $state<GalleryImage[]>([]);
	let selectedImages = $state<GalleryImage[]>([]);
	let selectedIds = $derived(new Set(selectedImages.map(i => i.id)));
	let batchIsOpen = $state(false);
	let hasMore = $state(false);
	let currentInfinitePage = $state(1);
	let isLoadingMore = $state(false);
	let searchTimeout: number | undefined;
	let focusedIndex = $state<number>(0);
	let focusBox = $state({ x: 0, y: 0, w: 0, h: 0, visible: false });
	let isKeyboardMode = $state(false);
	let isCullingOpen = $state(false);
	let isCullingPaging = $state(false);
	let pendingCullingJump: 'first' | 'last' | null = null;
	let jumpInputIndex = $state<number | null>(null);
	let jumpPageVal = $state<number | string>('');
	let isEditingName = $state(false);
	let editedName = $state('');
	let editInputEl = $state<HTMLInputElement | null>(null);
	let contextMenu = $state<{ x: number; y: number; image: GalleryImage } | null>(null);
	let isDragging = $state(false);
	let droppedImagePreview = $state<string | null>(null);
	let dragDepth = $state(0);
	let isWindowDragging = $derived(dragDepth > 0);

        let lastUrlQ = page.url.searchParams.get('q') || '';
	$effect(() => {
		galleryImages = data.images.images || [];
		hasMore = data.images.has_page;
		currentInfinitePage = data.images.current_page;

		if (pendingCullingJump && galleryImages.length > 0) {
			focusedIndex = pendingCullingJump === 'first' ? 0 : galleryImages.length - 1;
			pendingCullingJump = null;
			isCullingPaging = false;
		}

		if (data.settings) {
			infiniteScroll = data.settings.infinite_scroll;
		}
                const currentUrlQ = page.url.searchParams.get('q') || '';
                if (currentUrlQ !== lastUrlQ) {
                        lastUrlQ = currentUrlQ;
                        searchQuery = currentUrlQ;
                }
	});

	$effect(() => {
		if (carousel.isOpen && galleryImages.length > 0 && infiniteScroll) {
			const imagesLeft = galleryImages.length - carousel.currentIndex - 1;
			if (imagesLeft <= 20 && !isLoadingMore && hasMore) {
				loadMore();
			}
		}
	});

	$effect(() => {
		if (isKeyboardMode && focusedIndex !== null && galleryImages.length > 0) {
			updateFocusBox();
		} else {
                        focusBox.visible = false;
                }
	})

	function startEditingName() {
		editedName = data.gallery?.heading || data.gallery?.name || '';
		isEditingName = true;
		requestAnimationFrame(() => {
			editInputEl?.focus();
			editInputEl?.select();
		});
	}

	function cancelEditingName() {
		isEditingName = false;
		editedName = '';
	}

	function updateFocusBox() {
		if (focusedIndex === null || galleryImages.length === 0) {
			focusBox.visible = false;
			return;
		}

		const el = document.querySelector(`[data-index="${focusedIndex}"]`) as HTMLElement;
		if (el) {
			focusBox = {
				x: el.offsetLeft,
				y: el.offsetTop,
				w: el.offsetWidth,
				h: el.offsetHeight,
				visible: true
			};
		}
	}

	function viewPort(node: Element, callback: { (): Promise<void>; (): void; }): ActionReturn {
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) {
					callback();
				}
			},
			{
				root: node.closest('.main-content'),
				rootMargin: '300px'
			}
		);
		observer.observe(node);
		return {
			destroy() {
				observer.disconnect();
			}
		};
	}

	function applyFilter(key: string, value: string | null) {
		let currentUrl = new URL(location.href);
		if (currentUrl.searchParams.get(key) === value) {
			currentUrl.searchParams.delete(key);
		} else if (value) {
			currentUrl.searchParams.set(key, value);
		} else {
			currentUrl.searchParams.delete(key);
		}
		currentUrl.searchParams.set('page', '1');

		goto(currentUrl.pathname + currentUrl.search);
		document.querySelector('.main-content')?.scrollTo({ top: 0, behavior: 'auto' });
	}

	async function saveGalleryName() {
		const newName = editedName.trim();
		if (!newName || newName === data.gallery.name) {
			cancelEditingName();
			return;
		}

		const formData = new FormData();
		formData.append('name', newName);

		try {
			const res = await fetch(`/api/gallery/${encodeURIComponent(data.gallery.name)}/edit`, {
				method: 'POST',
				body: formData
			});
			if (res.ok) {
				isEditingName = false;
				await invalidateAll();
				await goto(`/gallery/${encodeURIComponent(newName)}`);
			} else {
				alert('Failed to rename gallery');
			}
		} catch(e) {
			console.error('Rename error:', e);
		}
	}

	async function toggleInfiniteScroll() {
		const newMode = !infiniteScroll;
		if (data.settings) {
			const updated = { ...data.settings, infinite_scroll: newMode };
			try {
				await fetch('/api/settings', {
					method: 'POST',
					body: JSON.stringify(updated)
				});
			} catch (e) {
				console.error('Failed to save infinite scroll setting:', e);
			}
		}
		infiniteScroll = newMode;
		await invalidateAll();
	}

	async function changePage(newPage: number, toBottom: boolean = false) {
		if (newPage < 1) return;

		let currentUrl = new URL(location.href);
		currentUrl.searchParams.set('page', newPage.toString());

		await goto(currentUrl.pathname + currentUrl.search);
		await tick();

		const container = document.querySelector('.main-content')
		if (container) {
			container.scrollTo({
				top: toBottom ? container.scrollHeight : 0,
				behavior: 'auto'
			});
		}
	}

	let cullingHasNextPage = $derived(
		infiniteScroll ? hasMore : currentPage < (data.images.last_page || 1)
	);
	let cullingHasPrevPage = $derived(!infiniteScroll && currentPage > 1);

	async function cullingNextPage() {
		if (isCullingPaging) return;

		if (infiniteScroll) {
			if (!hasMore || isLoadingMore) return;
			isCullingPaging = true;
			const before = galleryImages.length;
			await loadMore();
			if (galleryImages.length > before) focusedIndex = before;
			isCullingPaging = false;
			return;
		}

		if (currentPage >= (data.images.last_page || 1)) return;
		isCullingPaging = true;
		pendingCullingJump = 'first';
		await changePage(currentPage + 1);
	}

	async function cullingPrevPage() {
		if (isCullingPaging || infiniteScroll || currentPage <= 1) return;
		isCullingPaging = true;
		pendingCullingJump = 'last';
		await changePage(currentPage - 1, true);
	}

	function handleSearch(event?: SubmitEvent | Event) {
		if (event) event.preventDefault();

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

		currentUrl.searchParams.set('page', '1');

		goto(currentUrl.pathname + currentUrl.search);
		document.querySelector('.main-content')?.scrollTo({ top: 0, behavior: 'auto' });
        (document.activeElement as HTMLElement)?.blur();
	}

	function handleDebounceSearch() {
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			handleSearch();
		}, 500);
	}

	async function searchByImage(input: File | string) {
		const formData = new FormData();
		if (typeof input === 'string') {
			formData.append('path', input);
			droppedImagePreview = input.startsWith('http') ? input : `/api/image/raw?path=${encodeURIComponent(input)}`;
		} else {
			formData.append('file', input);
			droppedImagePreview = URL.createObjectURL(input);
		}
		searchQuery = '';
		toast.info('Searching visually similar images...');
		formData.append('threshold', searchThreshold);

		try {
			const res = await fetch(`/api/gallery/${data.gallery.name}/search`, {
				method: 'POST',
				body: formData
			});
			if (res.ok) {
				const result = await res.json();
				galleryImages = result.images || [];
				toast.success(`Found ${galleryImages.length} matching images`);
			} else {
				toast.error('Visual search failed');
			}
		} catch (e) {
			console.error('Visual search error:', e);
			toast.error('Visual search failed');
		}
	}

	function isImageFile(file: File): boolean {
		if (file.type && file.type.startsWith('image/')) return true;
		const ext = file.name.split('.').pop()?.toLowerCase();
		return ['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp', 'avif', 'tiff', 'svg', 'heic', 'jxl', 'cr2', 'cr3', 'nef', 'arw', 'dng', 'rw2', 'orf', 'pef'].includes(ext || '');
	}

	function hasFilePayload(e: DragEvent): boolean {
		if (!e.dataTransfer) return false;
		return e.dataTransfer.types.includes('Files') ||
		       e.dataTransfer.types.includes('text/uri-list') ||
		       e.dataTransfer.types.includes('text/plain');
	}

	function parseImageUri(uri: string): string | null {
		if (!uri) return null;
		const lines = uri.split(/\r?\n/).map(l => l.trim()).filter(l => l && !l.startsWith('#'));
		for (const line of lines) {
			let clean = line.replace(/^file:\/\/(localhost)?/, '');
			clean = decodeURIComponent(clean);
			const ext = clean.split('.').pop()?.toLowerCase();
			if (['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp', 'avif', 'tiff', 'svg', 'heic', 'jxl', 'cr2', 'cr3', 'nef', 'arw', 'dng', 'rw2', 'orf', 'pef'].includes(ext || '')) {
				return clean;
			}
		}
		return null;
	}

	async function handleDropImage(e: DragEvent) {
		e.preventDefault();
		isDragging = false;
		dragDepth = 0;
		const file = e.dataTransfer?.files?.[0];
		if (file && isImageFile(file)) {
			await searchByImage(file);
			return;
		}
		for (const type of ['text/uri-list', 'text/plain', 'text/x-moz-url', 'URL']) {
			const data = e.dataTransfer?.getData(type);
			if (data) {
				const path = parseImageUri(data);
				if (path) {
					await searchByImage(path);
					return;
				}
			}
		}
		toast.error('Only images can be dropped for visual search');
	}

	function handleWindowDragEnter(e: DragEvent) {
		if (hasFilePayload(e)) {
			e.preventDefault();
			dragDepth++;
		}
	}

	function handleWindowDragLeave(e: DragEvent) {
		if (hasFilePayload(e)) {
			e.preventDefault();
			dragDepth = Math.max(0, dragDepth - 1);
		}
	}

	function handleWindowDragOver(e: DragEvent) {
		e.preventDefault();
		if (e.dataTransfer) {
			e.dataTransfer.dropEffect = 'copy';
		}
	}

	async function handleWindowDrop(e: DragEvent) {
		e.preventDefault();
		dragDepth = 0;
		const file = e.dataTransfer?.files?.[0];
		if (file && isImageFile(file)) {
			await searchByImage(file);
			return;
		}
		for (const type of ['text/uri-list', 'text/plain', 'text/x-moz-url', 'URL']) {
			const data = e.dataTransfer?.getData(type);
			if (data) {
				const path = parseImageUri(data);
				if (path) {
					await searchByImage(path);
					return;
				}
			}
		}
		toast.error('Only images can be used for visual search');
	}

	function clearImageSearch() {
		if (droppedImagePreview) {
			URL.revokeObjectURL(droppedImagePreview);
			droppedImagePreview = null;
		}
		resetFilters();
	}

	async function loadMore() {
		console.log('loading more');
		if (isLoadingMore || !hasMore) return;
		isLoadingMore = true;

		let url = new URL(location.href);
		if (url.searchParams.get('q')) {
			isLoadingMore = false;
			return;
		}
		url.searchParams.set('page', (Number(currentInfinitePage) + 1).toString());

		try {
		const res = await fetch(`/api/gallery/${data.gallery.name}/images?${url.searchParams.toString()}`);
			if (res.ok) {
				const json = await res.json();
				galleryImages.push(...json.images);
				hasMore = json.has_page;
				currentInfinitePage = json.current_page;
			}
		} finally {
			isLoadingMore = false;
		}
	}

	async function setRating(rating: number) {
		const img = galleryImages[focusedIndex];
		if (!img || !img.id) return;

		const formData = new FormData();
		formData.append('rating', rating.toString());

		try {
			const res = await fetch(`/api/image/${img.id}`, {
				method: 'POST',
				body: formData
			});
			if (res.ok) {
				galleryImages[focusedIndex] = { ...img, rating };
			}
		} catch (error) {
			console.error("Rating Error:", error);
		}
	}

	async function setFlag(flag: number) {
		const img = galleryImages[focusedIndex];
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
				galleryImages[focusedIndex] = { ...img, flag };
			}
		} catch (error) {
			console.error("Flag Error:", error);
		}
	}

	async function rescanGallery() {
		const ok = await modal.confirm({
			title: 'Rescan Gallery',
			message: 'Scan this gallery for new Images?',
			confirmText: 'Scan'
		});
		if (!ok) return;
		toast.info('Scanning gallery...');
		await fetch(`/api/gallery/${data.gallery.name}/scan`, { method: 'POST' });
		await invalidateAll();
		toast.success('Gallery scan complete');
	}

	async function handleContextMenuFlag(img: GalleryImage, flag: number) {
		try {
			const res = await fetch(`/api/image/${img.id}/flag`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
				body: `flag=${flag}`
			});
			if (res.ok) {
				img.flag = flag;
			}
		} catch (e) {
			console.error('Failed to set flag:', e);
			toast.error('Failed to set flag');
		}
	}

	async function handleContextMenuRating(img: GalleryImage, rating: number) {
		try {
			const res = await fetch(`/api/image/${img.id}`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
				body: `rating=${rating}`
			});
			if (res.ok) {
				img.rating = rating;
			}
		} catch (e) {
			console.error('Failed to set rating:', e);
			toast.error('Failed to set rating');
		}
	}

	async function handleContextMenuDelete(img: GalleryImage) {
		const ok = await modal.confirm({
			title: 'Delete Image',
			message: 'Do you really want to delete this image?',
			confirmText: 'Delete',
			danger: true
		});
		if (ok) {
			try {
				const formData = new FormData();
				formData.append('image_id', String(img.id));
				const res = await fetch('/api/image/delete', {
					method: 'POST',
					body: formData
				});
				if (res.ok) {
					galleryImages = galleryImages.filter(i => i.id !== img.id);
					toast.success('Image deleted');
				} else {
					toast.error('Failed to delete image');
				}
			} catch (e) {
				console.error('Failed to delete image:', e);
				toast.error('Failed to delete image');
			}
		}
	}

	async function removeFlags() {
		const ok = await modal.confirm({
			title: 'Remove Flags',
			message: 'Remove all flags in this gallery?',
			confirmText: 'Remove',
			danger: true
		});
		if (!ok) return;
		await fetch(`/api/gallery/${data.gallery.name}/unflag`, { method: 'POST' });
		await invalidateAll();
		toast.info('All flags removed');
	}

	function handleBatchSuccess(action: 'delete' | 'move' | 'copy', criteria: string, ids: number[]) {
		if ((action === 'delete' || action === 'move') && criteria === 'selected') {
			const idSet = new Set(ids);
			galleryImages = galleryImages.filter(i => !idSet.has(i.id));
		} else {
			invalidateAll();
		}
		selectedImages = [];
	}

	async function runBatchAction(action: 'delete' | 'move' | 'copy', criteria = 'selected') {
		if (criteria === 'selected' && selectedImages.length === 0) return;

		if (action === 'delete') {
			const count = criteria === 'selected' ? `${selectedImages.length} selected` : 'all matching';
			const ok = await modal.confirm({
				title: 'Delete Images',
				message: `Do you really want to delete ${count} images?`,
				confirmText: 'Delete',
				danger: true
			});
			if (!ok) return;
		}

		const formData = new FormData();
		formData.append('criteria', criteria);
		formData.append('batch_action', action);
		formData.append('source_gallery', data.gallery.name);

		const ids = selectedImages.map(img => img.id);
		if (criteria === 'selected') {
			formData.append('image_ids', ids.join(','));
		}

		const res = await fetch('/api/gallery/batch', { method: 'POST', body: formData });
		if (res.ok) {
			handleBatchSuccess(action, criteria, ids);
			toast.success(`Batch action completed`);
		} else {
			toast.error(`Batch action failed`);
		}
	}

	function toggleSelectedCurrent() {
		const img = galleryImages[focusedIndex];
		if (!img || !img.id) return;

		const isSelected = selectedImages.some(i => i.id === img.id);

		if (isSelected) {
			selectedImages = selectedImages.filter(i => i.id !== img.id);
		} else {
			selectedImages = [...selectedImages, img];
		}
	}

	function checkRow(index: number) {
		const currentCard = document.querySelector(`[data-index="${index}"]`) as HTMLElement;
		const firstCard = document.querySelector(`[data-index="0"]`) as HTMLElement;
		const lastCard = document.querySelector(`[data-index="${galleryImages.length - 1}"]`) as HTMLElement;

		if (!currentCard || !firstCard || !lastCard) return { isFirstRow: false, isLastRow: false };

		const isFirstRow = Math.abs(currentCard.offsetTop - firstCard.offsetTop) < 20;
		const isLastRow = Math.abs(currentCard.offsetTop - lastCard.offsetTop) < 20;

		return { isFirstRow, isLastRow };
	}

	function navigate(step: number) {
		isKeyboardMode = true;
		const nextIndex = focusedIndex + step;
		if (nextIndex >= 0 && nextIndex < galleryImages.length) {
			const container = document.querySelector('.main-content');
			const { isFirstRow, isLastRow } = checkRow(nextIndex);
			if (isFirstRow) {
				container?.scrollTo({ top: 0, behavior: 'auto' });
			} else if (isLastRow) {
				container?.scrollTo({ top: container.scrollHeight, behavior: 'auto' });
			} else {
				const el = document.querySelector(`[data-index="${nextIndex}"]`);
				el?.scrollIntoView({ block: 'nearest', behavior: 'auto' });
			}
			focusedIndex = nextIndex;
		} else {
			const targetPage = currentPage + step;
    		if (!infiniteScroll && targetPage >= 1 && targetPage <= (data.images.last_page || 1)) {
				changePage(targetPage, step < 0);
        		focusedIndex = step < 0 ? galleryImages.length - 1 : 0;
    		}
		}
	}

	function navigateVert(direction: 'up' | 'down') {
		isKeyboardMode = true;
		const currentEl = document.querySelector(`[data-index="${focusedIndex}"]`) as HTMLElement;
		if (!currentEl) return;

		const currLeft = currentEl.offsetLeft;
		const currRight = currLeft + currentEl.offsetWidth;
		const currCenter = (currLeft + currRight) / 2;
		const currTop = currentEl.offsetTop

		const cards = Array.from(document.querySelectorAll('.gallery-grid [data-index]')) as HTMLElement[];
		const allCards = cards.slice(Math.max(0, focusedIndex - 150), focusedIndex + 151);

		let targetRowCards: HTMLElement[] = [];

		if (direction === 'down') {
			const belowTops = allCards.map(c => c.offsetTop).filter(top => top > currTop + 20);
			if (belowTops.length === 0) {
				if (!infiniteScroll && currentPage < data.images.last_page) {
					changePage(currentPage + 1, false);
					focusedIndex = 0;
				}
				return;
			}
			const nextRowTop = belowTops.reduce((a, b) => (b < a ? b : a));
			targetRowCards = allCards.filter(c => Math.abs(c.offsetTop - nextRowTop) < 20);
		} else {
			const aboveTops = allCards.map(c => c.offsetTop).filter(top => top < currTop - 20);
			if (aboveTops.length === 0) {
				if (!infiniteScroll && currentPage > 1) {
					changePage(currentPage - 1, true);
					focusedIndex = galleryImages.length - 1;
				}
				return;
			}
			const prevRowTop = aboveTops.reduce((a, b) => (b > a ? b : a));
			targetRowCards = allCards.filter(c => Math.abs(c.offsetTop - prevRowTop) < 20);
		}

		let bestCard: HTMLElement | null = null;
		let highestScore = -Infinity;
		let newIndex = null;

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
			if (newIndex === null) {
				const attr = bestCard.getAttribute('data-index')
				newIndex = attr !== null ? Number(attr) : NaN;
			}
			if (!isNaN(newIndex)) {
				const container = document.querySelector('.main-content');
				const { isFirstRow, isLastRow } = checkRow(newIndex);
				if (isFirstRow) {
					container?.scrollTo({ top: 0, behavior: 'auto' });
				} else if (isLastRow) {
					container?.scrollTo({ top: container?.scrollHeight, behavior: 'auto' });
				} else {
					bestCard.scrollIntoView({ block: 'nearest', behavior: 'auto' });
				}
				focusedIndex = newIndex;
			}
		}
	}

	function handleMouseMove(e: MouseEvent) {
		if (Math.abs(e.movementX) > 0 || Math.abs(e.movementY) > 0) {
			isKeyboardMode = false;
		}
	}

        async function copyImage() {
                try {
                        const img = galleryImages[focusedIndex]
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

	function handleKeyDown(e: KeyboardEvent) {
		const tag = (e.target as HTMLElement)?.tagName;
		if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;

		if (carousel.isOpen || batchIsOpen || isCullingOpen) return;

		if (galleryImages.length === 0) return;

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
				carousel.open(galleryImages, focusedIndex);
				break;
			case '0': case '1': case '2': case '3': case '4': case '5':
				setRating(Number(e.key));
				break;
			case 'm':
				setFlag(1);
				break;
			case 'n':
				setFlag(-1);
				break;
			case 'u':
				setFlag(0);
				break;
			case ' ':
				e.preventDefault();
				toggleSelectedCurrent();
				break;
			case 'c':
                                e.preventDefault();
                                if (e.ctrlKey) {
                                        copyImage();
                                } else {
                                        isCullingOpen = true;
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
                        case 'r':
                                rescanGallery()
                }
	}

	function handleGlobalClick(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (!target || typeof target.closest !== 'function') return;

		if (!target.closest('.batch-dialog, .inline-edit-form, .btn-icon, .page-dots-btn, .page-input-wrapper')) {
			batchIsOpen = false;
			if (isEditingName) cancelEditingName();
			jumpInputIndex = null;
		}
	}
</script>

<svelte:window
	onclick={handleGlobalClick}
	onkeydown={handleKeyDown}
	onmousemove={handleMouseMove}
	ondragenter={handleWindowDragEnter}
	ondragleave={handleWindowDragLeave}
	ondragover={handleWindowDragOver}
	ondrop={handleWindowDrop}
/>

{#if isWindowDragging}
	<div
		class="drag-drop-overlay"
		transition:fade={{ duration: 120 }}
		ondragover={handleWindowDragOver}
		ondrop={handleWindowDrop}
	>
		<div class="drop-zone-card">
			<span class="material-symbols-outlined drop-icon">image_search</span>
			<h2>Drop image to search</h2>
			<p>Visually search for similar images in this gallery</p>
		</div>
	</div>
{/if}

<div class="gallery-header">
	{#if isEditingName}
		<form class="inline-edit-form" onsubmit={(e) => { e.preventDefault(); saveGalleryName(); }}>
			<input
				bind:this={editInputEl}
				bind:value={editedName}
				class="inline-edit-input"
				onkeydown={(e) => { if (e.key === 'Escape') cancelEditingName(); }}
				onblur={saveGalleryName}
			/>
			<button type="submit" style="display: none;"></button>
		</form>
	{:else}
		<h1 ondblclick={startEditingName} title="Double Click to rename">{data.gallery.heading || data.gallery.name}</h1>
	{/if}

	<div class="header-actions">
		<button
			class="image-count"
			class:active={infiniteScroll}
			onclick={toggleInfiniteScroll}
			title="Toggle Infinite Scrolling"
		>
			{data.gallery.count} images
		</button>
		
		{#if selectedImages.length > 0}
			<span class="selected-count">{selectedImages.length}</span>
		{/if}

		<div class="divider"></div>

		<button class="btn-icon" class:active={isEditingName} title="Edit Gallery Name" onclick={isEditingName ? saveGalleryName : startEditingName}>
			<span class="material-symbols-outlined">{isEditingName ? 'check' : 'edit'}</span>
		</button>
		<button class="btn-icon" title="Rescan Gallery" onclick={rescanGallery}>
			<span class="material-symbols-outlined">refresh</span>
		</button>
		<button class="btn-icon" title="Culling Mode" onclick={() => {
			const nextUnflagged = galleryImages.findIndex(img => !img.flag);

			focusedIndex = nextUnflagged !== -1 ? nextUnflagged : 0;
			isCullingOpen = true;
		}}>
			<span class="material-symbols-outlined">bolt</span>
		</button>

		<div class="divider"></div>

		<button class="btn-icon" title="Remove flags" onclick={removeFlags}>
			<span class="material-symbols-outlined">block</span>
		</button>

		<div style="position: relative;">
			<button class="btn-icon" class:active={batchIsOpen} title="Batch Actions" onclick={() => batchIsOpen = !batchIsOpen}>
				<span class="material-symbols-outlined">more_vert</span>
			</button>

			{#if batchIsOpen}
				<BatchActionDialog
					selectedImages={selectedImages}
					onClose={() => batchIsOpen = false}
					onSuccess={handleBatchSuccess}
				/>
			{/if}
		</div>
	</div>

	<div class="search-container">
		<form class="search-form" onsubmit={handleSearch}>
			<div
				class="input-wrapper"
				class:active={searchQuery.trim() !== '' || Boolean(droppedImagePreview)}
				class:drag-over={isDragging}
				ondragover={(e) => { e.preventDefault(); isDragging = true; }}
				ondragleave={() => isDragging = false}
				ondrop={handleDropImage}
			>
				{#if droppedImagePreview}
					<img src={droppedImagePreview} alt="Search Preview" class="search-thumb" />
					<span class="search-thumb-label">Image Search</span>
					<button type="button" class="clear-thumb-btn" onclick={clearImageSearch} title="Clear Image Search">
						<span class="material-symbols-outlined">close</span>
					</button>
				{:else}
					<span class="material-symbols-outlined search-icon">search</span>
					<input
						type="search"
						placeholder="Search text or drop image..."
						bind:value={searchQuery}
						onpaste={(e) => {
							const item = e.clipboardData?.items?.[0];
							if (item?.type.startsWith('image/')) {
								const file = item.getAsFile();
								if (file) {
									e.preventDefault();
									searchByImage(file);
								}
							}
						}}
					/>
				{/if}
			</div>

			<div class="threshold-wrapper" title="Search Precision" class:active={searchThreshold != '0.9'}>
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

			{#if hasActiveFilters}
				<div class="divider"></div>
				<button
					class="filter-pill reset-pill"
					onclick={resetFilters}
					title="Reset all filters and search"
				>
					<span class="material-symbols-outlined">filter_alt_off</span>
				</button>
			{/if}
		</div>
	</div>
</div>

{#if navigating.to}
	<div class="loader-container">
		<Spinner />
	</div>
{/if}

{#if galleryImages.length === 0 && !navigating.to}
	<div class="empty-state">
		<span class="material-symbols-outlined empty-icon">
			{page.url.searchParams.get('q') ? 'search_off' : currentFlag ? 'filter_alt_off' : 'photo_library'}
		</span>
		<h3>
			{page.url.searchParams.get('q')
				? 'No results found' 
				: currentFlag 
					? 'No images match the filter' 
					: 'This gallery is empty'}
		</h3>
		<p>
			{#if page.url.searchParams.get('q')}
				No matching images found for "{searchQuery}". Try a different search term or adjust the threshold.
			{:else if currentFlag}
				There are no images matching your current flag filter.
			{:else}
				No image files have been indexed in this folder yet.
			{/if}
		</p>
		
		{#if page.url.searchParams.get('q') || currentFlag}
			<button class="empty-action-btn" onclick={() => goto(`/gallery/${data.gallery.name}`)}>
				<span class="material-symbols-outlined">restart_alt</span>
				Reset filters
			</button>
		{:else}
			<button class="empty-action-btn" onclick={rescanGallery}>
				<span class="material-symbols-outlined">sync</span>
				Scan gallery
			</button>
		{/if}
	</div>
{:else}
	<div class="gallery-grid medium" class:keyboard-nav={isKeyboardMode} class:loading={!!navigating.to}>
		{#if navigating.to}
			<div class="grid-veil" transition:fade={{ duration: 150 }}></div>
		{/if}
		<hr />
		{#if focusBox.visible && isKeyboardMode}
			<div
				class="floating-focus"
				style="transform: translate({focusBox.x}px, {focusBox.y}px); width: {focusBox.w}px; height: {focusBox.h}px;"
			></div>
		{/if}

		{#each galleryImages as image, index (image.filepath)}
			<ImageCard
				{image}
				dataIndex={index}
				onmousemove={() => {
					if (!isKeyboardMode && focusedIndex !== index) {
						focusedIndex = index;
					}
				}}
				isFocused={!isKeyboardMode && focusedIndex === index} 
				isSelected={selectedIds.has(image.id)}
				showMeta={false}
				onclick={() => {
					focusedIndex = index;
					carousel.open(galleryImages, index);
				}}
				oncontextmenu={(e) => {
					e.preventDefault();
					e.stopPropagation();
					focusedIndex = index;
					if (contextMenu && contextMenu.image.id === image.id) {
						contextMenu = null;
					} else {
						contextMenu = { x: e.clientX, y: e.clientY, image };
					}
				}}
			/>
		{/each}
	</div>
{/if}

{#if infiniteScroll && hasMore}
	<div use:viewPort={loadMore} class="infinite-trigger">
		{#if isLoadingMore}
			<div class="loader-container">
				<Spinner />
			</div>
		{/if}
	</div>
{/if}

{#if !infiniteScroll && galleryImages.length > 0}
	<div class="pagination">
		{#if currentPage > 1}
			<button class="filter-pill" onclick={() => changePage(currentPage - 1)}>
				<span class="material-symbols-outlined">arrow_back_ios</span>
			</button>
		{/if}
		<div class="page-numbers">
			{#each pageButtons as btn, i}
				{#if btn === '...'}
					<button
						type="button"
						class="filter-pill page-dots-btn"
						class:active={jumpInputIndex === i}
						title="Jump to Page..."
						onclick={() => {
							if (jumpInputIndex !== i) {
								jumpInputIndex = i;
								jumpPageVal = '';
							}
						}}
					>
						{#if jumpInputIndex === i}
							<input
								type="number"
								class="page-jump-input"
								min="1"
								max={data.images.last_page || 1}
								placeholder="#"
								bind:value={jumpPageVal}
								autofocus
								onkeydown={(e) => {
									if (e.key === 'Enter') {
										const target = Number(jumpPageVal);
										if (target >= 1 && target <= (data.images.last_page || 1)) {
											changePage(target);
										}
										jumpInputIndex = null;
									} else if (e.key === 'Escape') {
										jumpInputIndex = null;
									}
								}}
								onblur={() => {
									const target = Number(jumpPageVal);
									if (jumpPageVal && target >= 1 && target <= (data.images.last_page || 1)) {
										changePage(target);
									}
									jumpInputIndex = null;
								}}
							/>
						{:else}
							<span>...</span>
						{/if}
					</button>
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

{#if contextMenu}
	<ContextMenu
		x={contextMenu.x}
		y={contextMenu.y}
		image={contextMenu.image}
		galleryName={data.gallery.name}
		onClose={() => contextMenu = null}
		onFlag={handleContextMenuFlag}
		onRating={handleContextMenuRating}
		onDelete={handleContextMenuDelete}
		onMove={(img) => {
			selectedImages = [img];
			batchIsOpen = true;
		}}
	/>
{/if}

<CullingMode
	bind:isOpen={isCullingOpen}
	bind:images={galleryImages}
	bind:currentIndex={focusedIndex}
	hasNextPage={cullingHasNextPage}
	hasPrevPage={cullingHasPrevPage}
	isPaging={isCullingPaging}
	onNextPage={cullingNextPage}
	onPrevPage={cullingPrevPage}
	onClose={() => isCullingOpen = false}
/>

<style>
	span {
		font-size: 1rem;
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
		padding: 0 0 24px 0;
		cursor: pointer;
	}

	.inline-edit-form {
		grid-area: title;
		margin-bottom: 24px;
		display: flex;
		align-items: center;
	}

	.inline-edit-input {
		font-size: 3.5rem;
		font-weight: 700;
		line-height: 1.1;
		letter-spacing: -1px;
		text-transform: uppercase;
		color: var(--text);
		background: var(--bg-dark);
		border: 2px solid var(--primary);
		border-radius: 8px;
		padding: 4px 16px;
		outline: none;
		box-shadow: 0 0 20px color-mix(in srgb, var(--primary) 35%, transparent);
		font-family: inherit;
		max-width: 600px;
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
		border-radius: 0;
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
		z-index: 1
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
		border-radius: 0;
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
		border-radius: 0;
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

	.input-wrapper.drag-over {
		outline: 2px dashed var(--primary);
		outline-offset: 2px;
		background: color-mix(in srgb, var(--primary) 12%, transparent);
	}

	.search-thumb {
		width: 28px;
		height: 28px;
		object-fit: cover;
		border-radius: 4px;
		margin-left: 12px;
		border: 1px solid color-mix(in srgb, var(--secondary) 40%, var(--bg-light));
	}

	.search-thumb-label {
		font-size: 0.85rem;
		color: var(--text);
		margin-left: 8px;
		font-weight: 500;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.clear-thumb-btn {
		background: transparent;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 4px;
		margin-left: auto;
		margin-right: 8px;
		border-radius: 4px;
		transition: all 0.12s ease;
	}

	.clear-thumb-btn:hover {
		color: var(--danger);
		background: var(--bg-light);
	}

	.clear-thumb-btn .material-symbols-outlined {
		font-size: 1.1rem;
	}

	.drag-drop-overlay {
		position: fixed;
		inset: 0;
		z-index: 99999;
		background: color-mix(in srgb, var(--bg-dark) 82%, transparent);
		backdrop-filter: blur(10px);
		-webkit-backdrop-filter: blur(10px);
		display: flex;
		align-items: center;
		justify-content: center;
		pointer-events: all;
	}

	.drop-zone-card {
		border: 2px dashed var(--primary);
		border-radius: 16px;
		background: var(--bg);
		padding: 48px 64px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		text-align: center;
		box-shadow: 0 16px 40px rgba(0, 0, 0, 0.7);
	}

	.drop-icon {
		font-size: 3.5rem;
		color: var(--primary);
	}

	.drop-zone-card h2 {
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--text);
		margin: 0;
	}

	.drop-zone-card p {
		font-size: 0.95rem;
		color: var(--text-muted);
		margin: 0;
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
        appearance: textfield;
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
		background-color: var(--divider);
		margin: 0 4px;
	}

	.gallery-grid {
		position: relative;
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
		padding: 0 24px 24px;
	}

	.grid-veil {
		position: absolute;
		inset: 0;
		background: var(--bg-dark);
		opacity: 0.5;
		pointer-events: none;
		z-index: 20;
	}

	.gallery-grid.medium {
		--grid-base: 250px;
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

	.page-dots-btn {
		letter-spacing: 2px;
		font-size: 1.1rem;
		padding: 0;
	}

	.page-dots-btn .page-jump-input {
		width: 100%;
		height: 100%;
		background: transparent;
		border: none;
		color: var(--bg-dark);
		text-align: center;
		font-size: 0.95rem;
		font-weight: 600;
		padding: 0;
		outline: none;
        appearance: textfield;
		-moz-appearance: textfield;
	}

	.page-dots-btn .page-jump-input::-webkit-inner-spin-button,
	.page-dots-btn .page-jump-input::-webkit-outer-spin-button {
		-webkit-appearance: none;
		margin: 0;
	}

	.page-dots-btn.active::before {
		height: 100%;
		border-radius: 8px;
		background: var(--primary);
	}

	.filter-pill.page-dots-btn.active::before {
		height: 100%;
		border-radius: 8px;
		background: var(--tertiary);
	}

	.filter-pill.reset-pill {
		color: var(--danger);
	}

	.filter-pill.reset-pill::before {
		background: var(--danger);
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
		border-radius: 0;
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

	.gallery-grid.loading {
		pointer-events: none;
	}

	.infinite-trigger {
		width: 100%;
		height: 100px;
		display: flex;
		justify-content: center;
		align-items: center;
	}


	.floating-focus {
		position: absolute;
		top: 0;
		left: 0;
		pointer-events: none;
		border-radius: 8px;
		box-shadow: inset 0 0 0 3px var(--primary), 0 0 25px color-mix(in srgb, var(--primary) 40%, transparent);
		z-index: 10;
		transition:
			transform 0.18s cubic-bezier(0.2, 0, 0, 1),
			width 0.18s cubic-bezier(0.2, 0, 0, 1),
			height 0.18s cubic-bezier(0.2, 0, 0, 1);
	}
	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 80px 24px;
		text-align: center;
		color: var(--text-muted);
		gap: 12px;
		max-width: 460px;
		margin: 60px auto;
	}

	.empty-state .empty-icon {
		font-size: 3.5rem;
		color: color-mix(in srgb, var(--primary) 70%, transparent);
		margin-bottom: 6px;
	}

	.empty-state h3 {
		font-size: 1.25rem;
		font-weight: 600;
		color: var(--text);
		margin: 0;
	}

	.empty-state p {
		font-size: 0.95rem;
		line-height: 1.5;
		margin: 0;
	}

	.empty-state .empty-action-btn {
		margin-top: 12px;
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 10px 20px;
		background: color-mix(in srgb, var(--primary) 15%, transparent);
		border: 1px solid color-mix(in srgb, var(--primary) 30%, transparent);
		border-radius: 10px;
		color: var(--text);
		font-size: 0.95rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s ease;
	}

	.empty-state .empty-action-btn:hover {
		background: var(--primary);
		color: var(--bg-dark);
		border-color: var(--primary);
		transform: translateY(-1px);
	}
</style>
