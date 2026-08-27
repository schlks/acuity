<script lang="ts">
	import { page } from '$app/state';
	import { slide } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Carousel from '$lib/components/Carousel.svelte';
	import ProgressOverlay from '$lib/components/ProgressOverlay.svelte';
	import ToastContainer from '$lib/components/ToastContainer.svelte';
	import ConfirmModal from '$lib/components/ConfirmModal.svelte';
	import AIModelSetupModal from '$lib/components/AIModelSetupModal.svelte';
	import '../app.css';

	let { data, children } = $props();
	let showSidebar = $derived(page.url.pathname !== '/');
</script>

<svelte:head>
	<link rel="icon" href=/logo.svg />
</svelte:head>

<svelte:window
	ondragover={(e) => e.preventDefault()}
	ondrop={(e) => e.preventDefault()}
/>

<div class="app-layout">
	{#if showSidebar}
		<div transition:slide={{ axis: 'x', duration: 220, easing: quintOut }} class="sidebar-wrapper">
			<Sidebar galleries={data.galleries} />
		</div>
	{/if}

	<main class="main-content">
		{@render children()}
	</main>
</div>

<Carousel />
<ProgressOverlay />
<ToastContainer />
<ConfirmModal />
<AIModelSetupModal />

<style>
	.sidebar-wrapper {
		height: 100%;
		display: flex;
		flex-shrink: 0;
	}
</style>
