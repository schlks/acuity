<script lang="ts">
	import { onMount } from 'svelte';
	import { fade, scale } from 'svelte/transition';

	interface AIStatus {
		ready: boolean;
		downloading: boolean;
		current_task: string;
		current_bytes: number;
		total_bytes: number;
		percent: number;
		step: number;
		total_steps: number;
		error?: string;
	}

	let status = $state<AIStatus | null>(null);
	let isVisible = $derived(status !== null && status.downloading && !status.ready);
	let pollInterval: ReturnType<typeof setInterval> | null = null;

	function formatBytes(bytes: number): string {
		if (!bytes || bytes <= 0) return '0 MB';
		const mb = bytes / (1024 * 1024);
		return `${mb.toFixed(1)} MB`;
	}

	async function checkStatus() {
		try {
			const res = await fetch('/api/ai/status');
			if (res.ok) {
				const data = await res.json();
				status = data;
				if (data.ready && pollInterval) {
					clearInterval(pollInterval);
					pollInterval = null;
				}
			}
		} catch (e) {
			console.error('Failed to fetch AI status:', e);
		}
	}

	onMount(() => {
		checkStatus();
		pollInterval = setInterval(checkStatus, 500);
		return () => {
			if (pollInterval) clearInterval(pollInterval);
		};
	});
</script>

{#if isVisible && status}
	<div class="modal-backdrop overlay" transition:fade={{ duration: 150 }}>
		<div class="setup-card" transition:scale={{ start: 0.95, duration: 200 }}>
			<div class="icon-header">
                <div class="icon-wrapper">
                    <span class="material-symbols-outlined base-icon">download</span>
                    <span class="material-symbols-outlined active-icon">download</span>
                </div>
			</div>

			<div class="card-text">
				<h2>Setting up Local AI Search</h2>
				<p class="subtitle">Downloading neural models for visual & semantic search. This only happens once.</p>
			</div>

			<div class="steps-list">
				<div class="step-item" class:active={status.step === 1} class:done={status.step > 1}>
					<span class="material-symbols-outlined step-icon">
						{status.step > 1 ? 'check_circle' : status.step === 1 ? 'sync' : 'radio_button_unchecked'}
					</span>
					<div class="step-info">
						<span class="step-name">ONNX Runtime Engine</span>
						<span class="step-desc">{status.step > 1 ? 'Installed' : status.step === 1 ? 'Extracting binary...' : 'Pending'}</span>
					</div>
				</div>

				<div class="step-item" class:active={status.step === 2} class:done={status.step > 2}>
					<span class="material-symbols-outlined step-icon">
						{status.step > 2 ? 'check_circle' : status.step === 2 ? 'sync' : 'radio_button_unchecked'}
					</span>
					<div class="step-info">
						<span class="step-name">CLIP Vision Model</span>
						<span class="step-desc">
							{status.step > 2 ? 'Downloaded' : status.step === 2 ? `${formatBytes(status.current_bytes)} / ${formatBytes(status.total_bytes)}` : '~150 MB'}
						</span>
					</div>
				</div>

				<div class="step-item" class:active={status.step === 3} class:done={status.ready}>
					<span class="material-symbols-outlined step-icon">
						{status.ready ? 'check_circle' : status.step === 3 ? 'sync' : 'radio_button_unchecked'}
					</span>
					<div class="step-info">
						<span class="step-name">CLIP Text Model</span>
						<span class="step-desc">
							{status.ready ? 'Downloaded' : status.step === 3 ? `${formatBytes(status.current_bytes)} / ${formatBytes(status.total_bytes)}` : '~150 MB'}
						</span>
					</div>
				</div>
			</div>

			<div class="progress-section">
				<div class="progress-labels">
					<span class="current-task">{status.current_task || 'Downloading...'}</span>
					<span class="percent-label">{status.percent ? status.percent.toFixed(0) : 0}%</span>
				</div>
				<div class="progress-bar-bg">
					<div class="progress-bar-fill" style="width: {status.percent || 0}%;"></div>
				</div>
			</div>

			{#if status.error}
				<div class="error-banner">
					<span class="material-symbols-outlined">error</span>
					<span>{status.error}</span>
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		z-index: 100000;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 24px;
	}

	.setup-card {
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: var(--radius-xl);
		width: 100%;
		max-width: 520px;
		padding: 36px 32px;
		display: flex;
		flex-direction: column;
		gap: 24px;
		box-shadow: var(--shadow-lg);
		text-align: center;
	}

	.icon-header {
		display: flex;
		align-items: center;
		justify-content: center;
	}

    .icon-wrapper {
        position: relative;
        display: inline-flex;
    }

    .base-icon {
        font-size: 3.6rem;
        color: var(--bg-dark);
    }

	.active-icon {
                position: absolute;
                top: 0;
                left: 0;
                font-size: 3.6rem;
		color: var(--info);
                clip-path: inset(0 0 100% 0);
                animation: fill 2s infinite ease-in-out;
	}

	@keyframes fill {
		0%, 10% {
                        clip-path: inset(0 0 100% 0);
		}
		45%, 55% {
                        clip-path: inset(0 0 0 0);
		}
                90%, 100% {
                        clip-path: inset(100% 0 0 0);
                }
	}

	.card-text h2 {
		font-size: 1.5rem;
		font-weight: 700;
		color: var(--text);
		margin: 0 0 8px 0;
		letter-spacing: -0.01em;
	}

	.subtitle {
		font-size: 0.88rem;
		color: var(--text-muted);
		margin: 0;
		line-height: 1.4;
	}

	.steps-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
		text-align: left;
		background: var(--bg-light);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		padding: 14px 16px;
	}

	.step-item {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 6px 8px;
		border-radius: 6px;
		color: var(--text-muted);
		transition: all 0.15s ease;
	}

	.step-item.active {
		color: var(--text);
		background: color-mix(in srgb, var(--info) 10%, transparent);
	}

	.step-item.done {
		color: var(--text);
	}

	.step-icon {
		font-size: 1.3rem;
		color: var(--text-muted);
	}

	.step-item.active .step-icon {
		color: var(--info);
		animation: spin 2s linear infinite;
	}

	.step-item.done .step-icon {
		color: var(--success);
	}

	@keyframes spin {
		from { transform: rotate(0deg); }
		to { transform: rotate(360deg); }
	}

	.step-info {
		display: flex;
		flex-direction: column;
		gap: 2px;
		flex: 1;
	}

	.step-name {
		font-size: 0.9rem;
		font-weight: 600;
	}

	.step-desc {
		font-size: 0.76rem;
		color: var(--text-muted);
	}

	.progress-section {
		display: flex;
		flex-direction: column;
		gap: 8px;
		text-align: left;
	}

	.progress-labels {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-size: 0.82rem;
	}

	.current-task {
		color: var(--text);
		font-weight: 500;
	}

	.percent-label {
		color: var(--info);
		font-weight: 700;
		font-variant-numeric: tabular-nums;
	}

	.progress-bar-bg {
		width: 100%;
		height: 8px;
		background: var(--bg-light);
		border-radius: 4px;
		overflow: hidden;
		border: 1px solid var(--border-subtle);
	}

	.progress-bar-fill {
		height: 100%;
		background: var(--info);
		border-radius: 4px;
		transition: width 0.15s ease;
	}

	.error-banner {
		display: flex;
		align-items: center;
		gap: 8px;
		background: color-mix(in srgb, var(--danger) 15%, transparent);
		border: 1px solid var(--danger);
		color: var(--danger);
		padding: 10px 14px;
		border-radius: var(--radius-md);
		font-size: 0.85rem;
		text-align: left;
	}
</style>
