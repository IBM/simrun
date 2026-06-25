<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import ScenarioEditor from '$lib/components/ScenarioEditor.svelte';
	import ScheduleDialog from '$lib/components/ScheduleDialog.svelte';
	import { getAssessment, updateAssessment, runAssessment } from '$lib/api/client';
	import { parseScenarioYAML } from '$lib/utils/yaml-parser';
	import { createEmptyTarget } from '$lib/utils/yaml-generator';
	import type { FormScenario, FormTarget } from '$lib/utils/yaml-generator';
	import type { Assessment } from '$lib/types';

	let id = $derived($page.params.id!);

	let loading = $state(true);
	let loadError = $state('');
	let assessment = $state<Assessment | null>(null);
	let initialScenarios = $state<FormScenario[]>([]);
	let initialTarget = $state<FormTarget>(createEmptyTarget());
	let initialYaml = $state('');
	let builderSupported = $state(true);

	let scheduleDialogOpen = $state(false);

	onMount(async () => {
		try {
			const a = await getAssessment(id);
			assessment = a;
			initialYaml = a.yaml;
			const parseResult = parseScenarioYAML(a.yaml);
			if (parseResult.success && parseResult.builderSupported) {
				initialScenarios = parseResult.scenarios || [];
				initialTarget = parseResult.target || createEmptyTarget();
				builderSupported = true;
			} else {
				builderSupported = false;
			}
		} catch (e) {
			loadError = e instanceof Error ? e.message : 'Failed to load assessment';
		} finally {
			loading = false;
		}
	});

	async function handleSave(name: string, yaml: string, opts: { run?: boolean }) {
		if (!assessment) return;
		await updateAssessment(assessment.id, name, yaml, assessment.type);
		if (opts.run) {
			const resp = await runAssessment(assessment.id);
			await goto(`/runs/${resp.runId}`);
		}
	}

	function handleCancel() {
		goto('/assessments');
	}

	function openSchedule() {
		scheduleDialogOpen = true;
	}
</script>

{#if loading}
	<div class="mx-auto max-w-5xl px-6 py-10 space-y-4">
		<Skeleton class="h-6 w-48" />
		<Skeleton class="h-10 w-96" />
		<Skeleton class="h-32 w-full" />
		<Skeleton class="h-32 w-full" />
	</div>
{:else if loadError}
	<div class="mx-auto max-w-3xl px-6 py-10">
		<Alert.Root variant="destructive">
			<Alert.Description>{loadError}</Alert.Description>
		</Alert.Root>
		<div class="mt-4">
			<Button variant="outline" onclick={handleCancel}>Back to assessments</Button>
		</div>
	</div>
{:else if assessment}
	<ScenarioEditor
		mode="edit"
		type={assessment.type}
		initialName={assessment.name}
		{initialScenarios}
		{initialTarget}
		{initialYaml}
		initialBuilderSupported={builderSupported}
		onsave={handleSave}
		oncancel={handleCancel}
		onschedule={openSchedule}
	/>

	<ScheduleDialog
		bind:open={scheduleDialogOpen}
		{assessment}
		onclose={() => (scheduleDialogOpen = false)}
		onsuccess={() => (scheduleDialogOpen = false)}
	/>
{/if}
