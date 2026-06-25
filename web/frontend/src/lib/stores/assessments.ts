import { writable } from 'svelte/store';
import type { Assessment } from '$lib/types';
import { listAssessments, type ScenarioFilters } from '$lib/api/client';

// Holds the current page of assessments for the list view. Other consumers
// that need a different slice (e.g. dashboard widgets, picker dialogs) should
// call `listAssessments()` directly with their own page size rather than read
// this store.
export const assessments = writable<Assessment[]>([]);

export async function loadAssessmentPage(
	page = 1,
	perPage = 50,
	filters: ScenarioFilters = {}
) {
	const data = await listAssessments(page, perPage, filters);
	assessments.set(data.assessments);
	return data;
}
