<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Carousel from '$lib/components/Carousel.svelte';
	import ProgressOverlay from '$lib/components/ProgressOverlay.svelte';
	import '../app.css';

	let { data, children } = $props();
</script>

<svelte:head>
	<link rel="icon" href=/logo.svg />
</svelte:head>

<div class="app-layout">
	<Sidebar galleries={data.galleries} />

	<main class="main-content">
		{@render children()}
	</main>
</div>

<Carousel />
<ProgressOverlay />
