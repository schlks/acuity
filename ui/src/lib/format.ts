export function formatSize(bytes: number | string): string {
	const num = Number(bytes);
	if (!num || isNaN(num) || num <= 0) return '';
	const unit = 1024.0;
	if (num < unit) return `${num} B`;

	const units = ['K', 'M', 'G', 'T', 'P', 'E'];
	let div = unit;
	let exp = 0;

	for (let n = num / unit; n >= unit; n /= unit) {
		div *= unit;
		exp++;
	}

	return `${(num / div).toFixed(1)} ${units[exp]}B`;
}

export function formatResolution(pixels: number | string, aspectRatio?: number): string {
	const num = Number(pixels);
	if (!num || isNaN(num) || num <= 0) return '';
	if (!aspectRatio || aspectRatio === 0) {
		return `${(num / 1_000_000).toFixed(1)} MP`;
	}

	const h = Math.round(Math.sqrt(num / aspectRatio));
	const w = Math.round(num / h);

	const resMap: Record<number, string> = {
		720: 'HD',
		1080: 'FHD',
		1440: 'UHD',
		2160: '4K',
		4320: '8K'
	};

	return resMap[h] ?? `${Math.round(w)} x ${Math.round(h)}`;
}

export function formatDate(dateStr: string): string {
	const date = new Date(dateStr);
	if (isNaN(date.getTime())) return dateStr;

	return new Intl.DateTimeFormat('de-DE', {
		day: '2-digit',
		month: 'short',
		year: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	}).format(date);
}

export function formatExt(ext: string): string {
	return ext.replace(/^\./, '').toUpperCase();
}

export function formatAperture(a: string): string {
	const trimmed = (a || '').trim();
	if (!trimmed || trimmed === '0' || trimmed === '0.0' || trimmed === '0.00') return '--';
	return trimmed.replace(/^f\//i, '');
}

export function formatShutter(s: string): string {
	let clean = (s || '').trim().replace(/s$/i, '');
	if (!clean || clean === '0' || clean === '0.0' || clean === '0.00') return '--';

	if (!clean.includes('/')) {
		const f = parseFloat(clean);
		if (!isNaN(f) && f > 0) {
			if (f < 1.0) {
				const denominator = Math.round(1.0 / f);
				clean = `1/${denominator}`;
			} else {
				clean = `${f}`;
			}
		}
	}

	return clean.endsWith('s') ? clean : `${clean}s`;
}

export function formatFocalLength(f: string): string {
	const trimmed = (f || '').trim();
	if (!trimmed || /^0+(\.0+)?(mm)?$/i.test(trimmed)) return '--';
	if (!trimmed.toLowerCase().endsWith('mm')) {
		return `${trimmed}mm`;
	}
	return trimmed;
}

export function formatIso(iso: string): string {
	const trimmed = (iso || '').trim();
	if (!trimmed || trimmed === '0' || trimmed === '0.0' || trimmed === '0.00') return '--';
	return trimmed;
}

const lensMetaRegex = /\b\d+(\.\d+)?mm\b|\bf\/\d+(\.\d+)?\b/gi;

export function formatLens(make: string, model: string): string {
	let cleanMake = (make || 'NA').trim();
	const cleanModel = (model || 'NA').trim();

	if (cleanMake && cleanModel.toLowerCase().startsWith(cleanMake.toLowerCase())) {
		cleanMake = 'NA';
	}

	let res = `${cleanMake} ${cleanModel}`.trim();
	return res.replace(lensMetaRegex, '').replace(/\s+/g, ' ').trim();
}
