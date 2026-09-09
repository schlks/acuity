class CarouselStore {
	isOpen = $state(false);
	isInfoOpen = $state(false);
	mode = $state('normal');
	images = $state<any[]>([]);
	currentIndex = $state(0);

	currentImage = $derived(this.images[this.currentIndex]);
	hasNext = $derived(this.currentIndex < this.images.length - 1);
	hasPrev = $derived(this.currentIndex > 0);
	visibleImages = $derived.by(() => {
		let items = [];
		
		// 1. Das linke Bild (falls vorhanden)
		if (this.hasPrev) {
			items.push({ image: this.images[this.currentIndex - 1], offset: -1 });
		}
		
		// 2. Das aktuelle Bild in der Mitte
		if (this.images[this.currentIndex]) {
			items.push({ image: this.images[this.currentIndex], offset: 0 });
		}
		
		// 3. Das rechte Bild (falls vorhanden)
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
		this.currentIndex = startIndex;
		this.mode = mode;
		this.isOpen = true;
		this.preloadNeighbors();
	}

	close() {
		this.isOpen = false;
		this.images = [];
		this.isInfoOpen = false;
	}

	next() {
		if (this.hasNext) this.currentIndex++;
		this.preloadNeighbors();
	}

	prev() {
		if (this.hasPrev) this.currentIndex--;
		this.preloadNeighbors();
	}

	preloadNeighbors() {
		if (typeof window === 'undefined') return;
		
		if (this.hasNext) {
			const nextImg = new Image();
			nextImg.src = `/api/image?path=${encodeURIComponent(this.images[this.currentIndex + 1].filepath)}&preview=true`;
		}
		if (this.hasPrev) {
			const prevImg = new Image();
			prevImg.src = `/api/image?path=${encodeURIComponent(this.images[this.currentIndex - 1].filepath)}&preview=true`;
		}
	}
}

export const carousel = new CarouselStore();
