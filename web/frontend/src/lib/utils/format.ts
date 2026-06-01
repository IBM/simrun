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
