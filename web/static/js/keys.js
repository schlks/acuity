window.addEventListener('keydown', (e) => {
	// If typing in an input, only allow Escape to blur it
	if (['INPUT', 'TEXTAREA'].includes(e.target.tagName)) {
		if (e.key === 'Escape') {
			e.target.blur();
			e.preventDefault();
		}
		return;
	}

	const isCarouselOpen = document.querySelector('.carousel-modal')?.hasAttribute('open');
	
	switch(e.key) {
        case 'h':
        case 'H':
		case 'ArrowRight':
			if (isCarouselOpen) { window.dispatchEvent(new CustomEvent('acuity-next')); }
			else { moveGridFocus(1, 0); e.preventDefault(); }
			break;
        case 'l':
        case 'L':
		case 'ArrowLeft':
			if (isCarouselOpen) { window.dispatchEvent(new CustomEvent('acuity-prev')); }
			else { moveGridFocus(-1, 0); e.preventDefault(); }
			break;
        case 'k':
        case 'K':
		case 'ArrowUp':
			if (!isCarouselOpen) { moveGridFocus(0, -1); e.preventDefault(); }
			break;
        case 'j':
        case 'J':
		case 'ArrowDown':
			if (!isCarouselOpen) { moveGridFocus(0, 1); e.preventDefault(); }
			break;
		case 'Escape':
			window.dispatchEvent(new CustomEvent('acuity-escape'));
			break;
		case 'Delete':
			if (isCarouselOpen) {
				window.dispatchEvent(new CustomEvent('acuity-delete-carousel'));
			} else if (document.activeElement.classList.contains('image-card')) {
				const img = document.activeElement.querySelector('img');
				if (img && img.dataset.id) {
					const id = img.dataset.id;
					const isDeleteFromDisk = document.querySelector('body').__x?.$data?.deleteFromDisk;
					fetch(`/image/${id}?disk=${!!isDeleteFromDisk}`, { method: 'DELETE' })
						.then(() => {
							window.dispatchEvent(new CustomEvent('acuity-remove-selection', { detail: { id } }));
							document.activeElement.remove();
						});
				}
			}
			break;
		case 'd':
		case 'D':
			window.dispatchEvent(new CustomEvent('acuity-toggle-disk-delete'));
			break;
		case '/':
			if (isCarouselOpen) {
				window.dispatchEvent(new CustomEvent('acuity-search-similar'));
				e.preventDefault();
			} else {
				const searchInput = document.querySelector('input[name="q"]');
				if (searchInput) { searchInput.focus(); e.preventDefault(); }
			}
			break;
		case ' ':
			if (isCarouselOpen) {
				window.dispatchEvent(new CustomEvent('acuity-toggle-select-carousel'));
			} else {
				if (document.activeElement.classList.contains('image-card')) {
					const selectBtn = document.activeElement.querySelector('.select-btn');
					if (selectBtn) selectBtn.click();
				}
			}
			e.preventDefault();
			break;
		case '0': case '1': case '2': case '3': case '4': case '5':
			const rating = parseInt(e.key);
			if (isCarouselOpen) {
				window.dispatchEvent(new CustomEvent('acuity-rate-carousel', { detail: { rating } }));
			} else if (document.activeElement.classList.contains('image-card')) {
				const img = document.activeElement.querySelector('img');
				if (img && img.dataset.id) {
					const id = img.dataset.id;
					window.dispatchEvent(new CustomEvent('rating-updated', { detail: { id, rating } }));
					fetch(`/image/${id}`, {
						method: 'POST',
						headers: {'Content-Type': 'application/x-www-form-urlencoded'},
						body: `rating=${rating}`
					});
				}
			}
			break;
		case 'i':
		case 'I':
			if (isCarouselOpen) {
                window.dispatchEvent(new CustomEvent('acuity-toggle-info'));
            } else if (document.getElementById('settingsDialog')?.hasAttribute('open')) {
                const details = document.getElementById('settingsDialog').querySelector('details');
                if (details) {
                    details.open = !details.open;
                }
            }
			break;
		case 'Enter':
			if (!isCarouselOpen) {
				if (document.activeElement.classList.contains('image-card')) {
					const img = document.activeElement.querySelector('img');
					if (img && img.dataset.id) {
						const dupGroup = document.activeElement.closest('.duplicate-group');
						const group = dupGroup ? `#${dupGroup.id}` : undefined;
						window.dispatchEvent(new CustomEvent('open-carousel', { detail: { id: img.dataset.id, group } }));
					}
				} else if (document.activeElement.classList.contains('nav-item')) {
					document.activeElement.click();
				}
			}
			e.preventDefault();
			break;
		case 'a':
		case 'A':
			const addBtn = document.querySelector('[hx-get="/gallery/new"]');
			if (addBtn) addBtn.click();
			break;
		case 's':
		case 'S':
			const settingsBtn = document.querySelector('[hx-get="/settings"]');
			if (settingsBtn) settingsBtn.click();
			break;
		case 'u':
		case 'U':
			const dupBtn = document.querySelector('[hx-get="/gallery/global/duplicates"]');
			if (dupBtn) dupBtn.click();
			break;
		case 'g':
		case 'G':
			const firstGallery = document.querySelector('.nav-item[hx-get^="/gallery/"]');
			if (firstGallery) firstGallery.focus();
			break;
        case 'c':
        case 'C':
            window.dispatchEvent(new CustomEvent('acuity-copy'));
            break;
        case 'm':
        case 'M':
            window.dispatchEvent(new CustomEvent('acuity-move'));
            break;
	}
});

