import { writable } from 'svelte/store';
import type { SavedScenario } from '$lib/types';
import { listScenarios, type ScenarioFilters } from '$lib/api/client';

// Holds the current page of saved scenarios for the list view. Other
// consumers that need a different slice (e.g. dashboard widgets, picker
// dialogs) should call `listScenarios()` directly with their own page
// size rather than read this store.
export const scenarios = writable<SavedScenario[]>([]);

export async function loadScenarioPage(
	page = 1,
	perPage = 50,
	filters: ScenarioFilters = {}
) {
	const data = await listScenarios(page, perPage, filters);
	scenarios.set(data.scenarios);
	return data;
}
