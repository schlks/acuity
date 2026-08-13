<script lang="ts">
	import { toast } from '$lib/stores/toast.svelte';
	import { fly, fade } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
</script>

<div class="toast-container">
	{#each toast.items as item (item.id)}
		<div
			class="toast {item.type}"
			in:fly={{ y: 20, duration: 250, easing: quintOut }}
			out:fade={{ duration: 150 }}
		>
			<span class="material-symbols-outlined icon">
				{#if item.type === 'success'}
					check_circle
				{:else if item.type === 'danger'}
					error
				{:else}
					info
				{/if}
			</span>
			<span class="message">{item.message}</span>
			<button class="close-btn" onclick={() => toast.dismiss(item.id)} title="Dismiss">
				<span class="material-symbols-outlined">close</span>
			</button>
		</div>
	{/each}
</div>

<style>
	.toast-container {
		position: fixed;
		bottom: 24px;
		right: 24px;
		z-index: 100000;
		display: flex;
		flex-direction: column;
		gap: 8px;
		pointer-events: none;
		max-width: 400px;
	}

	.toast {
		pointer-events: auto;
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px 16px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--bg) 92%, black);
		backdrop-filter: blur(16px);
		-webkit-backdrop-filter: blur(16px);
		border: 1px solid color-mix(in srgb, var(--secondary) 40%, var(--bg-light));
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.6);
		color: var(--text);
		font-size: 0.88rem;
		font-weight: 500;
		user-select: none;
	}

	.toast.success .icon {
		color: var(--success);
	}

	.toast.info .icon {
		color: var(--info);
	}

	.toast.danger .icon {
		color: var(--danger);
	}

	.icon {
		font-size: 1.25rem;
		flex-shrink: 0;
	}

	.message {
		flex: 1;
		line-height: 1.4;
	}

	.close-btn {
		background: transparent;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		padding: 2px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 4px;
		transition: all 0.12s ease;
		flex-shrink: 0;
	}

	.close-btn .material-symbols-outlined {
		font-size: 1.1rem;
	}

	.close-btn:hover {
		background: var(--bg-light);
		color: var(--primary);
	}
</style>
