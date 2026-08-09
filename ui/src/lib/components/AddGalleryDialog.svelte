<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
	import { invalidateAll } from '$app/navigation';

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

	async function handleImport(e) {
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
			errorMsg = text || 'Failed to craate gallery';
			return;
		}

		await invalidateAll();
		onClose();
	}
</script>

<div 
	bind:this={dialogContainer} 
	class="settings-panel {popDirection}" 
	transition:fly={{ y: popDirection === 'up' ? 15 : -15, duration: 250, easing: quintOut }}
>
	<h3>Add Gallery</h3>
	<form onsubmit={handleImport}>
		{#if errorMsg}
			<div class="error-msg">{errorMsg}</div>
		{/if}
		<div class="form-group">
			<label>
				Name: 
				<input type="text" bind:value={name} placeholder="z. B. Vacation" required />
			</label>
		</div>
		<div class="form-group">
			<label>
				Path: 
				<input type="text" bind:value={path} placeholder="/home/user/images" required />
			</label>
		</div>

		<button type="submit" style="display: none;"></button>
		
		<div class="actions">
			<button type="button" class="btn-cancel" onclick={onClose}>Cancel</button>
			<button type="submit" class="btn-primary">Import</button>
		</div>
	</form>
</div>

<style>
	.settings-panel {
		position: absolute;
		left: 0;
		width: 100%;
		background: var(--bg-light);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 16px;
		z-index: 10;
	}
	
	.settings-panel.up {
		bottom: 100%;
		margin-bottom: 16px;
	}

	.settings-panel.down {
		top: 100%;
		margin-top: 16px;
	}
	
	.settings-panel h3 {
		margin-top: 0;
		margin-bottom: 16px;
		color: var(--text);
		font-size: 1.1rem;
		font-weight: 600;
	}

	.error-msg {
		background: rgba(255, 60, 60, 0.1);
		color: #ff5555;
		padding: 8px 12px;
		border-radius: 6px;
		font-size: 0.9rem;
		margin-bottom: 16px;
		border: 1px solid rgba(255, 60, 60, 0.2);
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
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	input[type="text"] {
		background-color: var(--bg);
		color: var(--text);
		border: 1px solid var(--bg);
		padding: 10px 12px;
		border-radius: 8px;
		font-family: inherit;
		font-size: 0.95rem;
		outline: none;
		transition: border-color 0.2s;
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

	.btn-primary {
		color: var(--text);
	}

	.btn-primary::before {
		background: var(--primary);
	}

	.btn-primary:hover {
		color: var(--bg-dark);
	}
</style>
