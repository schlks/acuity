document.addEventListener('alpine:init', () => {
    Alpine.data('searchBar', () => ({
			query: '',
			isDragging: false,
			imageUrl: null,
			handleFile(file) {
				if (!file || !file.type.startsWith('image/')) return;
				this.imageUrl = URL.createObjectURL(file);
				const dt = new DataTransfer();
				dt.items.add(file);
				this.$refs.fileInput.files = dt.files;
				this.query = ''; // Suchtext leeren, da jetzt ein Bild da ist
				this.$nextTick(() => {
					this.$refs.searchInput.closest('form').requestSubmit();
				    
});
			},
			handlePaste(event) {
				const items = (event.clipboardData || event.originalEvent.clipboardData).items;
				for (let index in items) {
					const item = items[index];
					if (item.kind === 'file' && item.type.startsWith('image/')) {
						this.handleFile(item.getAsFile());
						return;
					}
				}
			},
			removeImage() {
				this.imageUrl = null;
				this.$refs.fileInput.value = '';
			}
		}));

    Alpine.data('carousel', () => ({
			images: [],
			currentIndex: 0,
			direction: 1,
			showInfo: false,
			scale: 1,
			panX: 0,
			panY: 0,
			isDragging: false,
			startX: 0,
			startY: 0,
			isLoading: false,
			
			init() {
				//this.$watch('currentIndex', () => { this.isLoading = true; });
				this.$watch('currentIndex', () => {
					this.isLoading = true;
					this.$nextTick(() => {
						let currentImg = this.$el.querySelector(`img[src='${this.images[this.currentIndex]?.src}']`);
						if (currentImg && currentImg.complete) {
							this.isLoading = false;
						}
					    
});
				    
});
			},
			
			resetZoom() {
				this.scale = 1;
				this.panX = 0;
				this.panY = 0;
			},
			
			open(id, groupSelector) {
				const selector = groupSelector ? `${groupSelector} .image-card img` : '#main-content .image-card img';
				const imageEls = Array.from(document.querySelectorAll(selector));
				this.images = imageEls.map(img => ({ src: img.src, id: img.dataset.id, rating: parseInt(img.dataset.rating) || 0, flag: parseInt(img.dataset.flag) || 0 }));
				
				const idx = this.images.findIndex(img => img.id === id);
				this.currentIndex = idx !== -1 ? idx : 0;
				
				this.resetZoom();
				this.showInfo = false;
				document.querySelector('.carousel-modal').showModal();
				document.body.style.overflow = 'hidden';
			},
			
			close() {
				document.querySelector('.carousel-modal').close();
				document.body.style.overflow = '';
				
				if (this.images.length > 0 && this.images[this.currentIndex]) {
					const currentId = this.images[this.currentIndex].id;
					setTimeout(() => {
						const targetImg = document.querySelector(`.image-card img[data-id='${currentId}']`);
						if (targetImg) {
							const card = targetImg.closest('.image-card');
							if (card) {
								card.scrollIntoView({ behavior: 'instant', block: 'center'     
});
								card.focus({ preventScroll: true     
});
								document.body.classList.add('keyboard-navigating');
							}
						}
					}, 10);
				}
			},
			
			next() {
				this.direction = 1;
				if (this.currentIndex < this.images.length - 1) {
					this.currentIndex++;
					this.resetZoom();
					if(this.showInfo) {
						htmx.ajax('GET', '/image/' + this.images[this.currentIndex].id, {target: '#carousel-info-container'    
});
					}
				} else {
					const nextBtn = document.getElementById('next-page-btn');
					const infiniteTrigger = document.getElementById('infinite-scroll-trigger');
					if (nextBtn) {
						this.isLoading = true;
						sessionStorage.setItem('acuity-focus', 'first');
						sessionStorage.setItem('acuity-carousel-reopen', 'first');
						nextBtn.click();
					} else if (infiniteTrigger) {
						this.isLoading = true;
						htmx.trigger(infiniteTrigger, 'load-more');
					}
				}
			},
			
			updateImages() {
				let imageEls = Array.from(document.querySelectorAll('#main-content .image-card img'));
				if (imageEls.length > 0) {
					let lastId = null;
					if (this.images.length > 0 && this.images[this.currentIndex]) {
						lastId = this.images[this.currentIndex].id;
					}
					
					let oldLength = this.images.length;
					this.images = imageEls.map(img => ({ src: img.src, id: img.dataset.id, rating: parseInt(img.dataset.rating) || 0, flag: parseInt(img.dataset.flag) || 0 }));
					
					if (lastId) {
						let foundIdx = this.images.findIndex(img => img.id === lastId);
						if (foundIdx !== -1) {
							this.currentIndex = foundIdx;
							
							// If we were waiting for infinite scroll to load more
							if (this.isLoading) {
								if (this.images.length > oldLength && this.currentIndex < this.images.length - 1) {
									this.$nextTick(() => { this.next();     
});
								} else {
									this.isLoading = false; // Reached the very end
								}
							}
						}
					}
				}
			},
			
			prev() {
				this.direction = -1;
				if (this.currentIndex > 0) {
					this.currentIndex--;
					this.resetZoom();
					if(this.showInfo) {
						htmx.ajax('GET', '/image/' + this.images[this.currentIndex].id, {target: '#carousel-info-container'    
});
					}
				} else {
					const prevBtn = document.getElementById('prev-page-btn');
					if (prevBtn) {
						this.isLoading = true;
						sessionStorage.setItem('acuity-focus', 'last');
						sessionStorage.setItem('acuity-carousel-reopen', 'last');
						prevBtn.click();
					}
				}
			},
			clampPan(pan, clientSize, windowSize) {
				const scaledSize = clientSize * this.scale;
				if (scaledSize <= windowSize) return 0;
				const maxPan = (scaledSize - windowSize) / 2;
				return Math.max(-maxPan, Math.min(maxPan, pan));
			},
			
			handleWheel(e) {
				e.preventDefault();
				if (this.isDragging) return;
				
				const delta = e.deltaY > 0 ? -0.1 : 0.1;
				const newScale = Math.max(1, Math.min(this.scale * (1 + delta), 50));
				
				const img = this.$el.querySelector('.carousel-image');
				if (!img) return;

				const mouseX = e.clientX - (window.innerWidth / 2);
				const mouseY = e.clientY - (window.innerHeight / 2);
				
				const scaleRatio = newScale / this.scale;
				
				let newPanX = mouseX - (mouseX - this.panX) * scaleRatio;
				let newPanY = mouseY - (mouseY - this.panY) * scaleRatio;
				
				this.scale = newScale;
				
				if (this.scale === 1) {
					this.panX = 0;
					this.panY = 0;
				} else {
					this.panX = this.clampPan(newPanX, img.clientWidth, window.innerWidth);
					this.panY = this.clampPan(newPanY, img.clientHeight, window.innerHeight);
				}
			},
			
			startDrag(e) {
				if (this.scale <= 1) return;
				e.preventDefault();
				this.isDragging = true;
				this.startX = e.clientX - this.panX;
				this.startY = e.clientY - this.panY;
			},
			
			doDrag(e) {
				if (!this.isDragging) return;
				
				let newPanX = e.clientX - this.startX;
				let newPanY = e.clientY - this.startY;
				
				const img = this.$el.querySelector('.carousel-image');
				if (img) {
					this.panX = this.clampPan(newPanX, img.clientWidth, window.innerWidth);
					this.panY = this.clampPan(newPanY, img.clientHeight, window.innerHeight);
				} else {
					this.panX = newPanX;
					this.panY = newPanY;
				}
			},
			
			stopDrag() {
				this.isDragging = false;
			},
			
			deleteCurrent() {
				if(!this.images[this.currentIndex]) return;
				if(!confirm('Are you sure you want to delete this image? This will also delete the file from your disk!')) return;
				let img = this.images[this.currentIndex];
				fetch('/image/' + img.id + '/delete', {method: 'POST'}).then(() => {
					let prevId = img.id;
					window.dispatchEvent(new CustomEvent('acuity-remove-selection', {detail: {id: prevId}}));
					
					this.images = this.images.filter(i => i.id !== prevId);
					if(this.images.length === 0) {
						const nextBtn = document.getElementById('next-page-btn');
						if (nextBtn) {
							this.isLoading = true;
							sessionStorage.setItem('acuity-focus', 'first');
							sessionStorage.setItem('acuity-carousel-reopen', 'first');
							nextBtn.click();
						} else {
							this.close();
						}
					} else {
						if (this.currentIndex >= this.images.length) {
							const nextBtn = document.getElementById('next-page-btn');
							if (nextBtn) {
								this.isLoading = true;
								sessionStorage.setItem('acuity-focus', 'first');
								sessionStorage.setItem('acuity-carousel-reopen', 'first');
								nextBtn.click();
								return;
							} else {
								this.currentIndex = this.images.length - 1;
							}
						}
						this.isLoading = true;
						if(this.showInfo) {
							htmx.ajax('GET', '/image/' + this.images[this.currentIndex].id, {target: '#carousel-info-container'    
});
						}
						if(document.querySelector('body').__x?.$data?.showDetailPanel) {
							htmx.ajax('GET', '/image/' + this.images[this.currentIndex].id, {target: '#image-detail-container'    
});
						}
					}
				    
});
			}
		}));

    Alpine.data('culling', () => ({
		images: [],
		currentIndex: 0,
		direction: 1,
		isLoading: false,
		scale: 1,
		panX: 0,
		panY: 0,
		isDragging: false,
		startX: 0,
		startY: 0,
		
		init() {
			this.$watch('currentIndex', () => {
				this.isLoading = true;
				this.resetZoom();
				this.$nextTick(() => {
					let currentImg = this.$el.querySelector(`img[src='${this.images[this.currentIndex]?.src}']`);
					if (currentImg && currentImg.complete) {
						this.isLoading = false;
					}
				    
});
			    
});
		},
		
		resetZoom() {
			this.scale = 1;
			this.panX = 0;
			this.panY = 0;
			this.isDragging = false;
		},
		
		clampPan(pan, clientSize, windowSize) {
			const scaledSize = clientSize * this.scale;
			if (scaledSize <= windowSize) return 0;
			const maxPan = (scaledSize - windowSize) / 2;
			return Math.max(-maxPan, Math.min(maxPan, pan));
		},
		
		handleWheel(e) {
			e.preventDefault();
			if (this.isDragging) return;
			
			const delta = e.deltaY > 0 ? -0.1 : 0.1;
			const newScale = Math.max(1, Math.min(this.scale * (1 + delta), 50));
			
			const img = this.$el.querySelector('.carousel-image');
			if (!img) return;

			const mouseX = e.clientX - (window.innerWidth / 2);
			const mouseY = e.clientY - (window.innerHeight / 2);
			
			const scaleRatio = newScale / this.scale;
			
			let newPanX = mouseX - (mouseX - this.panX) * scaleRatio;
			let newPanY = mouseY - (mouseY - this.panY) * scaleRatio;
			
			this.scale = newScale;
			
			if (this.scale === 1) {
				this.panX = 0;
				this.panY = 0;
			} else {
				this.panX = this.clampPan(newPanX, img.clientWidth, window.innerWidth);
				this.panY = this.clampPan(newPanY, img.clientHeight, window.innerHeight);
			}
		},
		
		startDrag(e) {
			if (this.scale <= 1) return;
			e.preventDefault();
			this.isDragging = true;
			this.startX = e.clientX - this.panX;
			this.startY = e.clientY - this.panY;
		},
		
		doDrag(e) {
			if (!this.isDragging) return;
			
			let newPanX = e.clientX - this.startX;
			let newPanY = e.clientY - this.startY;
			
			const img = this.$el.querySelector('.carousel-image');
			if (img) {
				this.panX = this.clampPan(newPanX, img.clientWidth, window.innerWidth);
				this.panY = this.clampPan(newPanY, img.clientHeight, window.innerHeight);
			} else {
				this.panX = newPanX;
				this.panY = newPanY;
			}
		},
		
		stopDrag() {
			this.isDragging = false;
		},
		
		open() {
			const imageEls = Array.from(document.querySelectorAll('#main-content .image-card img'));
			if (imageEls.length > 0) {
				let lastId = null;
				if (this.images.length > 0 && this.images[this.currentIndex]) {
					lastId = this.images[this.currentIndex].id;
				}
				
				this.images = imageEls.map(img => ({ src: img.src, id: img.dataset.id, flag: parseInt(img.dataset.flag) || 0 }));
				
				let foundIdx = -1;
				if (lastId) {
					foundIdx = this.images.findIndex(img => img.id === lastId);
				}
				
				this.currentIndex = foundIdx !== -1 ? foundIdx : 0;
				this.resetZoom();
				document.getElementById('culling-modal').showModal();
				document.body.style.overflow = 'hidden';
			}
		},
		updateImages() {
			let imageEls = Array.from(document.querySelectorAll('.image-card img'));
			if (imageEls.length > 0) {
				let lastId = null;
				if (this.images.length > 0 && this.images[this.currentIndex]) {
					lastId = this.images[this.currentIndex].id;
				}
				
				let oldLength = this.images.length;
				this.images = imageEls.map(img => ({ src: img.src, id: img.dataset.id, flag: parseInt(img.dataset.flag) || 0 }));
				
				if (lastId) {
					let foundIdx = this.images.findIndex(img => img.id === lastId);
					if (foundIdx !== -1) {
						this.currentIndex = foundIdx;
						
						// If we were waiting for infinite scroll to load more
						if (this.isLoading) {
							if (this.images.length > oldLength && this.currentIndex < this.images.length - 1) {
								this.$nextTick(() => { this.next();     
});
							} else {
								this.isLoading = false; // Reached the very end
							}
						}
					}
				}
			}
		},
		close() {
			document.getElementById('culling-modal').close();
			document.body.style.overflow = '';
			
			if (this.images.length > 0 && this.images[this.currentIndex]) {
				const currentId = this.images[this.currentIndex].id;
				setTimeout(() => {
					const targetImg = document.querySelector(`.image-card img[data-id='${currentId}']`);
					if (targetImg) {
						const card = targetImg.closest('.image-card');
						if (card) {
							card.scrollIntoView({ behavior: 'instant', block: 'center'     
});
							card.focus({ preventScroll: true     
});
							document.body.classList.add('keyboard-navigating');
						}
					}
				}, 10);
			}
		},
		flag(val) {
			if(this.images.length === 0) return;
			let img = this.images[this.currentIndex];
			
			if (img.flag === val) {
				val = 0;
			}
			img.flag = val;
			fetch('/image/' + img.id + '/flag', { method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body: 'flag=' + val })
				.then(() => {
					window.dispatchEvent(new CustomEvent('flag-updated', {detail: {id: img.id, flag: val}}));
					this.next();
				    
});
		},
		
		next() {
			this.direction = 1;
			if (this.currentIndex < this.images.length - 1) {
				this.currentIndex++;
			} else {
				const nextBtn = document.getElementById('next-page-btn');
				const infiniteTrigger = document.getElementById('infinite-scroll-trigger');
				if (nextBtn) {
					this.isLoading = true;
					sessionStorage.setItem('acuity-focus', 'first');
					sessionStorage.setItem('acuity-culling-reopen', 'first');
					nextBtn.click();
				} else if (infiniteTrigger) {
					this.isLoading = true;
					htmx.trigger(infiniteTrigger, 'load-more');
				}
			}
		},
		
		prev() {
			this.direction = -1;
			if (this.currentIndex > 0) {
				this.currentIndex--;
			} else {
				const prevBtn = document.getElementById('prev-page-btn');
				if (prevBtn) {
					this.isLoading = true;
					sessionStorage.setItem('acuity-focus', 'last');
					sessionStorage.setItem('acuity-culling-reopen', 'last');
					prevBtn.click();
				}
			}
		},
		
		deleteCurrent() {
			if(!this.images[this.currentIndex]) return;
			if(!confirm('Are you sure you want to delete this image? This will also delete the file from your disk!')) return;
			let img = this.images[this.currentIndex];
			fetch('/image/' + img.id + '/delete', {method: 'POST'}).then(() => {
				let prevId = img.id;
				window.dispatchEvent(new CustomEvent('acuity-remove-selection', {detail: {id: prevId}}));
				
				this.images = this.images.filter(i => i.id !== prevId);
				if(this.images.length === 0) {
					const nextBtn = document.getElementById('next-page-btn');
					if (nextBtn) {
						this.isLoading = true;
						sessionStorage.setItem('acuity-focus', 'first');
						sessionStorage.setItem('acuity-culling-reopen', 'first');
						nextBtn.click();
					} else {
						this.close();
					}
				} else {
					if (this.currentIndex >= this.images.length) {
						const nextBtn = document.getElementById('next-page-btn');
						if (nextBtn) {
							this.isLoading = true;
							sessionStorage.setItem('acuity-focus', 'first');
							sessionStorage.setItem('acuity-culling-reopen', 'first');
							nextBtn.click();
							return;
						} else {
							this.currentIndex = this.images.length - 1;
						}
					}
					this.isLoading = true;
				}
			    
});
		}
	}));
});

    // Global App State
    Alpine.data('globalApp', () => ({
        showDetailPanel: false,
        currentGallery: '',
        currentFolder: '',
        path: '',
        showBrowser: false,
        selectedImages: []
    }));

    // Batch Action Dialog
    Alpine.data('batchActionDialog', () => ({
        isNew: false,
        action: 'move',
        criteria: 'selected',
        
        closeDialog(event, el) {
            if (event.target === el) {
                el.classList.add('closing');
                setTimeout(() => {
                    el.close();
                    el.classList.remove('closing');
                }, 300);
            }
        }
    }));

    // Transfer Dialog
    Alpine.data('transferDialog', () => ({
        transferPath: '',
        showTransferBrowser: false
    }));

    // Infinite Scroll
    Alpine.data('infiniteScroll', () => ({
        init() {
            let observer = new IntersectionObserver((entries) => {
                if (entries[0].isIntersecting) {
                    setTimeout(() => {
                        htmx.trigger(this.$el, 'load-more');
                    }, 50);
                    observer.disconnect();
                }
            }, { root: document.querySelector('.main-area'), rootMargin: '400px'     
});
            observer.observe(this.$el);
        }
    }));

