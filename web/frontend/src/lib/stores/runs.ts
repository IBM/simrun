import { writable, derived } from 'svelte/store';
import type { Run } from '$lib/types';
import { listRuns, getRun } from '$lib/api/client';

export const runs = writable<Run[]>([]);
export const currentRun = writable<Run | null>(null);

// Loads page 1 of recent runs into the global store. The assessments list
// page maintains its own paginated state and uses listRuns() directly.
export async function loadRuns() {
	const data = await listRuns(1, 50);
	runs.set(data.runs);
}

export async function loadRun(runId: string) {
	const data = await getRun(runId);
	currentRun.set(data);
}

export const activeRuns = derived(runs, ($runs) => $runs.filter((r) => r.status === 'running'));
