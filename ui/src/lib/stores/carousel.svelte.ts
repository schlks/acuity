import { PreviewBlobCache } from '$lib/stores/previewBlobCache.svelte';

class CarouselStore {
	isOpen = $state(false);
	isInfoOpen = $state(false);
	mode = $state('normal');
	images = $state<any[]>([]);

	#currentIndex = $state(0);
	previewCache = new PreviewBlobCache();

	get currentIndex() {
		return this.#currentIndex;
	}
	set currentIndex(value: number) {
		this.#currentIndex = value;
		this.syncWindow();
	}

	get displaySrcs() {
		return this.previewCache.displaySrcs;
	}

	currentImage = $derived(this.images[this.currentIndex]);
	hasNext = $derived(this.currentIndex < this.images.length - 1);
	hasPrev = $derived(this.currentIndex > 0);
	visibleImages = $derived.by(() => {
		let items = [];

		if (this.hasPrev) {
			items.push({ image: this.images[this.currentIndex - 1], offset: -1 });
		}

		if (this.images[this.currentIndex]) {
			items.push({ image: this.images[this.currentIndex], offset: 0 });
		}

		if (this.hasNext) {
			items.push({ image: this.images[this.currentIndex + 1], offset: 1 });
		}

		return items;
	});

	toggleInfo() {
		this.isInfoOpen = !this.isInfoOpen;
	}

	open(images: any[], startIndex = 0, mode: 'normal' | 'culling' = 'normal') {
		this.images = images;
		this.mode = mode;
		this.isOpen = true;
		this.currentIndex = startIndex;
	}

	close() {
		this.isOpen = false;
		this.images = [];
		this.isInfoOpen = false;
		this.previewCache.releaseAll();
	}

	next() {
		if (this.hasNext) this.currentIndex++;
	}

	prev() {
		if (this.hasPrev) this.currentIndex--;
	}

	syncWindow() {
		const windowPaths = this.visibleImages.map((item) => item.image.filepath);
		this.previewCache.syncWindow(windowPaths);
	}
}

export const carousel = new CarouselStore();
