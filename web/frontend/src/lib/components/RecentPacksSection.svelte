<script lang="ts">
	import type { Pack } from '$lib/types';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { formatUserEmail } from '$lib/utils/format';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';

	let { packs }: { packs: Pack[] } = $props();
</script>

<Card.Root class="animate-fade-up stagger-3">
	<Card.Header class="pb-3">
		<Card.Title class="text-base">Recent Packs</Card.Title>
	</Card.Header>
	<Card.Content class="pt-0">
		{#if packs.length === 0}
			<p class="text-sm text-muted-foreground py-4 text-center">No packs installed.</p>
		{:else}
			<div class="space-y-2">
				{#each packs as pack}
					<div
						class="flex items-center justify-between gap-2 rounded-lg ring-1 ring-foreground/10 bg-card/50 backdrop-blur-sm px-3 py-2"
					>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2">
								<span class="text-sm font-medium truncate">{pack.name}</span>
								<Badge variant="outline" class="text-[10px] px-1.5 py-0 shrink-0 font-mono">
									{pack.version}
								</Badge>
							</div>
							<div class="flex items-center gap-2 text-xs text-muted-foreground mt-0.5">
								{#if pack.installedBy && pack.installedBy !== 'anonymous'}
									<Tooltip.Root>
										<Tooltip.Trigger class="cursor-default">
											{formatUserEmail(pack.installedBy)}
										</Tooltip.Trigger>
										<Tooltip.Content>{pack.installedBy}</Tooltip.Content>
									</Tooltip.Root>
								{/if}
								<span>{new Date(pack.createdAt).toLocaleDateString()}</span>
							</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</Card.Content>
</Card.Root>