// Hide mouse cursor & hover effects when navigating via keyboard
window.addEventListener('keydown', (e) => {
	if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(e.key)) {
		document.body.classList.add('keyboard-navigating');
	}
});
window.addEventListener('mousemove', () => {
	document.body.classList.remove('keyboard-navigating');
});

function moveGridFocus(dx, dy) {
	const cards = Array.from(document.querySelectorAll('.image-card'));
	if (cards.length === 0) return;
	
	let currentIdx = cards.indexOf(document.activeElement);
	if (currentIdx === -1) {
		if (dx < 0 || dy < 0) {
			cards[cards.length - 1].focus();
		} else {
			cards[0].focus();
		}
		return;
	}
	
	if (dx !== 0) {
		let newIdx = currentIdx + dx;
		if (newIdx >= 0 && newIdx < cards.length) {
			cards[newIdx].focus();
		} else if (newIdx >= cards.length) {
			const nextBtn = document.getElementById('next-page-btn');
			if (nextBtn) { sessionStorage.setItem('acuity-focus', 'first'); nextBtn.click(); }
		} else if (newIdx < 0) {
			const prevBtn = document.getElementById('prev-page-btn');
			if (prevBtn) { sessionStorage.setItem('acuity-focus', 'last'); prevBtn.click(); }
		}
	} else if (dy !== 0) {
		const currentCard = cards[currentIdx];
		const currentTop = currentCard.offsetTop;
		const currentCX = currentCard.offsetLeft + currentCard.offsetWidth / 2;
		
		let targetTop = -1;
		if (dy < 0) {
			let tops = cards.map(c => c.offsetTop).filter(t => t < currentTop - 5);
			if (tops.length > 0) targetTop = Math.max(...tops);
		} else {
			let tops = cards.map(c => c.offsetTop).filter(t => t > currentTop + 5);
			if (tops.length > 0) targetTop = Math.min(...tops);
		}
		
		if (targetTop !== -1) {
			let bestCard = null;
			let minDiff = Infinity;
			for (let i = 0; i < cards.length; i++) {
				if (Math.abs(cards[i].offsetTop - targetTop) < 5) {
					const cx = cards[i].offsetLeft + cards[i].offsetWidth / 2;
					const diff = Math.abs(cx - currentCX);
					if (diff < minDiff) {
						minDiff = diff;
						bestCard = cards[i];
					}
				}
			}
			if (bestCard) bestCard.focus();
		} else {
			if (dy > 0) {
				const nextBtn = document.getElementById('next-page-btn');
				if (nextBtn) { sessionStorage.setItem('acuity-focus', 'first'); nextBtn.click(); }
				else cards[cards.length - 1].focus();
			} else {
				const prevBtn = document.getElementById('prev-page-btn');
				if (prevBtn) { sessionStorage.setItem('acuity-focus', 'last'); prevBtn.click(); }
				else cards[0].focus();
			}
		}
	}
}

window.addEventListener('htmx:afterSettle', () => {
	const focusAction = sessionStorage.getItem('acuity-focus');
	if (focusAction) {
		sessionStorage.removeItem('acuity-focus');
		const cards = Array.from(document.querySelectorAll('.image-card'));
		if (cards.length > 0) {
			if (focusAction === 'last') cards[cards.length - 1].focus();
			else if (focusAction === 'first') cards[0].focus();
		}
	}
});
