<script lang="ts">
	import { page } from '$app/state';
</script>

<div class="error-container">
	<div class="error-card">
		<span class="status-code">{page.status}</span>
		<h1>{page.status === 404 ? 'Page not found' : 'Something went wrong'}</h1>
		<p>{page.error?.message ?? 'Something unexpected happened.'}</p>

		<a href="/" class="btn-home">
			Back to Overview
		</a>
	</div>
</div>

<style>
	.error-container {
		display: flex;
		align-items: center;
		justify-content: center;
		flex: 1;
		height: 100%;
		padding: 24px;
	}

	.error-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		text-align: center;
		max-width: 460px;
		background-color: var(--bg);
		border: 1px solid var(--bg-light);
		border-radius: 12px;
		padding: 40px 32px;
		gap: 16px;
	}

	.status-code {
		font-size: 4rem;
		font-weight: 800;
		line-height: 1;
		color: var(--info);
		letter-spacing: -2px;
	}

	h1 {
		font-size: 1.5rem;
		color: var(--text);
	}

	p {
		font-size: 0.95rem;
		color: var(--text-muted);
		margin: 0 0 8px 0;
		line-height: 1.5;
	}

	.btn-home {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		position: relative;
		z-index: 1;
		font-family: inherit;
		font-size: 0.95rem;
		text-decoration: none;
		border-radius: 8px;
		padding: 10px 20px;
		color: var(--text);
		background: transparent;
		transition: color 0.3s;
	}

	.btn-home::before {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 2px;
		border-radius: 0px;
		z-index: -1;
		background: var(--primary);
		transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.btn-home:hover {
		color: var(--bg-dark);
	}

	.btn-home:hover::before {
		height: 100%;
		border-radius: 8px;
	}
</style>
