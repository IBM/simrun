<script module lang="ts">
	export type KeyValueEntry = { key: string; value: string };

	export function objectToEntries(v: unknown): KeyValueEntry[] {
		if (!v || typeof v !== 'object') return [];
		return Object.entries(v as Record<string, unknown>).map(([key, value]) => ({
			key,
			value: typeof value === 'string' ? value : JSON.stringify(value)
		}));
	}

	export function entriesToObject(entries: KeyValueEntry[]): Record<string, string> {
		const out: Record<string, string> = {};
		for (const e of entries) {
			const k = e.key.trim();
			if (!k) continue;
			out[k] = e.value;
		}
		return out;
	}

	// Blank keys/values are invalid — Terraform/cloud providers reject empty tag
	// keys or values, so callers check this before saving.
	export function hasBlankEntry(map: Record<string, unknown>): boolean {
		return Object.entries(map).some(
			([k, v]) => k.trim() === '' || typeof v !== 'string' || v.trim() === ''
		);
	}
</script>

<script lang="ts">
	import { Input } from '$lib/components/ui/input/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';

	let {
		value,
		inherited = {},
		inheritedLabel = 'Inherited from Settings',
		onchange
	}: {
		value: unknown;
		inherited?: Record<string, string>;
		inheritedLabel?: string;
		onchange: (next: Record<string, string>) => void;
	} = $props();

	let entries = $derived(objectToEntries(value));
	let inheritedEntries = $derived(Object.entries(inherited));
</script>

<div class="space-y-2">
	{#if inheritedEntries.length > 0}
		{@const packKeys = new Set(entries.map((e) => e.key.trim()))}
		<div class="space-y-1">
			{#each inheritedEntries as [k, v] (k)}
				{@const overridden = packKeys.has(k)}
				<div
					class="grid grid-cols-[1fr_1fr_auto] items-center gap-2 px-3 text-xs text-muted-foreground"
				>
					<span class="font-mono truncate {overridden ? 'line-through opacity-60' : ''}">
						{k}
					</span>
					<span class="font-mono truncate {overridden ? 'line-through opacity-60' : ''}">
						{v}
					</span>
					<span class="text-[10px]">{overridden ? 'overridden' : ''}</span>
				</div>
			{/each}
			<p class="px-3 text-[10px] text-muted-foreground/70">{inheritedLabel}</p>
		</div>
	{/if}
	{#each entries as entry, i}
		<div class="grid grid-cols-[1fr_1fr_auto] gap-2">
			<Input
				placeholder="key"
				value={entry.key}
				oninput={(e) => {
					const next = entries.slice();
					next[i] = { ...next[i], key: (e.target as HTMLInputElement).value };
					onchange(entriesToObject(next));
				}}
			/>
			<Input
				placeholder="value"
				value={entry.value}
				oninput={(e) => {
					const next = entries.slice();
					next[i] = { ...next[i], value: (e.target as HTMLInputElement).value };
					onchange(entriesToObject(next));
				}}
			/>
			<Button
				variant="outline"
				size="sm"
				class="h-9 w-8 p-0"
				onclick={() => {
					const next = entries.filter((_, idx) => idx !== i);
					onchange(entriesToObject(next));
				}}
			>
				<XIcon size={14} />
			</Button>
		</div>
	{/each}
	<Button
		variant="outline"
		size="sm"
		onclick={() => {
			onchange({ ...entriesToObject(entries), '': '' });
		}}
	>
		<PlusIcon data-icon="inline-start" />
		Add Entry
	</Button>
</div>
