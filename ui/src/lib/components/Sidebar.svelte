<script lang="ts">
	import { page } from '$app/state';
	import { goto, invalidateAll } from '$app/navigation';
	import { slide } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import { modal } from '$lib/stores/modal.svelte';
	import { toast } from '$lib/stores/toast.svelte';
	import SettingsDialog from './SettingsDialog.svelte';
	import AddGalleryDialog from './AddGalleryDialog.svelte';
	import MoveFolderDialog from './MoveFolderDialog.svelte';

	let { galleries } = $props();
	let showSettings = $state(false);
	let showAddGallery = $state(false);
	let moveFolder = $state<{ gallery: string; name: string; path: string } | null>(null);
	let expandedGalleries = $state<Record<string, boolean>>({});

	async function deleteGallery(name: string) {
		const ok = await modal.confirm({
			title: 'Delete Gallery',
			message: `Do you really want to remove the gallery "${name}" from Acuity? (Your photos on disk will not be deleted)`,
			confirmText: 'Remove',
			danger: true
		});
		if (!ok) return;

		try {
			const res = await fetch(`/api/gallery/${encodeURIComponent(name)}`, {
				method: 'DELETE'
			});
			if (res.ok) {
				toast.success('Gallery deleted');
				const currentGallery = page.params?.name ? decodeURIComponent(page.params.name) : '';
				if (currentGallery === name || window.location.pathname.includes(`/gallery/`)) {
					await goto('/', { replaceState: true });
				}
				await invalidateAll();
			} else {
				toast.error('Failed to delete gallery');
			}
		} catch (error) {
			console.error(error);
			toast.error('Failed to delete gallery');
		}
	}

	function handleGlobalClick(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (!target || typeof target.closest !== 'function') return;

		if (!target.closest('.settings-panel, .dialog-panel, .settings-container, .nav-item.add-btn')) {
			showSettings = false;
			showAddGallery = false;
		}
	}
</script>

<svelte:window onclick={handleGlobalClick} />

