<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
	import { invalidateAll } from '$app/navigation';

	let { onClose } = $props();

	let config = $state({
		default_sort_by: 'name',
		default_sort_order: 'asc',
		grid_size: 'medium',
		images_per_page: 100,
		infinite_scroll: false
	});
	let originalConfig = $state({...config});

	onMount(async () => {
		const res = await fetch('/api/settings', { cache: 'no-store' });
		if (res.ok) {
			const data = await res.json();
			config = data;
			originalConfig = structuredClone(data);
		}
	});

	async function saveSettings(e) {
		e.preventDefault();
		await fetch('/api/settings', {
			method: 'POST',
			body: JSON.stringify(config)
		});
		await invalidateAll();

		onClose();
	}

	async function wipeDatabases() {
		if (confirm("Do you want to delete all databases irreversably?")) {
			await fetch('/api/reset', { method: 'POST' });
		}
	}
</script>

<div class="settings-panel" transition:fly={{ y: 15, duration: 250, easing: quintOut }}>
	<h3>Settings</h3>

	<form onsubmit={saveSettings}>
		<div class="form-group">
			<label>
				Sort By:
				<select class:changed={config.default_sort_by !== originalConfig.default_sort_by} bind:value={config.default_sort_by}>
					<option value="rating">Rating</option>
					<option value="date">Date</option>
					<option value="name">Name</option>
					<option value="name_lex">Alphabet</option>
					<option value="resolution">Resolution</option>
					<option value="aspect_ratio">Aspect Ratio</option>
				</select>
			</label>
		</div>

		<div class="form-group">
			<label>
				Sort Order:
				<select class:changed={config.default_sort_order !== originalConfig.default_sort_order} bind:value={config.default_sort_order}>
					<option value="asc">Ascending</option>
					<option value="desc">Descending</option>
				</select>
			</label>
		</div>

		<div class="form-group">
			<label>
				Images on Page:
				<input
					type="number"
					min="50"
					max="1000"
					step="50"
					bind:value={config.images_per_page}
					class:changed={config.images_per_page !== originalConfig.images_per_page}
				/>
			</label>
		</div>

		<div class="form-group row-layout">
			<span class="label-text">Scrolling:</span>
			<button 
				type="button"
				class="btn-toggle"
				class:active={config.infinite_scroll}
				onclick={() => config.infinite_scroll = !config.infinite_scroll}
			>
				{config.infinite_scroll ? 'Infinite' : 'Paged'}
			</button>
		</div>

		<div class="form-group row-layout">
			<span class="label-text">Wipe Databases:</span>
			<button type="button" class="btn-danger" onclick={wipeDatabases}>Delete</button>
		</div>

		<button type="submit" style="display: none;"></button>
		
		<div class="actions">
			<button type="button" class="btn-cancel" onclick={onClose}>Cancel</button>
			<button type="submit" class="btn-primary">Save</button>
		</div>
	</form>
</div>

<style>
	.settings-panel {
		position: absolute;
		bottom: 100%;
		left: 0;
		width: 100%;
		background: var(--bg-light);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 16px;
		margin-bottom: 8px;
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

	.form-group.row-layout {
		flex-direction: row;
		align-items: center;
		justify-content: space-between;
	}

	.label-text {
		font-size: 0.9rem;
		color: var(--text-muted);
	}

	select, input[type="number"] {
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

	select:focus, input[type="number"]:focus {
		border-color: var(--primary);
	}

	select.changed, input[type="number"].changed {
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
		border-radius: 0px;
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
