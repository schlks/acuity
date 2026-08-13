export interface ConfirmOptions {
	title?: string;
	message: string;
	confirmText?: string;
	cancelText?: string;
	danger?: boolean;
}

class ModalStore {
	isOpen = $state(false);
	title = $state('Confirm Action');
	message = $state('');
	confirmText = $state('Confirm');
	cancelText = $state('Cancel');
	danger = $state(false);

	private resolvePromise: ((value: boolean) => void) | null = null;

	confirm(options: ConfirmOptions): Promise<boolean> {
		this.title = options.title ?? 'Confirm Action';
		this.message = options.message;
		this.confirmText = options.confirmText ?? 'Confirm';
		this.cancelText = options.cancelText ?? 'Cancel';
		this.danger = options.danger ?? false;
		this.isOpen = true;

		return new Promise((resolve) => {
			this.resolvePromise = resolve;
		});
	}

	handleConfirm() {
		this.isOpen = false;
		this.resolvePromise?.(true);
		this.resolvePromise = null;
	}

	handleCancel() {
		this.isOpen = false;
		this.resolvePromise?.(false);
		this.resolvePromise = null;
	}
}

export const modal = new ModalStore();
