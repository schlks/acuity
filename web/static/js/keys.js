window.addEventListener('keydown', (e) => {
	// If typing in an input, only allow Escape to blur it
	if (['INPUT', 'TEXTAREA'].includes(e.target.tagName)) {
		if (e.key === 'Escape') {
			e.target.blur();
			e.preventDefault();
		}
		return;
	}

	const isCarouselOpen = document.querySelector('.carousel-modal[open], #culling-modal[open]') !== null;
	
	switch(e.key) {
        case 'h': case 'H': case 'ArrowRight':
			if (isCarouselOpen) { window.dispatchEvent(new CustomEvent('acuity-next')); }
			else { moveGridFocus(1, 0); e.preventDefault(); }
			break;
        case 'l': case 'L': case 'ArrowLeft':
			if (isCarouselOpen) { window.dispatchEvent(new CustomEvent('acuity-prev')); }
			else { moveGridFocus(-1, 0); e.preventDefault(); }
			break;
        case 'k': case 'K': case 'ArrowUp':
			if (!isCarouselOpen) {
				if (document.activeElement.classList.contains('nav-item')) {
					moveSidebarFocus(-1);
				} else {
					moveGridFocus(0, -1);
				}
				e.preventDefault();
			} else {
				window.dispatchEvent(new CustomEvent('acuity-arrow-up'));
				e.preventDefault();
			}
			break;
        case 'j': case 'J': case 'ArrowDown':
			if (!isCarouselOpen) {
				if (document.activeElement.classList.contains('nav-item')) {
					moveSidebarFocus(1);
				} else {
					moveGridFocus(0, 1);
				}
				e.preventDefault();
			} else {
				window.dispatchEvent(new CustomEvent('acuity-arrow-down'));
				e.preventDefault();
			}
			break;
		case 'PageDown':
			if (isCarouselOpen) { window.dispatchEvent(new CustomEvent('acuity-page-next')); }
			break;
		case 'PageUp':
			if (isCarouselOpen) { window.dispatchEvent(new CustomEvent('acuity-page-prev')); }
			break;
		case 'Escape':
			if (isCarouselOpen) {
				window.dispatchEvent(new CustomEvent('acuity-escape'));
			} else {
				window.dispatchEvent(new CustomEvent('acuity-clear-selection'));
			}
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
						});
				}
			}
			break;
		case 'd': case 'D':
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
				window.dispatchEvent(new CustomEvent('acuity-space-carousel'));
			} else {
				if (document.activeElement.classList.contains('image-card')) {
					const img = document.activeElement.querySelector('img');
					if (img && img.dataset.id) {
						window.dispatchEvent(new CustomEvent('acuity-toggle-select', { detail: { id: img.dataset.id } }));
					}
				}
			}
			e.preventDefault();
			break;
		case 'Backspace':
			if (isCarouselOpen) {
				window.dispatchEvent(new CustomEvent('acuity-backspace-carousel'));
				e.preventDefault();
			}
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
		case 'i': case 'I':
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
		case 'a': case 'A':
            const addDialog = document.getElementById('addDialog');
            if (addDialog) {
                e.preventDefault();
                addDialog.showModal();
            }
			break;
		case 'b': case 'B':
			if (!isCarouselOpen) {
				window.dispatchEvent(new CustomEvent('acuity-start-culling'));
			}
			break;
		case 'n': case 'N':
			if (document.activeElement.classList.contains('image-card') && !isCarouselOpen) {
				const img = document.activeElement.querySelector('img');
				if (img && img.dataset.id) {
					const id = img.dataset.id;
					const badge = document.getElementById('flag-badge-' + id);
					let currentFlag = 0;
					if (badge && badge.innerHTML.includes('check_circle')) currentFlag = 1;
					else if (badge && badge.innerHTML.includes('cancel')) currentFlag = -1;
					
					let newFlag = currentFlag === 1 ? 0 : 1;
					const sidebarContainer = document.getElementById('flag-container-' + id);
					if (sidebarContainer) {
						htmx.ajax('POST', `/image/${id}/flag`, {values: {flag: newFlag}, target: '#flag-container-' + id, swap: 'outerHTML'});
					} else {
						fetch(`/image/${id}/flag`, {method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body: `flag=${newFlag}`})
							.then(() => {
								if (badge) badge.outerHTML = '<div id=\'flag-badge-' + id + '\' style=\'position: absolute; top: 8px; left: 8px; z-index: 10;\'>' + (newFlag === 1 ? '<span class=\'material-symbols-outlined icon-filled\' style=\'color: #2ecc71; filter: drop-shadow(0 2px 4px rgba(0,0,0,0.5));\'>check_circle</span>' : (newFlag === -1 ? '<span class=\'material-symbols-outlined icon-filled\' style=\'color: #e74c3c; filter: drop-shadow(0 2px 4px rgba(0,0,0,0.5));\'>cancel</span>' : '')) + '</div>';
							});
					}
				}
			}
			break;
		case 'w': case 'W':
			if (document.activeElement.classList.contains('image-card') && !isCarouselOpen) {
				const img = document.activeElement.querySelector('img');
				if (img && img.dataset.id) {
					const id = img.dataset.id;
					const badge = document.getElementById('flag-badge-' + id);
					let currentFlag = 0;
					if (badge && badge.innerHTML.includes('check_circle')) currentFlag = 1;
					else if (badge && badge.innerHTML.includes('cancel')) currentFlag = -1;
					
					let newFlag = currentFlag === -1 ? 0 : -1;
					const sidebarContainer = document.getElementById('flag-container-' + id);
					if (sidebarContainer) {
						htmx.ajax('POST', `/image/${id}/flag`, {values: {flag: newFlag}, target: '#flag-container-' + id, swap: 'outerHTML'});
					} else {
						fetch(`/image/${id}/flag`, {method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body: `flag=${newFlag}`})
							.then(() => {
								if (badge) badge.outerHTML = '<div id=\'flag-badge-' + id + '\' style=\'position: absolute; top: 8px; left: 8px; z-index: 10;\'>' + (newFlag === 1 ? '<span class=\'material-symbols-outlined icon-filled\' style=\'color: #2ecc71; filter: drop-shadow(0 2px 4px rgba(0,0,0,0.5));\'>check_circle</span>' : (newFlag === -1 ? '<span class=\'material-symbols-outlined icon-filled\' style=\'color: #e74c3c; filter: drop-shadow(0 2px 4px rgba(0,0,0,0.5));\'>cancel</span>' : '')) + '</div>';
							});
					}
				}
			}
			break;
		case 's': case 'S':
			const settingsBtn = document.querySelector('[hx-get="/settings"]');
			if (settingsBtn) settingsBtn.click();
			break;
		case 'u': case 'U':
			const dupBtn = document.querySelector('[hx-get="/gallery/global/duplicates"]');
			if (dupBtn) dupBtn.click();
			break;
		case '+': case '=':
			if (isCarouselOpen) {
				if (e.key === '=') {
					window.dispatchEvent(new CustomEvent('acuity-zoom-reset'));
				} else {
					window.dispatchEvent(new CustomEvent('acuity-zoom-in'));
				}
				e.preventDefault();
			} else if (!['INPUT', 'TEXTAREA'].includes(document.activeElement.tagName)) {
				const th = document.querySelector('input[name="threshold"]');
				if (th) {
					let v = parseFloat(th.value);
					if (v < parseFloat(th.max)) {
						th.value = (v + parseFloat(th.step || 0.05)).toFixed(2);
						th.dispatchEvent(new Event('input', {bubbles: true}));
						th.dispatchEvent(new Event('change', {bubbles: true}));
						if (th.closest('form') && !th.closest('form').hasAttribute('hx-trigger')) htmx.trigger(th.closest('form'), 'submit');
					}
				}
			}
			break;
		case '-': case '_':
			if (isCarouselOpen) {
				window.dispatchEvent(new CustomEvent('acuity-zoom-out'));
				e.preventDefault();
			} else if (!['INPUT', 'TEXTAREA'].includes(document.activeElement.tagName)) {
				const th = document.querySelector('input[name="threshold"]');
				if (th) {
					let v = parseFloat(th.value);
					if (v > parseFloat(th.min)) {
						th.value = (v - parseFloat(th.step || 0.05)).toFixed(2);
						th.dispatchEvent(new Event('input', {bubbles: true}));
						th.dispatchEvent(new Event('change', {bubbles: true}));
						if (th.closest('form') && !th.closest('form').hasAttribute('hx-trigger')) htmx.trigger(th.closest('form'), 'submit');
					}
				}
			}
			break;
		case 'g': case 'G':
			const firstGallery = document.querySelector('.nav-item[hx-get^="/gallery/"]');
			if (firstGallery) firstGallery.focus();
			break;
        case 'c': case 'C':
            window.dispatchEvent(new CustomEvent('acuity-copy'));
            break;
        case 'm': case 'M':
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

function moveSidebarFocus(dy) {
	// Only select visible nav-items in the sidebar
	const navItems = Array.from(document.querySelectorAll('.sidebar-left .nav-item')).filter(el => el.offsetWidth > 0 && el.offsetHeight > 0);
	if (navItems.length === 0) return;
	
	let currentIdx = navItems.indexOf(document.activeElement);
	if (currentIdx === -1) {
		navItems[0].focus();
		return;
	}
	
	let nextIdx = currentIdx + dy;
	if (nextIdx < 0) nextIdx = navItems.length - 1;
	if (nextIdx >= navItems.length) nextIdx = 0;
	
	navItems[nextIdx].focus();
}

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
