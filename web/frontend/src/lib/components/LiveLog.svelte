<script lang="ts">
	import type { ScenarioLogData } from '$lib/types';
	import * as ScrollArea from '$lib/components/ui/scroll-area/index.js';
	import { onMount, tick } from 'svelte';

	let { logs }: { logs: ScenarioLogData[] } = $props();
	let scrollContainer: HTMLDivElement;

	function levelColor(level: string): string {
		switch (level) {
			case 'error':
				return 'text-red-500';
			case 'warn':
			case 'warning':
				return 'text-yellow-500';
			default:
				return 'text-foreground';
		}
	}

	$effect(() => {
		if (logs.length && scrollContainer) {
			tick().then(() => {
				scrollContainer.scrollTop = scrollContainer.scrollHeight;
			});
		}
	});
</script>

<div
	bind:this={scrollContainer}
	class="h-[400px] overflow-auto rounded-md border border-border bg-zinc-950 p-4 font-mono text-sm"
>
	{#each logs as log}
		<div class={`${levelColor(log.level)} whitespace-pre-wrap`}>
			<span class="text-muted-foreground">[{log.level.toUpperCase().padEnd(5)}]</span>
			<span class="text-muted-foreground">[{log.scenarioName}]</span>
			{log.message}
		</div>
	{/each}
	{#if logs.length === 0}
		<div class="text-muted-foreground">Waiting for logs...</div>
	{/if}
</div>
