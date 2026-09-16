
<script lang="ts">
	import { fly } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
	import { page } from '$app/state';

let { onClose, selectedImages = [], onSuccess = () => {} } = $props();

	let criteria = $state("selected");
	let action = $state("copy");
	let targetGallery = $state('');

	async function executeBatch(e: SubmitEvent | Event) {
		e.preventDefault();

		const formData = new FormData();

		formData.append('criteria', criteria);
		formData.append('batch_action', action);
		formData.append('source_gallery', page.params.name || '');

		if (action === 'move' || action === 'copy') {
			formData.append('name', targetGallery)
		} 

		if (criteria === 'selected') {
			const ids = selectedImages.map(img => img.id ?? img.ID).join(',');
			formData.append('image_ids', ids);
		}

		const res = await fetch('/api/gallery/batch', {
			method: 'POST',
			body: formData
		});
		
		if (res.ok) {
			const ids = selectedImages.map((img: any) => img.id ?? img.ID);
			onSuccess?.(action, criteria, ids);
			onClose();
		} else {
			console.error("Batch Action failed:", await res.text());
		}
	}
</script>

<div class="settings-panel" transition:fly={{ y: 15, duration: 250, easing: quintOut }}>
	<h3>Batch Actions</h3>

	<form onsubmit={executeBatch}>
		<div class="form-group">
			<label>
				Criteria:
				<select class:changed={criteria !== "selected"} bind:value={criteria}>
					<option value="selected">Selected</option>
					<option value="flag_any">Any Flag</option>
					<option value="flag_keep">Kept Flag</option>
					<option value="flag_reject">Rejected Flag</option>
				</select>
			</label>
		</div>

		<div class="form-group">
			<label>
				Action:
				<select class:changed={action !== "copy"} bind:value={action}>
					<option value="delete">Delete</option>
					<option value="copy">Copy</option>
					<option value="move">Move</option>
				</select>
			</label>
		</div>

		{#if action !== "delete"}
			<div class="form-group">
				<label>
					Target Gallery:
					<select bind:value={targetGallery}>
						{#each page.data.galleries as gallery}
							<option value={gallery.name}>{gallery.name}</option>
						{/each}
					</select>
				</label>
			</div>
		{/if}

		<button title="submit" type="submit" style="display: none;"></button>
		
		<div class="actions">
			<button type="button" class="btn-cancel" onclick={onClose}>Cancel</button>
			<button type="submit" class="btn-primary">Execute</button>
		</div>
	</form>
</div>

<style>
	.settings-panel {
		position: absolute;
		top: 100%;
		left: 50%;
		transform: translateX(-50%);
		width: max-content;
		background: var(--bg-light);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 16px;
		margin-top: 8px;
		z-index: 100;
	}
	
	.settings-panel h3 {
		margin-top: 0;
		margin-bottom: 16px;
		color: var(--text);
		font-size: 1.1rem;
		font-weight: 600;
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
		border-radius: 8px;
		font-family: inherit;
		font-size: 0.95rem;
		outline: none;
		cursor: pointer;
		transition: border-color 0.2s;
	}

	select:focus {
		border-color: var(--primary);
	}

	select.changed {
		border-color: var(--tertiary);
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 12px;
		margin-top: 24px;
	}

	button {
		position: relative;
		z-index: 1;
		font-family: inherit;
		font-size: 0.95rem;
		border-radius: 8px;
		padding: 10px 18px;
		cursor: pointer;
		border: none;
		background: transparent;
		transition: color 0.3s;
	}

	button::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		border-radius: 0;
		z-index: -1;
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
	}

	button:hover::before {
		height: 100%;
		border-radius: 8px;
	}

	.btn-cancel {
		color: var(--text-muted);
	}
	
	.btn-cancel::before {
		background: var(--text-muted);
	}

	.btn-cancel:hover {
		color: var(--bg-dark);
	}

	.btn-danger {
		color: var(--text-muted);
	}

	.btn-danger::before {
		background: var(--danger);
	}

	.btn-danger:hover {
		color: var(--bg-dark);
	}

	.btn-primary {
		color: var(--text);
	}

	.btn-primary::before {
		background: var(--primary);
	}

	.btn-primary:hover {
		color: var(--bg-dark);
	}

	.btn-toggle {
		color: var(--text);
	}

	.btn-toggle::before {
		background: var(--primary);
	}

	.btn-toggle:hover {
		color: var(--bg-dark);
	}

	.btn-toggle.active {
		color: var(--text);
	}

	.btn-toggle.active::before {
		background: var(--tertiary);
	}

	.btn-toggle.active:hover {
		color: var(--bg-dark);
	}
</style>
