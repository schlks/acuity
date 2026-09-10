<script lang="ts">
	import { fly } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
	import { page } from '$app/state';
        import {
                formatSize,
                formatDate,
                formatResolution,
                formatExt,
                formatAperture,
                formatShutter,
                formatIso,
                formatFocalLength,
                formatLens
        } from '$lib/format';
	
	let { image, onDelete, onClose } = $props();
	let galleryName = $derived(
		page.data?.galleries?.find((g: any) => g.id === image.gallery_id)?.name ||
		page.data?.gallery?.name ||
		page.params.name ||
		'global'
	)
</script>

<div class="info-popup" transition:fly={{ y: -15, duration: 250, easing: quintOut }}>
	<h3>Details</h3>
	<hr />

	<div class="info-group">
		<span class="material-symbols-outlined">photo_camera</span>
		<div class="info-text">
			<span class="label">Camera and Lens</span>
			<span class="value">{formatLens(image.camera_make, image.lens_make)}</span>
		</div>
	</div>

	<div class="info-grid">
		<div class="grid-item">
			<span class="label">Aperture</span>
			<span class="value">{formatAperture(image.aperture)}</span>
		</div>
		<div class="grid-item">
			<span class="label">Shutter Speed</span>
			<span class="value">{formatShutter(image.shutter_speed)}</span>
		</div>
		<div class="grid-item">
			<span class="label">ISO</span>
			<span class="value">{formatIso(image.iso)}</span>
		</div>
		<div class="grid-item">
			<span class="label">Focal Length</span>
			<span class="value">{formatFocalLength(image.focal_length)}</span>
		</div>
                <div class="grid-item">
                        {#if image.flash}
                                <span class="material-symbols-outlined">flash_on</span>
                        {:else}
                                <span class="material-symbols-outlined icon">flash_off</span>
                        {/if}
                </div>
	</div>

    
	<hr />

	<div class="info-group">
		<span class="material-symbols-outlined">insert_drive_file</span>
		<div class="info-text">
			<span class="label">File</span>
			<span class="value">{formatResolution(image.resolution)} • {formatSize(image.size)} • {formatExt(image.extension)}</span>
			<span class="sub-value">{image.filepath.split('/').pop().split('.')[0]}</span>
		</div>
	</div>

        {#if image.taken !== image.date}
                <div class="info-group">
                        <span class="material-symbols-outlined">today</span>
                        <div class="info-text">
                                <span class="label">Date Taken</span>
                                <span class="value">{formatDate(image.taken)}</span>
                        </div>
                </div>
        {/if}

	<div class="info-group">
		<span class="material-symbols-outlined">event</span>
		<div class="info-text">
			<span class="label">Modified</span>
			<span class="value">{formatDate(image.date)}</span>
		</div>
	</div>

	<hr />

	<div class="info-actions">
		<a href="/gallery/{galleryName}/duplicates?imageID={image.id}" class="action-btn" onclick={onClose}>
			<span class="material-symbols-outlined">search</span>
		</a>
		<button class="action-btn danger" onclick={onDelete}>
			<span class="material-symbols-outlined">delete</span>
		</button>
	</div>
</div>

<style>
	.info-popup {
		position: absolute;
		top: 80px;
		right: 32px;
		width: 320px;
		max-height: calc(100vh - 120px);
		background: var(--bg-light, #1c1c1c); 
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 12px;
		box-shadow: 0 10px 40px rgba(0,0,0,0.5);
		padding: 24px;
		display: flex;
		flex-direction: column;
		gap: 16px;
		overflow-y: auto;
		z-index: 1000;
		color: var(--text);
		pointer-events: auto; /* Wichtig für Klicks! */
	}

	.info-actions {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
		margin-top: 8px;
	}

	.action-btn {
		position: relative;
		z-index: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 8px;
		border-radius: 8px;
		background: transparent;
		color: var(--text);
		font-family: inherit;
		font-size: 0.95rem;
		font-weight: 500;
		cursor: pointer;
		text-decoration: none;
		border: none;
		transition: color 0.3s;
	}

	.action-btn::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		border-radius: 0;
		z-index: -1;
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
		background: var(--primary); /* Standard-Farbe für Suchen */
	}

	.action-btn:hover {
		color: var(--bg-dark);
	}

	.action-btn:hover::before {
		height: 100%;
		border-radius: 8px;
	}

	.action-btn.danger {
		color: var(--text-muted);
	}

	.action-btn.danger::before {
		background: var(--danger);
	}

	.action-btn.danger:hover {
		color: var(--bg-dark);
	}

	h3 {
		margin: 0;
		font-size: 1.2rem;
		font-weight: 600;
	}

	hr {
		border: none;
		border-top: 1px solid rgba(255, 255, 255, 0.1);
		margin: 0;
	}

	.info-group {
		display: flex;
		align-items: flex-start;
		gap: 16px;
	}

	.info-group .material-symbols-outlined {
		color: var(--text-muted, #aaa);
		font-size: 1.5rem;
		margin-top: 2px;
	}

	.info-text {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.info-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 16px;
		background: rgba(255, 255, 255, 0.05);
		padding: 16px;
		border-radius: 12px;
	}

	.grid-item {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.label {
		font-size: 0.75rem;
		text-transform: uppercase;
		letter-spacing: 0.03em;
		color: var(--text-muted, #aaa);
	}

	.value {
		font-size: 0.95rem;
		font-weight: 500;
	}

        .icon {
            font-size: 1.2rem;
            font-weight: 500;
        }

	.sub-value {
		font-size: 0.75rem;
		color: var(--text-muted, #aaa);
		margin-top: 4px;
		word-break: break-all;
	}
</style>
