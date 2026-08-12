<script lang="ts">
    import { onMount } from 'svelte';
    import { invalidateAll } from '$app/navigation';
    import Spinner from './Spinner.svelte';
    import { fly, fade } from 'svelte/transition';

    let progresses = $state([]);
    let isPolling = false;
    let lastStr = "";
    let completed = $state(false);

    async function pollProgress() {
        while (isPolling) {
            try {
                const res = await fetch(`/api/progress?last=${encodeURIComponent(lastStr)}`);
                if (res.ok) {
                    const contentType = res.headers.get("content-type");
                    if (contentType && contentType.includes("application/json")) {
                        const data = await res.json();
                        progresses = data.progresses || [];
                        lastStr = data.last_str || "";
                    } else {
                        if (progresses.length > 0) {
                            await invalidateAll();
                            completed = true;
                            // Warte 1,5 Sekunden bevor es ausgeblendet wird
                            await new Promise(r => setTimeout(r, 1500));
                            completed = false;
                        }
                        progresses = [];
                        lastStr = "";
                        await new Promise(r => setTimeout(r, 2000));
                    }
                } else {
                    await new Promise(r => setTimeout(r, 5000));
                }
            } catch (e) {
                await new Promise(r => setTimeout(r, 5000));
            }
        }

        onMount(() => {
            isPolling = true;
            pollProgress();

            return () => {
                isPolling = false;
            }
        })
    }

    async function cancelImport(name: string) {
        await fetch(`/api/gallery/${encodeURIComponent(name)}/scan`, {
            method: 'DELETE'
        });
    }

    onMount(() => {
        pollProgress();
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
                        <div class="progress-item">
                            <span class="gallery-name">{p.GalleryName} </span>
                            <div class="progress-stats">
                                <span class="percent">{p.Percent}%</span>
                                <span class="count">({p.Current} / {p.Expected === -1 ? '?' : p.Expected})</span>
                                <button class="cancel-btn" onclick={() => cancelImport(p.GalleryName)} title="Cancel Import">
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
        background: var(--bg-light);
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
        background: var(--bg);
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
    }

    .progress-stats {
        display: flex;
        align-items: center;
        gap: 8px;
        flex-shrink: 0;
    }

    .percent {
        color: var(--primary);
        font-weight: 600;
    }

    .count {
        color: var(--text-muted);
        font-size: 0.9rem;
    }

    .cancel-btn {
        background: transparent;
        border: none;
        color: #ff5555;
        cursor: pointer;
        padding: 4px;
        margin-left: 8px;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 4px;
        transition: background 0.2s;
    }

    .cancel-btn:hover {
        background: rgba(255, 60, 60, 0.1);
    }

    .cancel-btn .material-symbols-outlined {
        font-size: 1.2rem;
    }

    .checkmark-circle {
        width: 48px;
        height: 48px;
        border-radius: 50%;
        background: rgba(46, 213, 115, 0.15);
        display: flex;
        align-items: center;
        justify-content: center;
        margin-bottom: 8px;
    }

    .check-icon {
        color: #2ed573;
        font-size: 32px;
    }
</style>
