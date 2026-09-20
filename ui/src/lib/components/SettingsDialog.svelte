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

	async function saveSettings(e: SubmitEvent | Event) {
		e.preventDefault();
        config.images_per_page = Number(config.images_per_page) || 100;
		await fetch('/api/settings', {
			method: 'POST',
			body: JSON.stringify(config)
		});
		await invalidateAll();

		onClose();
	}

	async function wipeDatabases() {
		if (confirm("Do you want to delete all databases irreversibly?")) {
			await fetch('/api/reset', { method: 'POST' });
		}
	}
</script>

<div class="settings-panel panel-glass" transition:fly={{ y: 15, duration: 250, easing: quintOut }}>
	<h3>Settings</h3>

	<form onsubmit={saveSettings}>
		<div class="form-group">
			<label for="sort-by">Sort By:</label>
			<select id="sort-by" class:changed={config.default_sort_by !== originalConfig.default_sort_by} bind:value={config.default_sort_by}>
				<option value="rating">Rating</option>
				<option value="date">Date</option>
				<option value="name">Name</option>
				<option value="name_lex">Alphabet</option>
				<option value="resolution">Resolution</option>
				<option value="aspect_ratio">Aspect Ratio</option>
			</select>
		</div>

		<div class="form-group">
			<label for="sort-order">Sort Order:</label>
			<select id="sort-order" class:changed={config.default_sort_order !== originalConfig.default_sort_order} bind:value={config.default_sort_order}>
				<option value="asc">Ascending</option>
				<option value="desc">Descending</option>
			</select>
		</div>

		<div class="form-group">
			<label for="images-per-page">Images on Page:</label>
			<input
				id="images-per-page"
				type="number"
				min="1"
				max="1000"
				step="any"
				bind:value={config.images_per_page}
				class:changed={config.images_per_page !== originalConfig.images_per_page}
			/>
		</div>

		<div class="form-group">
			<label for="scrolling-toggle">Scrolling:</label>
			<button
				id="scrolling-toggle"
				type="button"
				class="btn btn-toggle"
				class:active={config.infinite_scroll}
				onclick={() => config.infinite_scroll = !config.infinite_scroll}
			>
				{config.infinite_scroll ? 'Infinite' : 'Paged'}
			</button>
		</div>

		<div class="form-group">
			<label for="wipe-btn">Wipe Databases:</label>
			<button id="wipe-btn" type="button" class="btn btn-danger" onclick={wipeDatabases}>Delete</button>
		</div>

		<button type="submit" style="display: none;"></button>
		
		<div class="actions">
			<button type="button" class="btn btn-cancel" onclick={onClose}>Cancel</button>
			<button type="submit" class="btn btn-primary">Save</button>
		</div>
	</form>
</div>

<style>
	.settings-panel {
		width: 100%;
		border-radius: var(--radius-md);
		padding: 16px;
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
		flex-direction: row;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		margin-bottom: 16px;
	}

	.form-group label {
		font-size: 0.9rem;
		color: var(--text-muted);
	}

	select, input[type="number"] {
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
		width: 180px;
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
</style>
