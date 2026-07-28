<script lang="ts">
    import type { PageData } from './$types';

    export let data: PageData;
</script>

<h1>Search</h1>

<form method="GET" class="search-form">
    <input
        type="search"
        name="q"
        placeholder="Search for images"
        value={data.query ?? ''}
    />
    <button type="submit">Search</button>
</form>

{#if data.result && data.results.length > 0}
	<ul>
		{#each data.result as result}
			<li>{result.name}</li>
		{/each}
	</ul>
{:else if data.query}
	<p>No results found for "{data.query}".</p>
{/if}

<style>
	search-form {
		display: flex;
		gap: 0.5rem;
		margin-bottom: 1rem;
	}
</style>
