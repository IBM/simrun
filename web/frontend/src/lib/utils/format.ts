import type { Run, ScenarioType } from '$lib/types';
import type { BadgeVariant } from '$lib/components/ui/badge/badge.svelte';

export function statusVariant(status: Run['status']): BadgeVariant {
	switch (status) {
		case 'completed':
			return 'success';
		case 'running':
			return 'secondary';
		case 'failed':
			return 'destructive';
		default:
			return 'outline';
	}
}

export function formatDuration(start: string, end: string | null): string {
	if (!end) return 'In progress';
	const durationMs = new Date(end).getTime() - new Date(start).getTime();
	const secs = Math.floor(durationMs / 1000);
	if (secs < 60) return `${secs}s`;
	const mins = Math.floor(secs / 60);
	return `${mins}m ${secs % 60}s`;
}

export function formatTime(dateStr: string): string {
	return new Date(dateStr).toLocaleString();
}

/**
 * Compact, scannable relative time ("just now", "5m ago", "2h ago", "yesterday",
 * "3d ago") falling back to a short date for anything older than a week. Pair with
 * a tooltip showing the absolute time via formatTime().
 */
export function formatRelativeTime(dateStr: string): string {
	const then = new Date(dateStr).getTime();
	if (Number.isNaN(then)) return '--';
	const secs = Math.floor((Date.now() - then) / 1000);
	if (secs < 45) return 'just now';
	const mins = Math.floor(secs / 60);
	if (mins < 60) return `${mins}m ago`;
	const hours = Math.floor(mins / 60);
	if (hours < 24) return `${hours}h ago`;
	const days = Math.floor(hours / 24);
	if (days === 1) return 'yesterday';
	if (days < 7) return `${days}d ago`;
	return new Date(then).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

export function scenarioTypeVariant(type?: ScenarioType | string): 'default' | 'secondary' | 'outline' {
	switch (type) {
		case 'explore':
			return 'secondary';
		case 'collect':
			return 'outline';
		default:
			return 'default';
	}
}

/** Extract the username part of an email for compact display. Returns "anonymous" as-is. */
export function formatUserEmail(email: string): string {
	if (!email || email === 'anonymous') return 'anonymous';
	const atIndex = email.indexOf('@');
	return atIndex > 0 ? email.substring(0, atIndex) : email;
}
