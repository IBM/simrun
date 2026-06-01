// Preset cron expressions for quick selection.
export const cronPresets = [
	{ label: 'Every 5 minutes', value: '*/5 * * * *' },
	{ label: 'Every 15 minutes', value: '*/15 * * * *' },
	{ label: 'Every 30 minutes', value: '*/30 * * * *' },
	{ label: 'Every hour', value: '0 * * * *' },
	{ label: 'Every 6 hours', value: '0 */6 * * *' },
	{ label: 'Daily at midnight', value: '0 0 * * *' },
	{ label: 'Daily at 9:00 AM', value: '0 9 * * *' },
	{ label: 'Weekdays at 9:00 AM', value: '0 9 * * 1-5' },
	{ label: 'Weekly on Monday', value: '0 0 * * 1' },
	{ label: 'Monthly on 1st', value: '0 0 1 * *' }
];

// Parse a cron expression into a human-readable description.
export function describeCronExpression(expression: string): string {
	const parts = expression.trim().split(/\s+/);
	if (parts.length !== 5) return 'Custom schedule';

	const [minute, hour, day, month, weekday] = parts;

	// Match known presets first
	const preset = cronPresets.find((p) => p.value === expression.trim());
	if (preset) return preset.label;

	// Build description from parts
	let desc = '';

	if (minute === '*') {
		desc += 'Every minute';
	} else if (minute.startsWith('*/')) {
		desc += `Every ${minute.slice(2)} minutes`;
	} else {
		desc += `At minute ${minute}`;
	}

	if (hour !== '*') {
		if (hour.startsWith('*/')) {
			desc += ` of every ${hour.slice(2)} hours`;
		} else {
			const h = parseInt(hour);
			const ampm = h >= 12 ? 'PM' : 'AM';
			const h12 = h % 12 || 12;
			desc = `At ${h12}:${minute.padStart(2, '0')} ${ampm}`;
		}
	}

	if (day !== '*') {
		desc += ` on day ${day}`;
	}

	if (month !== '*') {
		const months = [
			'Jan',
			'Feb',
			'Mar',
			'Apr',
			'May',
			'Jun',
			'Jul',
			'Aug',
			'Sep',
			'Oct',
			'Nov',
			'Dec'
		];
		const monthNum = parseInt(month) - 1;
		if (monthNum >= 0 && monthNum < 12) {
			desc += ` in ${months[monthNum]}`;
		}
	}

	if (weekday !== '*') {
		const days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
		if (weekday.includes('-')) {
			const [start, end] = weekday.split('-').map(Number);
			if (start >= 0 && end < 7) {
				desc += ` on ${days[start]}-${days[end]}`;
			}
		} else {
			const dayNum = parseInt(weekday);
			if (dayNum >= 0 && dayNum < 7) {
				desc += ` on ${days[dayNum]}`;
			}
		}
	}

	return desc;
}

// Validate a cron expression. Returns an error string or null if valid.
export function validateCronExpression(expression: string): string | null {
	const parts = expression.trim().split(/\s+/);

	if (parts.length !== 5) {
		return 'Cron expression must have 5 parts (minute hour day month weekday)';
	}

	const [minute, hour, day, month, weekday] = parts;

	const isValidPart = (part: string, min: number, max: number): boolean => {
		if (part === '*') return true;
		if (part.startsWith('*/')) {
			const interval = parseInt(part.slice(2));
			return !isNaN(interval) && interval > 0;
		}
		if (part.includes('-')) {
			const [start, end] = part.split('-').map(Number);
			return !isNaN(start) && !isNaN(end) && start >= min && end <= max && start <= end;
		}
		if (part.includes(',')) {
			return part.split(',').every((p) => {
				const num = parseInt(p);
				return !isNaN(num) && num >= min && num <= max;
			});
		}
		const num = parseInt(part);
		return !isNaN(num) && num >= min && num <= max;
	};

	if (!isValidPart(minute, 0, 59)) return 'Invalid minute (0-59)';
	if (!isValidPart(hour, 0, 23)) return 'Invalid hour (0-23)';
	if (!isValidPart(day, 1, 31)) return 'Invalid day (1-31)';
	if (!isValidPart(month, 1, 12)) return 'Invalid month (1-12)';
	if (!isValidPart(weekday, 0, 6)) return 'Invalid weekday (0-6, 0=Sunday)';

	return null;
}
