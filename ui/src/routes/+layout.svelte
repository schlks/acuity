<script lang="ts">
	import logo from '$lib/assets/logo.svg';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Carousel from '$lib/components/Carousel.svelte';
	import '../app.css';

	let { data, children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={logo} />
</svelte:head>

<div class="app-layout">
	<Sidebar galleries={data.galleries} />

	<main class="main-content">
		{@render children()}
	</main>
</div>

<Carousel />
