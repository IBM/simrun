<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import ScenarioEditor from '$lib/components/ScenarioEditor.svelte';
	import { saveAssessment, runAssessment } from '$lib/api/client';
	import type { ScenarioType } from '$lib/types';

	const validTypes: ScenarioType[] = ['standard', 'explore', 'collect'];

	let selected = $derived.by<ScenarioType | null>(() => {
		const t = $page.url.searchParams.get('type');
		return t && (validTypes as string[]).includes(t) ? (t as ScenarioType) : null;
	});

	onMount(() => {
		if (!selected) {
			goto('/assessments?new=1', { replaceState: true });
		}
	});

	async function handleSave(name: string, yaml: string, opts: { run?: boolean }) {
		const saved = await saveAssessment(name, yaml, selected ?? 'standard');
		if (opts.run) {
			const resp = await runAssessment(saved.id);
			await goto(`/runs/${resp.runId}`);
		} else {
			await goto(`/assessments/${saved.id}/edit`);
		}
	}

	function handleCancel() {
		goto('/assessments');
	}
</script>

{#if selected}
	<ScenarioEditor mode="create" type={selected} onsave={handleSave} oncancel={handleCancel} />
{/if}
