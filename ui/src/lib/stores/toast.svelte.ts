export interface Toast {
	id: string;
	type: 'info' | 'success' | 'danger';
	message: string;
	duration?: number;
}

class ToastStore {
	items = $state<Toast[]>([]);

	show(message: string, type: 'info' | 'success' | 'danger' = 'info', duration = 3000) {
		const id = Math.random().toString(36).substring(2, 9);
		const item: Toast = { id, message, type, duration };
		this.items = [...this.items, item];

		if (duration > 0) {
			setTimeout(() => {
				this.dismiss(id);
			}, duration);
		}
		return id;
	}

	info(message: string, duration = 3000) {
		return this.show(message, 'info', duration);
	}

	success(message: string, duration = 3000) {
		return this.show(message, 'success', duration);
	}

	error(message: string, duration = 4000) {
		return this.show(message, 'danger', duration);
	}

	dismiss(id: string) {
		this.items = this.items.filter((item) => item.id !== id);
	}
}

export const toast = new ToastStore();
