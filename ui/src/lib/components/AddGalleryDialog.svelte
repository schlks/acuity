<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
	import { goto, invalidateAll } from '$app/navigation';

	let { onClose } = $props();

	let name = $state('');
	let path = $state('');
	let errorMsg = $state('');
	
	let popDirection = $state('up'); 
	let dialogContainer: HTMLElement;

	onMount(() => {
		if (dialogContainer && dialogContainer.parentElement) {
			const rect = dialogContainer.parentElement.getBoundingClientRect();
			if (rect.top > window.innerHeight / 2) {
				popDirection = 'up';
			} else {
				popDirection = 'down';
			}
		}
	});

	async function handleSelectDirectory() {
		if (typeof window !== 'undefined' && (window as any).go?.main?.App?.SelectDirectory) {
			try {
				const selected = await (window as any).go.main.App.SelectDirectory();
				if (selected) {
					path = selected;
					if (!name) {
						const parts = selected.split(/[/\\]/).filter(Boolean);
						name = parts[parts.length - 1] || '';
					}
				}
			} catch (err) {
				console.error("Directory selection error:", err);
			}
		}
	}

	async function handleImport(e: Event) {
		e.preventDefault();
		errorMsg = '';
		
		const formData = new FormData();
		formData.append('name', name);
		formData.append('path', path);

		const res = await fetch('/api/gallery', {
			method: 'POST',
			body: formData
		});

		if (!res.ok) {
			const text = await res.text();
			errorMsg = text || 'Failed to create gallery';
			return;
		}

		await invalidateAll();
		onClose();
		await goto(`/gallery/${encodeURIComponent(name)}`)
	}
</script>

<div 
	bind:this={dialogContainer} 
	class="settings-panel panel-glass {popDirection}" 
	transition:fly={{ y: popDirection === 'up' ? 15 : -15, duration: 250, easing: quintOut }}
>
	<h3>Add Gallery</h3>
	<form onsubmit={handleImport}>
		{#if errorMsg}
			<div class="error-msg">{errorMsg}</div>
		{/if}
		<div class="form-group">
			<label for="gallery-name-input">Name:</label>
			<input id="gallery-name-input" type="text" bind:value={name} placeholder="z. B. Vacation" required />
		</div>
		<div class="form-group">
			<label for="gallery-path-input">Path:</label>
			<div class="input-with-button">
				<input id="gallery-path-input" type="text" bind:value={path} placeholder="/home/user/images" required />
				<button type="button" class="btn-browse" onclick={handleSelectDirectory} title="Choose a folder">
					<span class="material-symbols-outlined">folder_open</span>
				</button>
			</div>
		</div>

		<button type="submit" style="display: none;"></button>
		
		<div class="actions">
			<button type="button" class="btn btn-cancel" onclick={onClose}>Cancel</button>
			<button type="submit" class="btn btn-primary">Import</button>
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

	.error-msg {
		background: var(--bg);
		color: var(--danger);
		padding: 8px 12px;
		border-radius: var(--radius-sm);
		font-size: 0.9rem;
		margin-bottom: 16px;
		border: 1px solid var(--danger);
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

	.input-with-button {
		display: flex;
		gap: 8px;
	}

	.input-with-button input {
		flex: 1;
	}

	.btn-browse {
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--bg);
		border: 1px solid var(--bg);
		color: var(--primary);
		padding: 8px 12px;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: border-color var(--duration-base) ease, background-color var(--duration-base) ease;
	}

	.btn-browse:hover {
		border-color: var(--primary);
	}

	.btn-browse .material-symbols-outlined {
		font-size: 20px;
	}

	input[type="text"] {
		background-color: var(--bg);
		color: var(--text);
		border: 1px solid var(--bg);
		padding: 10px 12px;
		border-radius: var(--radius-md);
		font-family: inherit;
		font-size: 0.95rem;
		outline: none;
		transition: border-color var(--duration-base) ease;
	}

	input[type="text"]:focus {
		border-color: var(--primary);
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 12px;
		margin-top: 24px;
	}
</style>
