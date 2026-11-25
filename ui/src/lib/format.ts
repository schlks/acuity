export function formatSize(bytes: number | string): string {
	const num = Number(bytes);
	if (!num) return '';
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

const resolutionLabels: Record<string, string> = {
	'1280x720': 'HD',
	'1920x1080': 'FHD',
	'2560x1440': 'QHD',
	'3840x2160': '4K',
	'7680x4320': '8K'
};

export function formatResolution(width?: number, height?: number): string {
	if (!width || !height) return '';
	const key = `${Math.max(width, height)}x${Math.min(width, height)}`;

	return resolutionLabels[key] ?? `${width} x ${height}`
}

const dateFormat = new Intl.DateTimeFormat('de-DE', {
			day: '2-digit',
			month: 'short',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});

export function formatDate(dateStr: string): string {
	const date = new Date(dateStr);
	if (isNaN(date.getTime())) return dateStr;

	return dateFormat.format(date);
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

export function formatMakeModel(make?: string, model?: string): string {
	const cleanMake = (make ?? '').trim();
	const cleanModel = (model ?? '').trim();
	if (!cleanModel) return cleanMake || '--';

	const brand = cleanMake.split(/\s+/)[0].toLowerCase();
	if (!brand || cleanModel.toLowerCase().startsWith(brand)) return cleanModel;
	return `${cleanMake} ${cleanModel}`;
}

const sharpnessFormat = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 0 });

export function formatSharpness(s?: number): string {
	if (s == null) return '...';
	if (s < 0) return '--';
	return sharpnessFormat.format(s);
}
