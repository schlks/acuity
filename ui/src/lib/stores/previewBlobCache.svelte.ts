export class PreviewBlobCache {
	displaySrcs = $state<Record<string, string>>({});

	blobCache = new Map<string, string>();
	pendingFetches = new Map<string, Promise<string>>();

	previewUrl(filepath: string): string {
		if (typeof window === 'undefined') {
			return `/api/image?path=${encodeURIComponent(filepath)}&preview=true`;
		}
		const dpr = window.devicePixelRatio || 1;
		const width = Math.min(3840, Math.max(640, Math.ceil(window.innerWidth * dpr)));
		return `/api/image?path=${encodeURIComponent(filepath)}&preview=true&width=${width}`;
	}

	async loadBlob(filepath: string): Promise<string> {
		if (this.blobCache.has(filepath)) return this.blobCache.get(filepath)!;
		if (this.pendingFetches.has(filepath)) return this.pendingFetches.get(filepath)!;

		const promise = (async () => {
			const res = await fetch(this.previewUrl(filepath));
			const blob = await res.blob();
			const url = URL.createObjectURL(blob);
			this.blobCache.set(filepath, url);
			this.displaySrcs[filepath] = url;
			return url;
		})();

		this.pendingFetches.set(filepath, promise);
		try {
			return await promise;
		} finally {
			this.pendingFetches.delete(filepath);
		}
	}

	syncWindow(keepPaths: string[]) {
		if (typeof window === 'undefined') return;

		const keep = new Set(keepPaths);

		for (const [path, url] of this.blobCache) {
			if (!keep.has(path)) {
				URL.revokeObjectURL(url);
				this.blobCache.delete(path);
				delete this.displaySrcs[path];
			}
		}

		for (const path of keepPaths) {
			this.loadBlob(path);
		}
	}

	releaseAll() {
		for (const url of this.blobCache.values()) URL.revokeObjectURL(url);
		this.blobCache.clear();
		this.displaySrcs = {};
	}
}
