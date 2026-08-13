<script lang="ts">
	import { modal } from '$lib/stores/modal.svelte';
	import { fade, scale } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';

	function handleKeydown(e: KeyboardEvent) {
		if (!modal.isOpen) return;
		if (e.key === 'Escape') {
			e.preventDefault();
			modal.handleCancel();
		} else if (e.key === 'Enter') {
			e.preventDefault();
			modal.handleConfirm();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if modal.isOpen}
	<div class="modal-backdrop" transition:fade={{ duration: 150 }} onclick={() => modal.handleCancel()}>
		<div
			class="modal-card"
			transition:scale={{ duration: 200, start: 0.95, easing: quintOut }}
			onclick={(e) => e.stopPropagation()}
			tabindex="-1"
		>
			<div class="modal-header">
				<div class="title-row">
					{#if modal.danger}
						<span class="material-symbols-outlined danger-icon">warning</span>
					{:else}
						<span class="material-symbols-outlined info-icon">help</span>
					{/if}
					<h3>{modal.title}</h3>
				</div>
			</div>

			<p class="modal-message">{modal.message}</p>

			<div class="modal-actions">
				<button type="button" class="btn-cancel" onclick={() => modal.handleCancel()}>
					{modal.cancelText}
				</button>
				<button
					type="button"
					class={modal.danger ? 'btn-danger' : 'btn-primary'}
					onclick={() => modal.handleConfirm()}
					autofocus
				>
					{modal.confirmText}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		z-index: 100000;
		background: rgba(0, 0, 0, 0.7);
		backdrop-filter: blur(8px);
		-webkit-backdrop-filter: blur(8px);
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 16px;
	}

	.modal-card {
		width: 100%;
		max-width: 440px;
		background: color-mix(in srgb, var(--bg) 92%, black);
		border: 1px solid color-mix(in srgb, var(--secondary) 40%, var(--bg-light));
		border-radius: 12px;
		box-shadow: 0 20px 40px rgba(0, 0, 0, 0.7);
		padding: 24px;
		display: flex;
		flex-direction: column;
		gap: 16px;
		user-select: none;
	}

	.modal-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
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

	.danger-icon {
		color: var(--danger);
		font-size: 1.4rem;
	}

	.info-icon {
		color: var(--info);
		font-size: 1.4rem;
	}

	.modal-message {
		color: var(--text-muted);
		font-size: 0.92rem;
		line-height: 1.5;
		margin: 0;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 10px;
		margin-top: 8px;
	}

	button {
		padding: 8px 18px;
		border-radius: 6px;
		font-size: 0.88rem;
		font-weight: 600;
		cursor: pointer;
		border: none;
		transition: all 0.12s ease;
	}

	.btn-cancel {
		background: transparent;
		color: var(--text-muted);
		border: 1px solid var(--bg-light);
	}

	.btn-cancel:hover {
		background: var(--bg-light);
		color: var(--primary);
		border-color: var(--primary);
	}

	.btn-primary {
		background: var(--primary);
		color: white;
	}

	.btn-primary:hover {
		filter: brightness(1.1);
	}

	.btn-danger {
		background: var(--danger);
		color: white;
	}

	.btn-danger:hover {
		filter: brightness(1.15);
	}
</style>
