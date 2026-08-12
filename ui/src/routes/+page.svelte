<script lang="ts">

	let searchQuery = $state('');

	function handleSearch(event: SubmitEvent | Event) {
		event.preventDefault();
		console.log("Submitting search to API for:", searchQuery);
	}
</script>

<div class="search-container">
	<form class="search-form" onsubmit={handleSearch}>
		<input
			type="search"
			placeholder="Search for images"
			bind:value={searchQuery}
		/>
	</form>	
</div>

<style>
	.search-container {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 2rem;
	}

	.search-form {
		width: 100%;
		max-width: 500px;
		justify-self: center;
	}

	.search-form input {
		width: 100%;
		padding: 14px 20px 14px 48px;
		font-size: 1.1rem;
		background-color: var(--bg);
		color: var(--text);
		border: 3px solid transparent;
		border-radius: 16px;
		outline: none;
		transition: border-color 0.2s;
	}

	.search-form input:focus {
		border-color: var(--primary);
		box-shadow: 0 0 40px color-mix(in srgb, var(--primary) 45%, transparent);
	}

	.search-form input:hover {
		box-shadow: 0 0 40px color-mix(in srgb, var(--primary) 45%, transparent);
	}
</style>
