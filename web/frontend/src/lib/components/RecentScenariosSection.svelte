<script lang="ts">
	import type { SavedScenario } from '$lib/types';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { formatUserEmail, scenarioTypeVariant } from '$lib/utils/format';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';

	let { scenarios }: { scenarios: SavedScenario[] } = $props();
</script>

<Card.Root class="animate-fade-up stagger-2">
	<Card.Header class="pb-3">
		<Card.Title class="text-base">Recent Scenarios</Card.Title>
	</Card.Header>
	<Card.Content class="pt-0">
		{#if scenarios.length === 0}
			<p class="text-sm text-muted-foreground py-4 text-center">No scenarios yet.</p>
		{:else}
			<div class="space-y-2">
				{#each scenarios as scenario}
					<div
						class="flex items-center justify-between gap-2 rounded-lg ring-1 ring-foreground/10 bg-card/50 backdrop-blur-sm px-3 py-2"
					>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2">
								<span class="text-sm font-medium truncate">{scenario.name}</span>
								<Badge
									variant={scenarioTypeVariant(scenario.type)}
									class="text-[10px] px-1.5 py-0 shrink-0"
								>
									{scenario.type || 'standard'}
								</Badge>
							</div>
							<div class="flex items-center gap-2 text-xs text-muted-foreground mt-0.5">
								{#if scenario.createdBy && scenario.createdBy !== 'anonymous'}
									<Tooltip.Root>
										<Tooltip.Trigger class="cursor-default">
											{formatUserEmail(scenario.createdBy)}
										</Tooltip.Trigger>
										<Tooltip.Content>{scenario.createdBy}</Tooltip.Content>
									</Tooltip.Root>
								{/if}
								<span>{new Date(scenario.createdAt).toLocaleDateString()}</span>
							</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</Card.Content>
</Card.Root>