<aside class="sidebar-left">
	<a href="/" class="logo-link">
		<img src="/logo.svg" alt="Acuity Logo" width="32" height="32" />
		<h1>Acuity</h1>
	</a>

	{#if galleries.length > 0}
		<hr />
	{/if}

	<nav class="nav-list">
		<div class="nav-scroll">
			{#each galleries as gallery}
				{@const subFolders = gallery.sub_folders || []}
				{@const isExpanded = expandedGalleries[gallery.name] ?? page.params.name === gallery.name}
				<div class="gallery-block">
					<div class="gallery-item-wrapper">
						{#if subFolders.length > 0}
							<button
								class="toggle-btn"
								onclick={() => (expandedGalleries[gallery.name] = !isExpanded)}
								title="Toggle Subfolders"
							>
								<span class="material-symbols-outlined chevron-icon" class:rotated={isExpanded}
									>chevron_right</span
								>
							</button>
						{/if}
						<a
							href="/gallery/{encodeURIComponent(gallery.name)}"
							class="nav-item"
							class:active={page.params.name === gallery.name}
						>
							<span class="material-symbols-outlined"
								>{page.params.name === gallery.name ? 'folder_open' : 'folder'}</span
							>
							{gallery.name}
						</a>
						<button
							class="delete-btn"
							onclick={() => deleteGallery(gallery.name)}
							title="delete gallery"
						>
							<span class="material-symbols-outlined">delete</span>
						</button>
					</div>
					{#if isExpanded && subFolders.length > 0}
						<div class="subfolder-list" transition:slide={{ duration: 180, easing: cubicOut }}>
							{#each subFolders as sub}
								<div class="sub-item-wrapper">
									<a
										href="/gallery/{encodeURIComponent(gallery.name)}?folder={encodeURIComponent(
											sub.path
										)}"
										class="nav-item sub-item"
										class:active={page.params.name === gallery.name &&
											page.url.searchParams.get('folder') === sub.path}
									>
										<span class="material-symbols-outlined"
											>{page.params.name === gallery.name &&
											page.url.searchParams.get('folder') == sub.path
												? 'folder_open'
												: 'folder'}</span
										>
										<span class="sub-name">{sub.name}</span>
									</a>
									{#if galleries.length > 1}
										<button
											class="move-btn"
											onclick={() =>
												(moveFolder = { gallery: gallery.name, name: sub.name, path: sub.path })}
											title="move subgallery"
										>
											<span class="material-symbols-outlined">drive_file_move</span>
										</button>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				</div>
			{/each}


		</div>
		<div class="nav-bottom">
                        <hr />
			<a
				href="/gallery/global/duplicates"
				class="nav-item"
				class:active={page.url.pathname === '/gallery/global/duplicates'}
			>
				<span class="material-symbols-outlined">search</span>
				Duplicates
			</a>
                        <div class="settings-container up">
                                {#if showAddGallery}
                                        <AddGalleryDialog onClose={() => (showAddGallery = false)} />
                                {/if}
                                <button
                                        class="nav-item add-btn"
                                        class:active={showAddGallery}
                                        onclick={() => (showAddGallery = !showAddGallery)}
                                >
                                        <span class="material-symbols-outlined">add</span>
                                        Add Gallery
                                </button>
                        </div>
			<div class="settings-container up">
				{#if showSettings}
					<SettingsDialog onClose={() => (showSettings = false)} />
				{/if}

				<button
					class="nav-item"
					class:active={showSettings}
					onclick={() => (showSettings = !showSettings)}
				>
					<span class="material-symbols-outlined">settings</span>
					Settings
				</button>
			</div>
		</div>
	</nav>
</aside>

{#if moveFolder}
	<MoveFolderDialog
		gallery={moveFolder.gallery}
		folder={moveFolder}
		{galleries}
		onClose={() => (moveFolder = null)}
	/>
{/if}

<style>
	.sidebar-left {
		width: 350px;
		flex-shrink: 0;
		background-color: var(--bg);
		border-right: 1px solid var(--border);
		padding: 24px 16px;
		display: flex;
		flex-direction: column;
	}

	.logo-link {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 12px;
		text-decoration: none;
	}

	.logo-link h1 {
		color: var(--primary);
	}

	.nav-list {
		display: flex;
		flex-direction: column;
		flex-grow: 1;
		overflow: hidden;
		gap: 8px;
	}

	.nav-scroll {
		flex-grow: 1;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.nav-bottom {
		flex-shrink: 0;
		display: flex;
		flex-direction: column;
		margin-top: auto;
		gap: 8px;
	}

	.nav-item {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 8px 12px;
		color: var(--text-muted);
		text-decoration: none;
		border-radius: var(--radius-sm);
		cursor: pointer;
		border: none;
		font-family: inherit;
		font-size: 0.95rem;
		font-weight: 500;
		width: 95%;
		margin: 0 auto;
		text-align: left;
		position: relative;
		z-index: 1;
		background: transparent;
		transition:
			transform 0.16s cubic-bezier(0.2, 0, 0, 1),
			background 0.15s ease,
			color 0.15s ease;
	}

	.nav-item::before {
		content: '';
		position: absolute;
		left: 0;
		top: 18%;
		height: 64%;
		width: 3px;
		background: var(--primary);
		border-radius: 0 3px 3px 0;
		transform: scaleY(0);
		transform-origin: center;
		transition:
			transform 0.18s cubic-bezier(0.34, 1.56, 0.64, 1),
			background 0.15s ease;
	}

	.nav-item:hover {
		background: var(--bg-light);
		color: var(--primary);
		transform: translateX(4px);
	}

	.nav-item:hover::before {
		transform: scaleY(0.75);
		background: var(--primary);
	}

	.nav-item.active {
		color: var(--tertiary);
		background: color-mix(in srgb, var(--tertiary) 10%, transparent);
		font-weight: 600;
	}

	.nav-item.active::before {
		transform: scaleY(1);
		background: var(--tertiary);
		box-shadow: 0 0 8px color-mix(in srgb, var(--tertiary) 60%, transparent);
	}

	.add-btn {
		color: var(--text-muted);
		font-size: 0.9rem;
		padding: 6px 12px;
	}

	.settings-container {
		position: relative;
	}

        .settings-container.up :global(.settings-panel) {
                position: absolute;
                bottom: 100%;
                left: 0;
                margin-bottom: 8px;
                z-index: 10;
        }

        .settings-container.down :global(.settings-panel) {
                position: absolute;
                top: 100%;
                left: 0;
                margin-top: 8px;
                z-index: 10;
        }

	.gallery-item-wrapper {
		display: flex;
		align-items: center;
		position: relative;
		width: 95%;
		margin: 0 auto;
	}

	.gallery-item-wrapper .nav-item {
		width: 100%;
	}

	.delete-btn {
		position: absolute;
		right: 8px;
		background: transparent;
		border: none;
		color: var(--danger);
		cursor: pointer;
		opacity: 0;
		transition:
			opacity 0.2s,
			transform 0.2s;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 4px;
		border-radius: 4px;
		z-index: 2;
	}

	.gallery-item-wrapper:hover .delete-btn {
		opacity: 1;
	}

	.delete-btn:hover {
		background: color-mix(in srgb, var(--danger) 15%, transparent);
	}

	.gallery-block {
		display: flex;
		flex-direction: column;
		width: 100%;
	}

	.toggle-btn {
		background: transparent;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 2px;
		margin-right: 4px;
		border-radius: 4px;
		transition:
			color 0.2s,
			transform 0.2s;
		z-index: 2;
	}

	.toggle-btn:hover {
		color: var(--primary);
		background: var(--bg-light);
	}

	.chevron-icon {
		font-size: 1.2rem;
		transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.chevron-icon.rotated {
		transform: rotate(90deg);
	}

	.subfolder-list {
		display: flex;
		flex-direction: column;
		gap: 2px;
		margin-left: calc(5% + 14px);
		padding-left: 10px;
		border-left: 1px solid var(--border-subtle);
		margin-top: 4px;
		margin-bottom: 6px;
	}

	.sub-item-wrapper {
		display: flex;
		align-items: center;
		position: relative;
		width: 100%;
	}

	.move-btn {
		position: absolute;
		right: 6px;
		background: transparent;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		opacity: 0;
		transition:
			opacity 0.2s,
			color 0.2s;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 2px;
		border-radius: 4px;
		z-index: 2;
	}

	.move-btn .material-symbols-outlined {
		font-size: 1.1rem;
	}

	.sub-item-wrapper:hover .move-btn,
	.move-btn:focus-visible {
		opacity: 1;
	}

	.move-btn:hover {
		color: var(--primary);
		background: color-mix(in srgb, var(--primary) 15%, transparent);
	}

	.sub-item {
		font-size: 0.88rem;
		font-weight: 500;
		padding: 6px 10px;
		width: 100%;
		gap: 8px;
		border-radius: var(--radius-sm);
	}

	.sub-item .material-symbols-outlined {
		font-size: 1rem;
	}

	.sub-name {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
</style>
