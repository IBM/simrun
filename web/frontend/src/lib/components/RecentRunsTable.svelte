<script lang="ts">
	import type { Run } from '$lib/types';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { goto } from '$app/navigation';
	import { statusVariant, formatDuration } from '$lib/utils/format';

	let { runs }: { runs: Run[] } = $props();
</script>

<Card.Root class="animate-fade-up stagger-1">
	<Card.Header class="pb-3">
		<Card.Title class="text-base">Recent Runs</Card.Title>
	</Card.Header>
	<Card.Content class="pt-0">
		{#if runs.length === 0}
			<p class="text-sm text-muted-foreground py-4 text-center">
				No runs yet. Start a run from the Assessments page.
			</p>
		{:else}
			<div class="space-y-2">
				{#each runs as run (run.id)}
					<button
						type="button"
						class="w-full text-left rounded-lg ring-1 ring-foreground/10 bg-card/50 backdrop-blur-sm px-3 py-2 hover:bg-accent/50 transition-colors cursor-pointer"
						onclick={() => goto(`/runs/${run.id}`)}
					>
						<div class="flex items-center justify-between gap-2">
							<div class="flex items-center gap-2 min-w-0">
								<span class="font-mono text-xs text-muted-foreground">{run.id.slice(0, 8)}</span>
								<Badge variant={statusVariant(run.status)} class="text-[10px] px-1.5 py-0">
									{run.status}
								</Badge>
							</div>
							<span class="text-xs text-muted-foreground shrink-0">
								{formatDuration(run.startTime, run.endTime)}
							</span>
						</div>
						{#if run.assessmentName}
							<div class="text-xs text-muted-foreground mt-0.5 truncate">
								{run.assessmentName}
							</div>
						{/if}
					</button>
				{/each}
			</div>
		{/if}
	</Card.Content>
</Card.Root>
