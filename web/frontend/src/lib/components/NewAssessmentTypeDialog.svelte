<script lang="ts">
	import { goto } from '$app/navigation';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
	import SearchIcon from '@lucide/svelte/icons/search';
	import FileSearchIcon from '@lucide/svelte/icons/file-search';
	import type { Component } from 'svelte';
	import type { ScenarioType } from '$lib/types';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	type TypeMeta = {
		value: ScenarioType;
		title: string;
		description: string;
		icon: Component;
		iconClass: string;
	};

	const types: TypeMeta[] = [
		{
			value: 'standard',
			title: 'Standard',
			description: 'Detonate simulations and verify that specific detection rules trigger alerts.',
			icon: ShieldCheckIcon,
			iconClass: 'text-primary'
		},
		{
			value: 'explore',
			title: 'Explore',
			description:
				'Run simulations and discover all triggered alerts without specifying expected rules.',
			icon: SearchIcon,
			iconClass: 'text-attr-identity'
		},
		{
			value: 'collect',
			title: 'Collect',
			description: 'Run simulations and collect related logs for analysis after detonation.',
			icon: FileSearchIcon,
			iconClass: 'text-attr-environment'
		}
	];

	function pick(t: ScenarioType) {
		open = false;
		goto(`/assessments/new?type=${t}`);
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-3xl">
		<Dialog.Header>
			<Dialog.Title>New Assessment</Dialog.Title>
			<Dialog.Description>
				Choose an assessment type. The type is fixed once created.
			</Dialog.Description>
		</Dialog.Header>
		<div class="grid gap-4 sm:grid-cols-3 pt-2">
			{#each types as t}
				<button
					type="button"
					class="flex flex-col items-center gap-3 rounded-lg border-2 border-border p-6 text-left transition-colors hover:bg-muted/50 hover:border-primary/50"
					onclick={() => pick(t.value)}
				>
					<t.icon size={36} class={t.iconClass} />
					<div class="text-center">
						<p class="text-sm font-medium">{t.title}</p>
						<p class="text-xs text-muted-foreground mt-1 leading-relaxed">
							{t.description}
						</p>
					</div>
				</button>
			{/each}
		</div>
	</Dialog.Content>
</Dialog.Root>
