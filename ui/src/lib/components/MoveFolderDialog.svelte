<script lang="ts">
	import { page } from '$app/state';
	import { goto, invalidateAll } from '$app/navigation';
	import { fade, scale } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
	import { toast } from '$lib/stores/toast.svelte';

	let { gallery, folder, galleries, onClose } = $props();

	let targets = $derived(galleries.filter((g: any) => g.name !== gallery));
	let target = $state('');
	let moving = $state(false);
	let errorMsg = $state('');

	$effect(() => {
		if (!target && targets.length > 0) target = targets[0].name;
	});

	function close() {
		if (!moving) onClose();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			close();
		}
	}

	function relocatedFolder(current: string, newPath: string): string | null {
		if (current === folder.path) return newPath;
		if (current.startsWith(folder.path + '/') || current.startsWith(folder.path + '\\')) {
			return newPath + current.slice(folder.path.length);
		}
		return null;
	}

	async function handleMove(e: Event) {
		e.preventDefault();
		if (moving || !target) return;
		moving = true;
		errorMsg = '';

		try {
			const formData = new FormData();
			formData.append('folder', folder.path);
			formData.append('target_gallery', target);

			const res = await fetch(`/api/gallery/${encodeURIComponent(gallery)}/folder/move`, {
				method: 'POST',
				body: formData
			});

			if (!res.ok) {
				errorMsg = (await res.text()).trim() || 'Failed to move folder';
				return;
			}

			const data = await res.json();
			const viewedGallery = page.params.name ? decodeURIComponent(page.params.name) : '';
			const viewedFolder = page.url.searchParams.get('folder') || '';
			const relocated =
				viewedGallery === gallery && viewedFolder ? relocatedFolder(viewedFolder, data.path) : null;

			toast.success(`Moved "${folder.name}" to ${target}`);
			onClose();
			if (relocated) {
				await goto(`/gallery/${encodeURIComponent(target)}?folder=${encodeURIComponent(relocated)}`, {
					replaceState: true,
					invalidateAll: true
				});
			} else {
				await invalidateAll();
			}
		} catch (err) {
			console.error('Move folder error:', err);
			errorMsg = 'Failed to move folder';
		} finally {
			moving = false;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div
	class="modal-backdrop overlay"
	role="presentation"
	transition:fade={{ duration: 150 }}
	onclick={close}
>
	<div
		class="modal-card panel-glass"
		role="dialog"
		aria-modal="true"
		tabindex="-1"
		transition:scale={{ duration: 200, start: 0.95, easing: quintOut }}
		onclick={(e) => e.stopPropagation()}
	>
		<div class="title-row">
			<span class="material-symbols-outlined info-icon">drive_file_move</span>
			<h3>Move Subgallery</h3>
		</div>

		{#if targets.length === 0}
			<p class="modal-message">There is no other gallery to move "{folder.name}" to.</p>
			<div class="actions">
				<button type="button" class="btn btn-cancel" onclick={close}>Close</button>
			</div>
		{:else}
			<form onsubmit={handleMove}>
				<p class="modal-message">
					Move "{folder.name}" with all its images from {gallery} into the root of another gallery.
				</p>

				{#if errorMsg}
					<div class="error-msg">{errorMsg}</div>
				{/if}

				<div class="form-group">
					<label for="move-target-select">Target Gallery:</label>
					<select id="move-target-select" bind:value={target} disabled={moving}>
						{#each targets as g}
							<option value={g.name}>{g.name}</option>
						{/each}
					</select>
				</div>

				{#if moving}
					<p class="modal-hint">
						Moving… this can take a while if the galleries are on different drives.
					</p>
				{/if}

				<div class="actions">
					<button type="button" class="btn btn-cancel" onclick={close} disabled={moving}>
						Cancel
					</button>
					<button type="submit" class="btn btn-primary" disabled={moving || !target}>
						{moving ? 'Moving…' : 'Move'}
					</button>
				</div>
			</form>
		{/if}
	</div>
</div>

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		z-index: 100000;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 16px;
	}

	.modal-card {
		width: 100%;
		max-width: 440px;
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-lg);
		padding: 24px;
		display: flex;
		flex-direction: column;
		gap: 16px;
		user-select: none;
	}

	.title-row {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.title-row h3 {
		font-size: 1.1rem;
		color: var(--text);
		margin: 0;
	}

	.info-icon {
		color: var(--info);
		font-size: 1.4rem;
	}

	.modal-message {
		color: var(--text-muted);
		font-size: 0.92rem;
		line-height: 1.5;
		margin: 0 0 16px;
	}

	.modal-hint {
		color: var(--text-muted);
		font-size: 0.85rem;
		margin: 0;
	}

	.error-msg {
		background: var(--bg);
		color: var(--danger);
		padding: 8px 12px;
		border-radius: var(--radius-sm);
		font-size: 0.9rem;
		margin-bottom: 16px;
		border: 1px solid var(--danger);
		user-select: text;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin-bottom: 16px;
	}

	.form-group label {
		font-size: 0.9rem;
		color: var(--text-muted);
	}

	select {
		background-color: var(--bg);
		color: var(--text);
		border: 1px solid var(--bg);
		padding: 10px 12px;
		border-radius: var(--radius-md);
		font-family: inherit;
		font-size: 0.95rem;
		outline: none;
		cursor: pointer;
		transition: border-color var(--duration-base) ease;
	}

	select:focus {
		border-color: var(--primary);
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 10px;
		margin-top: 8px;
	}
</style>
