<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { invalidateAll } from '$app/navigation';
    import Spinner from './Spinner.svelte';
    import { fly, fade } from 'svelte/transition';

    interface ProgressItem {
        gallery_name: string;
        current: number;
        expected: number;
        percent: number;
    }

    let progresses = $state<ProgressItem[]>([]);
    let completed = $state(false);
    let hadActive = false;
    let isPolling = true;
    let pollTimeout: any = null;

    async function pollProgress() {
        if (!isPolling) return;
        try {
            const res = await fetch('/api/progress');
            if (res.ok) {
                const data = await res.json();
                const items: ProgressItem[] = data.progresses || [];

                if (items.length > 0) {
                    hadActive = true;
                    progresses = items;
                } else {
                    if (hadActive) {
                        hadActive = false;
                        progresses = [];
                        completed = true;
                        await invalidateAll();
                        setTimeout(() => {
                            completed = false;
                        }, 1500);
                    } else {
                        progresses = [];
                    }
                }
            }
        } catch (e) {
            // ignore network errors during navigation
        }

        if (isPolling) {
            const delay = progresses.length > 0 ? 400 : 1200;
            pollTimeout = setTimeout(pollProgress, delay);
        }
    }

    async function cancelImport(name: string) {
        await fetch(`/api/gallery/${encodeURIComponent(name)}/scan`, {
            method: 'DELETE'
        });
        progresses = progresses.filter(p => (p.gallery_name || (p as any).GalleryName) !== name);
        if (progresses.length === 0) {
            hadActive = false;
        }
    }

    onMount(() => {
        isPolling = true;
        pollProgress();
    });

    onDestroy(() => {
        isPolling = false;
        if (pollTimeout) clearTimeout(pollTimeout);
    });
</script>

{#if progresses.length > 0 || completed}
    <div class="overlay" transition:fade={{ duration: 300 }}>
        <div class="progress-box" transition:fly={{ y: 20, duration: 400 }}>
            {#if completed}
                <div class="checkmark-circle">
                    <span class="material-symbols-outlined check-icon">check</span>
                </div>
                <h3>Import Complete!</h3>
            {:else}
                <Spinner />
                <h3>Importing Gallery</h3>
                <div class="progress-list">
                    {#each progresses as p}
                        {@const name = p.gallery_name || (p as any).GalleryName}
                        {@const current = p.current ?? (p as any).Current ?? 0}
                        {@const expected = p.expected ?? (p as any).Expected ?? -1}
                        {@const percent = p.percent ?? (p as any).Percent ?? 0}
                        <div class="progress-item">
                            <span class="gallery-name">{name}</span>
                            <div class="progress-stats">
                                <span class="percent">{percent}%</span>
                                <span class="count">({current} / {expected === -1 ? '?' : expected})</span>
                                <button class="cancel-btn" onclick={() => cancelImport(name)} title="Cancel Import">
                                    <span class="material-symbols-outlined">close</span>
                                </button>
                            </div>
                        </div>
                    {/each}
                </div>
            {/if}
        </div>
    </div>
{/if}

<style>
    .overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100vw;
        height: 100vh;
        background: rgba(0, 0, 0, 0.6);
        backdrop-filter: blur(12px);
        z-index: 9999;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .progress-box {
        background: var(--bg);
        border: 1px solid var(--border);
        border-radius: 12px;
        padding: 32px 40px;
        display: flex;
        flex-direction: column;
        align-items: center;
        min-width: 420px;
        box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
    }

    h3 {
        margin: 24px 0 16px;
        color: var(--text);
        font-size: 1.25rem;
        font-weight: 600;
    }

    .progress-list {
        display: flex;
        flex-direction: column;
        gap: 12px;
        width: 100%;
    }

    .progress-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 20px;
        background: var(--bg-light);
        padding: 12px 16px;
        border-radius: 8px;
        border: 1px solid var(--border);
    }

    .gallery-name {
        color: var(--text);
        font-weight: 500;
        font-size: 1rem;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        max-width: 180px;
    }

    .progress-stats {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .percent {
        font-weight: 600;
        color: var(--info);
        min-width: 45px;
        text-align: right;
    }

    .count {
        color: var(--text-secondary);
        font-size: 0.9rem;
        min-width: 80px;
        text-align: right;
    }

    .cancel-btn {
        background: none;
        border: none;
        color: var(--text-secondary);
        cursor: pointer;
        padding: 4px;
        border-radius: 4px;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: all 0.2s ease;
    }

    .cancel-btn:hover {
        color: var(--danger);
        background: rgba(255, 255, 255, 0.05);
    }

    .checkmark-circle {
        width: 56px;
        height: 56px;
        border-radius: 50%;
        background: var(--success);
        display: flex;
        align-items: center;
        justify-content: center;
        animation: scaleUp 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
    }

    .check-icon {
        color: white;
        font-size: 36px;
    }

    @keyframes scaleUp {
        0% {
            transform: scale(0);
        }
        100% {
            transform: scale(1);
        }
    }
</style>
