<svg class="spinner" viewBox="25 25 50 50">
	<circle cx="50" cy="50" r="20"></circle>
</svg>

<style>
	.spinner {
		width: 48px;
		height: 48px;
		animation: rotate 2s linear infinite;
		transform-origin: center center;
	}

	circle {
		fill: none;
		stroke: var(--primary);
		stroke-width: 4;
		stroke-dasharray: 1, 200;
		stroke-dashoffset: 0;
		stroke-linecap: round;
		animation: dash 1.5s ease-in-out infinite;
	}

	@keyframes rotate {
		100% {
			transform: rotate(360deg);
		}
	}

	@keyframes dash {
		0% {
			stroke-dasharray: 1, 200;
			stroke-dashoffset: 0;
		}
		50% {
			stroke-dasharray: 90, 200;
			stroke-dashoffset: -35px;
		}
		100% {
			stroke-dasharray: 90, 200;
			stroke-dashoffset: -124px;
		}
	}
</style>
