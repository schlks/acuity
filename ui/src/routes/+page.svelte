<script lang="ts">
	import { goto } from '$app/navigation';
	import { fade } from 'svelte/transition';
	import AddGalleryDialog from '$lib/components/AddGalleryDialog.svelte';
	import SettingsDialog from '$lib/components/SettingsDialog.svelte';
	import { toast } from '$lib/stores/toast.svelte';

	let { data } = $props();
	let searchQuery = $state('');
	let showAddGallery = $state(false);
	let showSettings = $state(false);
	let dragDepth = $state(0);
	let isWindowDragging = $derived(dragDepth > 0);

	function handleSearch(e: SubmitEvent | Event) {
		e.preventDefault();
		const query = searchQuery.trim();
		if (query && data.galleries?.length > 0) {
			goto(`/gallery/${encodeURIComponent(data.galleries[0].name)}?q=${encodeURIComponent(query)}`);
		}
	}

	function isImageFile(file: File): boolean {
		if (file.type && file.type.startsWith('image/')) return true;
		const ext = file.name.split('.').pop()?.toLowerCase();
		return ['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp', 'avif', 'tiff', 'svg', 'heic', 'jxl'].includes(ext || '');
	}

	function hasFilePayload(e: DragEvent): boolean {
		if (!e.dataTransfer) return false;
		return e.dataTransfer.types.includes('Files') ||
		       e.dataTransfer.types.includes('text/uri-list') ||
		       e.dataTransfer.types.includes('text/plain');
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
		if (hasFilePayload(e)) {
			e.preventDefault();
		}
	}

	function parseImageUri(uri: string): string | null {
		const lines = uri.split(/\r?\n/).map(l => l.trim()).filter(l => l && !l.startsWith('#'));
		if (lines.length === 0) return null;
		let path = decodeURIComponent(lines[0].replace(/^file:\/\//, ''));
		const ext = path.split('.').pop()?.toLowerCase();
		if (['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp', 'avif', 'tiff', 'svg', 'heic', 'jxl'].includes(ext || '')) {
			return path;
		}
		return null;
	}

	async function handleWindowDrop(e: DragEvent) {
		e.preventDefault();
		dragDepth = 0;
		const file = e.dataTransfer?.files?.[0];
		if (file && isImageFile(file)) {
			if (data.galleries && data.galleries.length > 0) {
				goto(`/gallery/${encodeURIComponent(data.galleries[0].name)}`);
			} else {
				toast.info('Please import a gallery first');
				showAddGallery = true;
			}
			return;
		}

		const uriList = e.dataTransfer?.getData('text/uri-list') || e.dataTransfer?.getData('text/plain');
		if (uriList) {
			const path = parseImageUri(uriList);
			if (path) {
				if (data.galleries && data.galleries.length > 0) {
					goto(`/gallery/${encodeURIComponent(data.galleries[0].name)}`);
				} else {
					toast.info('Please import a gallery first');
					showAddGallery = true;
				}
				return;
			}
		}

		showAddGallery = true;
	}
</script>

<svelte:window
	ondragenter={handleWindowDragEnter}
	ondragleave={handleWindowDragLeave}
	ondragover={handleWindowDragOver}
	ondrop={handleWindowDrop}
/>

{#if isWindowDragging}
	<div class="drag-drop-overlay" transition:fade={{ duration: 120 }}>
		<div class="drop-zone-card">
			<span class="material-symbols-outlined drop-icon">image_search</span>
			<h2>Drop to search or import</h2>
			<p>Drop images to visually search or folders to import</p>
		</div>
	</div>
{/if}

<div class="landing-page">
	<header class="hero-section">
		<img src="/logo.svg" alt="Acuity Logo" class="hero-logo" width="60" height="60" />
		<h1>Acuity</h1>
		<p class="hero-subtitle">High-performance image curation & gallery organizer</p>

		{#if (data.galleries || []).length > 0}
			<form class="search-form" onsubmit={handleSearch}>
				<div class="input-wrapper" class:active={searchQuery.trim() !== ''}>
					<span class="material-symbols-outlined search-icon">search</span>
					<input type="search" placeholder="Search across collections..." bind:value={searchQuery} />
				</div>
				<button type="submit" style="display: none;"></button>
			</form>
		{/if}
	</header>

	{#if (data.galleries || []).length > 0}
		<div class="quick-actions">
			<button type="button" class="quick-btn" onclick={() => showAddGallery = true}>
				<span class="material-symbols-outlined icon">add_circle</span>
				<span>Import Gallery</span>
			</button>
			<a href="/gallery/global/duplicates" class="quick-btn">
				<span class="material-symbols-outlined icon">auto_awesome</span>
				<span>Duplicate Finder</span>
			</a>
			<button type="button" class="quick-btn" onclick={() => showSettings = true}>
				<span class="material-symbols-outlined icon">settings</span>
				<span>Settings</span>
			</button>
		</div>
	{/if}

	<section class="collections-section">
		{#if (data.galleries || []).length > 0}
			<div class="section-header">
				<span class="section-title">Collections ({data.galleries.length})</span>
			</div>

			<div class="gallery-grid">
				{#each data.galleries as gallery}
					{@const subCount = gallery.sub_folders?.length || 0}
					<a href="/gallery/{encodeURIComponent(gallery.name)}" class="gallery-card">
						<div class="card-icon-area">
							<span class="material-symbols-outlined folder-icon">folder</span>
						</div>
						<div class="card-content">
							<div class="card-header">
								<h3>{gallery.name}</h3>
								{#if subCount > 0}
									<span class="sub-badge">{subCount} subfolders</span>
								{/if}
							</div>
							<div class="path-container">
								<span class="card-path">{gallery.path}</span>
							</div>
						</div>
					</a>
				{/each}
			</div>
		{:else}
			<button type="button" class="empty-import-box" onclick={() => showAddGallery = true}>
				<span class="material-symbols-outlined empty-icon">add_circle</span>
				<h2>Import your first collection</h2>
				<p class="empty-subtext">Click here to select an image folder and get started</p>
			</button>
		{/if}
	</section>

	{#if showAddGallery}
		<div class="modal-backdrop" onclick={() => showAddGallery = false}>
			<div class="modal-dialog import-modal" onclick={(e) => e.stopPropagation()}>
				<AddGalleryDialog onClose={() => showAddGallery = false} />
			</div>
		</div>
	{/if}

	{#if showSettings}
		<div class="modal-backdrop" onclick={() => showSettings = false}>
			<div class="modal-dialog" onclick={(e) => e.stopPropagation()}>
				<SettingsDialog onClose={() => showSettings = false} />
			</div>
		</div>
	{/if}
</div>

<style>
	.landing-page {
		max-width: 1000px;
		margin: 0 auto;
		width: 100%;
		padding: 48px 24px 64px 24px;
		display: flex;
		flex-direction: column;
		gap: 36px;
		align-items: center;
	}

	.hero-section {
		display: flex;
		flex-direction: column;
		align-items: center;
		text-align: center;
		gap: 12px;
		width: 100%;
	}

	.hero-logo {
		margin-bottom: 4px;
	}

	.hero-section h1 {
		font-size: 2.8rem;
		font-weight: 700;
		color: var(--text);
		margin: 0;
		letter-spacing: -0.02em;
	}

	.hero-subtitle {
		font-size: 0.95rem;
		color: var(--text-muted);
		margin: 0 0 12px 0;
	}

	.search-form {
		width: 100%;
		max-width: 580px;
	}

	.input-wrapper {
		position: relative;
		display: flex;
		align-items: center;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 6px 14px 6px 42px;
		transition: border-color 0.15s ease;
	}

	.input-wrapper:hover,
	.input-wrapper:focus-within {
		border-color: var(--primary);
	}

	.input-wrapper.active {
		border-color: var(--tertiary);
	}

	.search-icon {
		position: absolute;
		left: 14px;
		color: var(--text-muted);
		font-size: 1.25rem;
		pointer-events: none;
	}

	.input-wrapper input {
		width: 100%;
		background: transparent;
		border: none;
		outline: none;
		color: var(--text);
		font-size: 0.95rem;
		padding: 6px 0;
		font-family: inherit;
	}

	.quick-actions {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 12px;
		flex-wrap: wrap;
		width: 100%;
	}

	.quick-btn {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 18px;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 8px;
		color: var(--text);
		font-size: 0.88rem;
		font-weight: 500;
		text-decoration: none;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.quick-btn:hover {
		border-color: var(--primary);
		color: var(--primary);
		transform: translateY(-2px);
		background: var(--bg-light);
	}

	.quick-btn .icon {
		font-size: 1.15rem;
		color: var(--text-muted);
		transition: color 0.15s ease;
	}

	.quick-btn:hover .icon {
		color: var(--primary);
	}

	.collections-section {
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.section-header {
		display: flex;
		align-items: center;
		border-bottom: 1px solid var(--divider);
		padding-bottom: 10px;
	}

	.section-title {
		font-size: 0.8rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--info);
		font-weight: 600;
	}

	.gallery-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(290px, 1fr));
		gap: 16px;
		width: 100%;
	}

	.gallery-card {
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 18px;
		display: flex;
		flex-direction: column;
		gap: 12px;
		text-decoration: none;
		color: var(--text);
		transition: transform 0.18s cubic-bezier(0.2, 0, 0, 1),
		            border-color 0.18s ease;
		cursor: pointer;
		user-select: none;
		overflow: hidden;
	}

	.gallery-card:hover {
		transform: translateY(-3px);
		border-color: var(--primary);
	}

	.card-icon-area {
		display: flex;
		align-items: center;
	}

	.folder-icon {
		font-size: 2.2rem;
		color: var(--primary);
		transition: transform 0.18s ease;
	}

	.gallery-card:hover .folder-icon {
		color: var(--primary);
		transform: scale(1.05);
	}

	.card-content {
		display: flex;
		flex-direction: column;
		gap: 4px;
		width: 100%;
		overflow: hidden;
	}

	.card-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.card-header h3 {
		font-size: 1.05rem;
		font-weight: 600;
		color: var(--text);
		margin: 0;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.sub-badge {
		font-size: 0.7rem;
		color: var(--info);
		background: var(--bg-light);
		border: 1px solid var(--border);
		padding: 2px 8px;
		border-radius: 4px;
		font-weight: 500;
		flex-shrink: 0;
	}

	.path-container {
		width: 100%;
		overflow: hidden;
		white-space: nowrap;
		position: relative;
		mask-image: linear-gradient(to right, black 85%, transparent 100%);
	}

	.gallery-card:hover .path-container {
		mask-image: none;
	}

	.card-path {
		font-size: 0.78rem;
		color: var(--text-muted);
		margin: 0;
		white-space: nowrap;
		display: inline-block;
		will-change: transform;
	}

	.gallery-card:hover .card-path {
		animation: marquee-pingpong 6s ease-in-out infinite alternate;
	}

	@keyframes marquee-pingpong {
		0%, 20% {
			transform: translateX(0%);
		}
		80%, 100% {
			transform: translateX(calc(-100% + 220px));
		}
	}

	.empty-import-box {
		width: 100%;
		min-height: 240px;
		border: 2px dashed var(--border);
		border-radius: 12px;
		background: var(--bg);
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 12px;
		cursor: pointer;
		color: var(--text);
		transition: all 0.2s ease;
		padding: 40px 24px;
		text-align: center;
	}

	.empty-import-box:hover {
		border-color: var(--primary);
		background: var(--bg-light);
		transform: translateY(-2px);
	}

	.empty-icon {
		font-size: 3.2rem;
		color: var(--primary);
		transition: transform 0.2s ease;
	}

	.empty-import-box:hover .empty-icon {
		transform: scale(1.1);
	}

	.empty-import-box h2 {
		font-size: 1.4rem;
		font-weight: 600;
		color: var(--text);
		margin: 0;
	}

	.empty-subtext {
		font-size: 0.9rem;
		color: var(--text-muted);
		margin: 0;
	}

	.modal-backdrop {
		position: fixed;
		inset: 0;
		z-index: 10000;
		background: color-mix(in srgb, var(--bg-dark) 85%, transparent);
		backdrop-filter: blur(14px);
		-webkit-backdrop-filter: blur(14px);
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 24px;
	}

	.modal-dialog {
		position: relative;
		width: 100%;
		max-width: 480px;
	}

	.modal-dialog.import-modal {
		max-width: 600px;
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
		pointer-events: none;
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
</style>
