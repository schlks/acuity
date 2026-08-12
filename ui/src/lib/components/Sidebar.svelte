<script lang="ts">
	import { page } from '$app/state';
	import { goto, invalidateAll } from '$app/navigation';
	import SettingsDialog from "./SettingsDialog.svelte";
	import AddGalleryDialog from "./AddGalleryDialog.svelte";
	
	let { galleries } = $props();
	let showSettings = $state(false);
	let showAddGallery = $state(false);
    let expandedGalleries = $state<Record<string, boolean>>({});


	async function deleteGallery(name: string) {
		if (!confirm(`Do you really want to delete the "${name}"`)) return;

		try {
			const res = await fetch(`/api/gallery/${encodeURIComponent(name)}`, {
				method: 'DELETE'
			});
			if (res.ok) {
				await invalidateAll();
				if (page.params.name === name) {
					await goto('/');
				}
			} else {
				alert("Failed to delete Gallery")
			}
		} catch (error) {
			console.error(error);
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
		<img src=/logo.svg alt="Acuity Logo" width="32" height="32" />
		<h1>Acuity</h1>
	</a>

	<hr />

	<nav class="nav-list">
		<div class="nav-scroll">
			{#each galleries as gallery}
                {@const subFolders = gallery.sub_folders || []}
                {@const isExpanded = expandedGalleries[gallery.name] ?? (page.params.name === gallery.name)}
                <div class="gallery-block">
                    <div class="gallery-item-wrapper">
                        {#if subFolders.length > 0}
                            <button
                                class="toggle-btn"
                                onclick="{() => expandedGalleries[gallery.name] = !isExpanded}"
                                title="Toggle Subfolders">
                                <span class="material-symbols-outlined">{isExpanded ? 'expand_more' : 'chevron_right'}</span>
                            </button>
                        {/if}
                        <a
                            href="/gallery/{encodeURIComponent(gallery.name)}"
                            class="nav-item"
                            class:active={page.params.name === gallery.name}
                        >
                            <span class="material-symbols-outlined">folder</span>
                            {gallery.name}
                        </a>
                        <button class="delete-btn" onclick={() => deleteGallery(gallery.name)} title="delete gallery">
                            <span class="material-symbols-outlined">delete</span>
                        </button>
                    </div>
                    {#if isExpanded && subFolders.length > 0}
                        <div class="subfolder-list">
                            {#each subFolders as sub}
                                <a
                                    href="/gallery/{encodeURIComponent(gallery.name)}?folder={encodeURIComponent(sub.path)}"
                                    class="nav-item sub-item"
                                    class:active={page.params.name === gallery.name && page.url.searchParams.get('folder') === sub.path}
                                >
                                    <span class="material-symbols-outlined">folder_open</span>
                                    <span class="sub-name">{sub.name}</span>
                                </a>
                            {/each}
                        </div>
                    {/if}
                </div>
                {/each}

            <hr />

            <div class="settings-container">
                {#if showAddGallery}
                    <AddGalleryDialog onClose={() => showAddGallery = false} />
                {/if}
                <button class="nav-item add-btn" class:active={showAddGallery} onclick={() => showAddGallery = !showAddGallery}>
                    <span class="material-symbols-outlined">add</span>
                    Add Gallery
                </button>
            </div>
		</div>
		<div class="nav-bottom">
			<a href="/gallery/global/duplicates" class="nav-item" class:active={page.url.pathname === '/gallery/global/duplicates'}>
				<span class="material-symbols-outlined">search</span>
				Duplicates
			</a>
			<hr />
			<div class="settings-container">
				{#if showSettings}
					<SettingsDialog onClose={() => showSettings = false} />
				{/if}

				<button class="nav-item" class:active={showSettings} onclick={() => showSettings = !showSettings}>
					<span class="material-symbols-outlined">settings</span>
					Settings
				</button>
			</div>
		</div>
	</nav>
</aside>

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
		padding: 10px 24px 10px 12px;
		color: var(--text-muted);
		text-decoration: none;
		border-radius: 8px;
		cursor: pointer;
		border: none;
		font-family: inherit;
		font-size: 1.1rem;
		font-weight: 600;
		width: 90%;
		margin: 0 auto;
		text-align: left;
		position: relative;
		z-index: 1;
		background: transparent;
		transition: color 0.2s;
	}

	.nav-item::before {
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

	.nav-item:hover {
		color: var(--text);
	}

	.nav-item:hover::before {
		height: 100%;
		border-radius: 8px;
		background: var(--primary);
	}

	.nav-item.active {
		color: var(--text);
	}

	.nav-item.active::before {
		height: 2px;
		border-radius: 0;
		background: var(--tertiary);
	}
	
	.nav-item.active:hover {
		color: var(--bg-dark);
	}
	
	.nav-item.active:hover::before {
		height: 100%;
		border-radius: 8px;
		background: var(--tertiary);
	}

	.add-btn {
		color: var(--text-muted);
		font-size: 0.9rem;
		padding: 6px 12px;
	}

	.settings-container {
		position: relative;
	}

	.gallery-item-wrapper {
        display: flex;
        align-items: center;
        position: relative;
        width: 90%;
        margin: 0 auto;
    }

    .gallery-item-wrapper .nav-item {
        width: 100%; /* Link nimmt den vollen Platz ein */
    }

    .delete-btn {
        position: absolute;
        right: 8px;
        background: transparent;
        border: none;
        color: var(--danger);
        cursor: pointer;
        opacity: 0;
        transition: opacity 0.2s, transform 0.2s;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 4px;
        border-radius: 4px;
        z-index: 2; /* Über dem Link */
    }

    .gallery-item-wrapper:hover .delete-btn {
        opacity: 1;
    }

    .delete-btn:hover {
        background: rgba(255, 0, 0, 0.1);
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
        margin-right: -4px;
        border-radius: 4px;
        transition: color 0.2s, transform 0.2s;
        z-index: 2;
    }

    .toggle-btn:hover {
        color: var(--text);
        background: rgba(255, 255, 255, 0.05);
    }

    .toggle-btn .material-symbols-outlined {
        font-size: 1.2rem;
    }

    .subfolder-list {
        display: flex;
        flex-direction: column;
        gap: 4px;
        margin-left: calc(5% + 18px);
        padding-left: 10px;
        border-left: 2px solid var(--border);
        margin-top: 4px;
        margin-bottom: 4px;
    }

    .sub-item {
        font-size: 0.95rem;
        font-weight: 500;
        padding: 6px 12px;
        width: 100%;
        gap: 8px;
    }

    .sub-item .material-symbols-outlined {
        font-size: 1.1rem;
    }

    .sub-name {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }
</style>
