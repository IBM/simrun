<script lang="ts">
	import type { Run } from '$lib/types';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { statusVariant, formatDuration, formatTime } from '$lib/utils/format';

	let { run }: { run: Run } = $props();
</script>

<a href="/assessments/{run.id}" class="block">
	<Card.Root class="transition-colors hover:bg-accent/50 cursor-pointer">
		<Card.Header class="pb-2">
			<div class="flex items-center justify-between">
				<Card.Title class="text-sm font-mono">{run.id.slice(0, 8)}</Card.Title>
				<Badge variant={statusVariant(run.status)}>{run.status}</Badge>
			</div>
		</Card.Header>
		<Card.Content>
			<div class="flex items-center justify-between text-sm text-muted-foreground">
				<div class="flex gap-3">
					<span>{run.total} total</span>
					<span class="text-success">{run.succeeded} passed</span>
					{#if run.failed > 0}
						<span class="text-destructive">{run.failed} failed</span>
					{/if}
				</div>
				<span>{formatDuration(run.startTime, run.endTime)}</span>
			</div>
			<div class="mt-1 text-xs text-muted-foreground">
				{formatTime(run.startTime)}
			</div>
		</Card.Content>
	</Card.Root>
</a>
