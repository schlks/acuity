<script lang="ts">
	import { fly } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
	import { onMount } from 'svelte';
	import { toast } from '$lib/stores/toast.svelte';

	let { galleryName, folder = '', onClose, onSuccess = () => {} }: {
		galleryName: string;
		folder?: string;
		onClose: () => void;
		onSuccess?: () => void;
	} = $props();

	type SharpnessStats = { scored: number; pending: number };
	
	let stats = $state<SharpnessStats | null>(null);
	let percent = $state(10);
	let busy = $state(false);

	let estimate = $derived(stats ? Math.ceil(stats.scored * percent / 100) : 0);

	onMount(async () => {
		const params = new URLSearchParams({ folder });
		
		try {
			const res = await fetch(`/api/gallery/${encodeURIComponent(galleryName)}/sharpness?${params}`);
			if (!res.ok) {
				toast.error(await res.text());
				onClose();
				return;
			}
			stats = await res.json();
		} catch {
			onClose();
			toast.error("Failed to load sharpness stats");
		}
	});

	async function submit(e: Event) {
		e.preventDefault();

		const formData = new FormData();

		formData.append('percent', String(percent));
		formData.append('folder', folder);
		
		busy = true;

		try {
			const res = await fetch(`/api/gallery/${encodeURIComponent(galleryName)}/reject-blurry`, {
				method: 'POST',
				body: formData
			});

			if (res.ok) {
				const { rejected }: { rejected: number } = await res.json();
				onSuccess();
				if (rejected === 0) {
					toast.success("Nothing new to reject");
				} else {
					toast.success(`Rejected ${rejected} ${rejected === 1 ? 'Image' : 'Images'}`);
				}
				onClose();
			} else {
				console.error("Reject Action failed: ", await res.text());
				toast.error("Failed to reject Images");
			}
		} catch {
			toast.error("Failed to reject Images");
		} finally {
			busy = false;
		}
	}
</script>

<div class="settings-panel panel-glass blurry-dialog" transition:fly={{ y: 15, duration: 250, easing: quintOut }}>
	<h3>Reject blurry Images</h3>

	<form onsubmit={submit}>
		<div class="form-group">
			<label>
				Percent
				<select bind:value={percent}>
					<option value={5}>5 %</option>
					<option value={10}>10 %</option>
					<option value={20}>20 %</option>
				</select>
			</label>
			<span class="hint">
				{#if !stats}
					Loading…
				{:else if stats.scored === 0}
					No analyzed images yet
				{:else}
					Up to {estimate} of {stats.scored} images
				{/if}
			</span>
		</div>

		{#if stats && stats.pending > 0}
			<div class="pending" role="status">
				Analysis not finished: {stats.pending}
				{stats.pending === 1 ? 'image is' : 'images are'} not rated yet and will be skipped.
			</div>
		{/if}

		<div class="actions">
			<button type="button" class="btn btn-cancel" onclick={onClose}>Cancel</button>
			<button type="submit" class="btn btn-primary" disabled={busy || !stats || estimate === 0}>
				Reject
			</button>
		</div>
	</form>
</div>

<style>
	.settings-panel {
		position: absolute;
		top: 100%;
		left: 50%;
		transform: translateX(-50%);
		width: 280px;
		border-radius: var(--radius-md);
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

	.hint {
		font-size: 0.85rem;
		color: var(--text-muted);
	}

	.pending {
		padding: 10px 12px;
		border: 1px solid var(--danger);
		border-radius: var(--radius-sm);
		background: color-mix(in srgb, var(--danger) 20%, transparent);
		color: var(--text);
		font-size: 0.85rem;
		line-height: 1.4;
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 12px;
		margin-top: 24px;
	}

	.actions button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
		pointer-events: none;
	}
</style>
