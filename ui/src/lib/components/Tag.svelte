<script>
	let { label, color = "var(--primary)"} = $props();
</script>

<span class="tag" style="--tag-color: {color}">
	{label}
</span>

<style>
	.tag {
		display: inline-flex;
		align-items: center;
		padding: 0.25rem 0.75rem;
		border-radius: 9999px;
		font-size: 0.75rem;
		font-weight: 600;

		color: var(--tag-color);
		background-color: color-mix(in srgb, var(--tag-color) 15%, transparent);
	}
</style>
