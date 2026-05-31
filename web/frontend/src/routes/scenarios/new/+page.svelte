<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import ScenarioEditor from '$lib/components/ScenarioEditor.svelte';
	import { saveScenario, runScenario } from '$lib/api/client';
	import type { ScenarioType } from '$lib/types';

	const validTypes: ScenarioType[] = ['standard', 'explore', 'collect'];

	let selected = $derived.by<ScenarioType | null>(() => {
		const t = $page.url.searchParams.get('type');
		return t && (validTypes as string[]).includes(t) ? (t as ScenarioType) : null;
	});

	onMount(() => {
		if (!selected) {
			goto('/scenarios?new=1', { replaceState: true });
		}
	});

	async function handleSave(name: string, yaml: string, opts: { run?: boolean }) {
		const saved = await saveScenario(name, yaml, selected ?? 'standard');
		if (opts.run) {
			const resp = await runScenario(saved.id);
			await goto(`/assessments/${resp.runId}`);
		} else {
			await goto(`/scenarios/${saved.id}/edit`);
		}
	}

	function handleCancel() {
		goto('/scenarios');
	}
</script>

{#if selected}
	<ScenarioEditor mode="create" type={selected} onsave={handleSave} oncancel={handleCancel} />
{/if}
