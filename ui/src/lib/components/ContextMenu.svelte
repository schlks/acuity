<script lang="ts">
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { toast } from '$lib/stores/toast.svelte';
    import type { GalleryImage } from '$lib/types';

    interface ContextMenuProps {
        x: number;
        y: number;
        image: GalleryImage;
        galleryName: string;
        onClose: () => void;
        onFlag?: (image: GalleryImage, flag: number) => void;
        onRating?: (image: GalleryImage, rating: number) => void;
        onDelete?: (image: GalleryImage) => void;
        onMove?: (image: GalleryImage) => void;
    }

    let {
        x,
        y,
        image,
        galleryName,
        onClose,
        onFlag,
        onRating,
        onDelete,
        onMove
    }: ContextMenuProps = $props();

    let menuElement = $state<HTMLDivElement | null>(null);
    let pos = $state({ top: y, left: x});

    $effect(() => {
        if (!menuElement) return;
        const rect = menuElement.getBoundingClientRect();
        const winWidth = window.innerWidth;
        const winHeight = window.innerHeight;

        let nextLeft = x;
        let nextTop = y;

        if(x + rect.width > winWidth - 10) {
            nextLeft = Math.max(10, winWidth - rect.width - 10)
        }
        if (y + rect.height > winHeight - 10) {
            nextTop = Math.max(10, winHeight - rect.height - 10);
        }

        pos = { top: nextTop, left: nextLeft };
    })

    async function copyImage() {
        try {
            onClose();
            const imageBlobPromise = (async () => {
                const res = await fetch(`/api/image?path=${encodeURIComponent(image.filepath)}`);
                const blob = await res.blob();

                if (blob.type === 'image/png') return blob;

                const img = new Image();
                img.src = URL.createObjectURL(blob);
                await new Promise((resolve) => (img.onload = resolve));

                const canvas = document.createElement('canvas');
                canvas.width = img.naturalWidth;
                canvas.height = img.naturalHeight;
                const ctx = canvas.getContext('2d');
                ctx?.drawImage(img, 0, 0);

                const pngBlob = await new Promise<Blob>((resolve) => canvas.toBlob((b) => resolve(b!), 'image/png'));
                URL.revokeObjectURL(img.src);
                return pngBlob;
            })();

            await navigator.clipboard.write([
                new ClipboardItem({ 'image/png': imageBlobPromise })
            ]);
            toast.success('Image copied to clipboard');
        } catch (err) {
            console.error('Failed to copy image', err);
            toast.error('Failed to copy image');
        }
    }

    async function copyPath() {
        try {
            onClose();
            await navigator.clipboard.writeText(image.filepath);
            toast.success('Path copied to clipboard');
        } catch (err) {
            console.error('Failed to copy path', err);
            toast.error('Failed to copy path');
        }
    }

    function handleFindDuplicates() {
        onClose();
        goto(`/gallery/${encodeURIComponent(galleryName)}/duplicates?imageID=${image.id}`);
    }

    function handleKeyDown(e: KeyboardEvent) {
        if (e.key === 'Escape') {
            onClose();
        }
    }

    function handleWindowClick(e: MouseEvent) {
        if (menuElement && !menuElement.contains(e.target as Node)) {
            onClose();
        }
    }
</script>

<svelte:window onkeydown={handleKeyDown} onclick={handleWindowClick} />

<div
    bind:this={menuElement}
    class="context-menu"
    style="top: {pos.top}px; left: {pos.left}px;"
    tabindex = "-1"
