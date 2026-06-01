<script lang="ts">
	import type { RunLogEntry } from '$lib/types';
	import { tick } from 'svelte';

	let { entries, class: className = '' }: { entries: RunLogEntry[]; class?: string } = $props();
	let scrollContainer: HTMLDivElement;

	function levelColor(level: string): string {
		switch (level) {
			case 'error':
			case 'fatal':
				return 'text-red-400';
			case 'warn':
			case 'warning':
				return 'text-yellow-400';
			case 'debug':
			case 'trace':
				return 'text-zinc-500';
			default:
				return 'text-zinc-300';
		}
	}

	function shortLevel(level: string): string {
		switch (level) {
			case 'warning':
				return 'WARN';
			case 'info':
				return 'INFO';
			case 'error':
				return 'ERR ';
			case 'fatal':
				return 'FATL';
			case 'debug':
				return 'DEBG';
			case 'trace':
				return 'TRCE';
			default:
				return level.toUpperCase().slice(0, 4).padEnd(4);
		}
	}

	function formatTime(ts: string): string {
		try {
			const d = new Date(ts);
			const h = String(d.getHours()).padStart(2, '0');
			const m = String(d.getMinutes()).padStart(2, '0');
			const s = String(d.getSeconds()).padStart(2, '0');
			const ms = String(d.getMilliseconds()).padStart(3, '0');
			return `${h}:${m}:${s}.${ms}`;
		} catch {
			return ts;
		}
	}

	function formatFields(fields: Record<string, unknown> | undefined): string {
		if (!fields || Object.keys(fields).length === 0) return '';
		return Object.entries(fields)
			.map(([k, v]) => `${k}=${typeof v === 'string' ? v : JSON.stringify(v)}`)
			.join(' ');
	}

	$effect(() => {
		if (entries.length && scrollContainer) {
			tick().then(() => {
				scrollContainer.scrollTop = scrollContainer.scrollHeight;
			});
		}
	});
</script>

<div
	bind:this={scrollContainer}
	class="h-[500px] overflow-auto rounded-md border border-border bg-zinc-950 p-3 font-mono text-xs leading-5 {className}"
>
	{#each entries as entry}
		<div class="whitespace-nowrap hover:bg-zinc-900/50 px-1 rounded">
			<span class="text-zinc-600">{formatTime(entry.ts)}</span>
			<span class="inline-block w-[38px] text-center font-semibold {levelColor(entry.level)}"
				>{shortLevel(entry.level)}</span
			>
			<span class="text-zinc-200">{entry.msg}</span>
			{#if entry.fields && Object.keys(entry.fields).length > 0}
				<span class="text-zinc-600 ml-2">{formatFields(entry.fields)}</span>
			{/if}
		</div>
	{/each}
	{#if entries.length === 0}
		<div class="text-zinc-600 py-4 text-center">No log entries yet.</div>
	{/if}
</div>
