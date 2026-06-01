<script lang="ts">
	import { onMount } from 'svelte';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import SectionCards from '$lib/components/SectionCards.svelte';
	import RecentRunsTable from '$lib/components/RecentRunsTable.svelte';
	import RecentScenariosSection from '$lib/components/RecentScenariosSection.svelte';
	import RecentPacksSection from '$lib/components/RecentPacksSection.svelte';
	import { runs, activeRuns, loadRuns } from '$lib/stores/runs';
	import { loadScenarioPage } from '$lib/stores/scenarios';
	import { packs, loadPacks } from '$lib/stores/packs';
	import { getRuleCoverage } from '$lib/api/client';
	import type { SavedScenario } from '$lib/types';

	let loading = $state(true);
	let error = $state('');
	let ruleCoveragePercent = $state(0);
	let recentScenarios = $state<SavedScenario[]>([]);
	let savedScenariosTotal = $state(0);

	const recentRuns = $derived($runs.slice(0, 5));
	const recentPacks = $derived($packs.slice(0, 5));

	onMount(async () => {
		try {
			const [, scenarioPage] = await Promise.all([
				loadRuns(),
				loadScenarioPage(1, 5, {}),
				loadPacks()
			]);
			recentScenarios = scenarioPage.scenarios;
			savedScenariosTotal = scenarioPage.total;

			// Load rule coverage (non-blocking, may fail if no elastic connector)
			try {
				const coverage = await getRuleCoverage();
				ruleCoveragePercent = Math.round(coverage.summary.coveragePercent);
			} catch {
				// Rule coverage may not be available
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	});
</script>

<div class="flex flex-1 flex-col">
	<div class="@container/main flex flex-1 flex-col gap-2">
		<div class="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
			{#if error}
				<div class="px-4 lg:px-6">
					<Alert.Root variant="destructive">
						<Alert.Description>{error}</Alert.Description>
					</Alert.Root>
				</div>
			{/if}

			{#if loading}
				<div class="grid grid-cols-1 gap-4 px-4 lg:px-6 @xl/main:grid-cols-2 @5xl/main:grid-cols-4">
					{#each Array(4) as _}
						<Skeleton class="h-32 w-full rounded-xl" />
					{/each}
				</div>
				<div class="px-4 lg:px-6 space-y-3">
					<Skeleton class="h-6 w-32" />
					{#each Array(5) as _}
						<Skeleton class="h-12 w-full" />
					{/each}
				</div>
			{:else}
				<SectionCards
					totalRuns={$runs.length}
					{ruleCoveragePercent}
					activeRuns={$activeRuns.length}
					savedScenarios={savedScenariosTotal}
				/>

				<div class="grid grid-cols-1 gap-4 px-4 lg:px-6 @xl/main:grid-cols-3">
					<RecentRunsTable runs={recentRuns} />
					<RecentScenariosSection scenarios={recentScenarios} />
					<RecentPacksSection packs={recentPacks} />
				</div>
			{/if}
		</div>
	</div>
</div>