>
    <button class="menu-item" onclick={copyImage}>
            <span class="material-symbols-outlined">content_copy</span>
            <span>Copy Image</span>
        </button>

        <button class="menu-item" onclick={copyPath}>
            <span class="material-symbols-outlined">link</span>
            <span>Copy Path</span>
        </button>

        <hr class="divider" />

        <button class="menu-item" onclick={handleFindDuplicates}>
            <span class="material-symbols-outlined">auto_awesome</span>
            <span>Search for duplicates</span>
        </button>

        <hr class="divider" />

        <div class="menu-section-title">Flag</div>
        <div class="flag-actions">
            <button
                class="flag-btn keep"
                class:active={image.flag === 1}
                title="Keep (1)"
                onclick={() => { onFlag?.(image, 1); onClose(); }}
            >
                <span class="material-symbols-outlined">check_circle</span>
                <span>Keep</span>
            </button>
            <button
                    class="flag-btn clear"
                    class:active={image.flag === 0}
                    title="Unflag (0)"
                    onclick={() => { onFlag?.(image, 0); onClose(); }}
            >
                <span class="material-symbols-outlined">remove_circle_outline</span>
                <span>Unflag</span>
            </button>
            <button
                    class="flag-btn reject"
                    class:active={image.flag === -1}
                    title="Reject (-1)"
                    onclick={() => { onFlag?.(image, -1); onClose(); }}
            >
                <span class="material-symbols-outlined">cancel</span>
                <span>Reject</span>
            </button>
        </div>

        <div class="menu-section-title">Rating</div>
        <div class="rating-actions">
            {#each [1, 2, 3, 4, 5] as star}
                <button
                    class="star-btn"
                    class:active={image.rating >= star}
                    onclick={() => { onRating?.(image, image.rating === star ? 0 : star); onClose(); }}
                    title="{star} Stars"
                >
                    <span class="material-symbols-outlined">star</span>
                </button>
            {/each}
        </div>

        <hr class="divider" />

        {#if onMove}
            <button class="menu-item" onclick={() => { onMove(image); onClose() }}>
                <span class="material-symbols-outlined">drive_file_move</span>
                <span>Move to Gallery</span>
            </button>
        {/if}

        {#if onDelete}
            <button class="menu-item danger" onclick={() => { onDelete(image); onClose(); }}>
                <span class="material-symbols-outlined">delete</span>
                <span>Delete Image</span>
            </button>
        {/if}
</div>

<style>
	.context-menu {
		position: fixed;
		z-index: 10000;
		min-width: 220px;
		background: color-mix(in srgb, var(--bg) 94%, black);
		backdrop-filter: blur(16px);
		-webkit-backdrop-filter: blur(16px);
		border: 1px solid color-mix(in srgb, var(--secondary) 40%, var(--bg-light));
		border-radius: 8px;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.7);
		padding: 6px;
		display: flex;
		flex-direction: column;
		gap: 2px;
		user-select: none;
		animation: popIn 0.12s cubic-bezier(0.16, 1, 0.3, 1);
	}

	@keyframes popIn {
		from {
			opacity: 0;
			transform: scale(0.96);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}

	.status-banner {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 12px 16px;
		font-size: 0.9rem;
		font-weight: 500;
		color: var(--info);
		justify-content: center;
	}

	.status-banner .material-symbols-outlined {
		color: var(--info);
		font-size: 1.2rem;
	}

	.menu-section-title {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--info);
		padding: 4px 12px 2px 12px;
		font-weight: 600;
	}

	.menu-item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 8px 12px;
		border: none;
		background: transparent;
		color: var(--text);
		font-size: 0.85rem;
		font-weight: 500;
		border-radius: 6px;
		cursor: pointer;
		text-align: left;
		transition: all 0.12s ease;
	}

	.menu-item:hover {
		background: var(--bg-light);
		color: var(--primary);
	}

	.menu-item .material-symbols-outlined {
		font-size: 1.15rem;
		color: var(--text-muted);
		transition: color 0.12s ease;
	}

	.menu-item:hover .material-symbols-outlined {
		color: var(--primary);
	}

	.menu-item.danger {
		color: var(--danger);
	}

	.menu-item.danger .material-symbols-outlined {
		color: var(--danger);
	}

	.menu-item.danger:hover {
		background: color-mix(in srgb, var(--danger) 15%, transparent);
	}

	.divider {
		border: none;
		height: 1px;
		background: color-mix(in srgb, var(--secondary) 30%, var(--bg-light));
		margin: 4px 0;
	}

	.flag-actions {
		display: grid;
		grid-template-columns: 1fr 1fr 1fr;
		gap: 4px;
		padding: 2px 6px;
	}

	.flag-btn {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 3px;
		padding: 6px 4px;
		border: 1px solid var(--bg-light);
		background: transparent;
		border-radius: 6px;
		cursor: pointer;
		font-size: 0.72rem;
		color: var(--text-muted);
		transition: all 0.12s ease;
	}

	.flag-btn:hover {
		background: var(--bg-light);
		color: var(--primary);
		border-color: var(--primary);
	}

	.flag-btn.active {
		border-color: var(--tertiary);
		background: color-mix(in srgb, var(--tertiary) 15%, transparent);
		color: var(--tertiary);
	}

	.rating-actions {
		display: flex;
		justify-content: space-between;
		padding: 4px 8px;
	}

	.star-btn {
		background: transparent;
		border: none;
		cursor: pointer;
		padding: 4px;
		border-radius: 4px;
		color: var(--bg-light);
		transition: all 0.12s ease;
	}

	.star-btn:hover {
		color: var(--primary);
		transform: scale(1.1);
	}

	.star-btn.active {
		color: var(--tertiary);
		transform: scale(1.1);
	}

	.star-btn .material-symbols-outlined {
		font-size: 1.25rem;
		font-variation-settings: 'FILL' 1;
	}
</style>