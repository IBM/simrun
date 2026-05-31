<script lang="ts">
	import TrendingUpIcon from '@lucide/svelte/icons/trending-up';
	import TrendingDownIcon from '@lucide/svelte/icons/trending-down';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import PlayIcon from '@lucide/svelte/icons/play';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Card from '$lib/components/ui/card/index.js';

	let {
		totalRuns,
		ruleCoveragePercent,
		activeRuns,
		savedScenarios
	}: {
		totalRuns: number;
		ruleCoveragePercent: number;
		activeRuns: number;
		savedScenarios: number;
	} = $props();

	const rateIsGood = $derived(ruleCoveragePercent >= 50);
</script>

<div class="grid grid-cols-1 gap-4 px-4 lg:px-6 @xl/main:grid-cols-2 @5xl/main:grid-cols-4">
	<!-- Success Rate - Hero card -->
	<Card.Root
		class="@container/card relative overflow-hidden animate-fade-up stagger-1 @5xl/main:col-span-1 border-primary/20 bg-gradient-to-br from-primary/5 via-card to-card dark:from-primary/10 dark:via-card dark:to-card"
	>
		<div
			class="absolute top-0 right-0 w-24 h-24 bg-primary/5 rounded-full -translate-y-8 translate-x-8 dark:bg-primary/10"
		></div>
		<Card.Header>
			<div class="flex items-center gap-2">
				<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10">
					<ShieldCheckIcon class="h-4 w-4 text-primary" />
				</div>
				<Card.Description class="text-sm font-medium">Rule Coverage</Card.Description>
			</div>
			<Card.Title class="text-3xl font-bold tabular-nums font-mono @[250px]/card:text-4xl">
				{ruleCoveragePercent}%
			</Card.Title>
			<Card.Action>
				<Badge
					variant="outline"
					class={rateIsGood
						? 'border-success/30 text-success'
						: 'border-destructive/30 text-destructive'}
				>
					{#if rateIsGood}
						<TrendingUpIcon class="size-3" />
					{:else}
						<TrendingDownIcon class="size-3" />
					{/if}
					{ruleCoveragePercent}%
				</Badge>
			</Card.Action>
		</Card.Header>
		<Card.Footer class="flex-col items-start gap-1.5 text-sm">
			<div class="line-clamp-1 flex gap-2 font-medium">
				{#if rateIsGood}
					Rule coverage is strong <TrendingUpIcon class="size-4 text-success" />
				{:else}
					Coverage needs attention <TrendingDownIcon class="size-4 text-destructive" />
				{/if}
			</div>
			<div class="text-muted-foreground text-xs">Rules covered by saved scenarios</div>
		</Card.Footer>
	</Card.Root>

	<Card.Root class="@container/card animate-fade-up stagger-2">
		<Card.Header>
			<div class="flex items-center gap-2">
				<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-muted">
					<PlayIcon class="h-4 w-4 text-muted-foreground" />
				</div>
				<Card.Description class="text-sm font-medium">Total Assessments</Card.Description>
			</div>
			<Card.Title class="text-2xl font-bold tabular-nums font-mono @[250px]/card:text-3xl">
				{totalRuns}
			</Card.Title>
		</Card.Header>
		<Card.Footer class="flex-col items-start gap-1.5 text-sm">
			<div class="text-muted-foreground text-xs">All completed and active assessments</div>
		</Card.Footer>
	</Card.Root>

	<Card.Root class="@container/card animate-fade-up stagger-3">
		<Card.Header>
			<div class="flex items-center gap-2">
				<div
					class="flex h-8 w-8 items-center justify-center rounded-lg {activeRuns > 0
						? 'bg-primary/10'
						: 'bg-muted'}"
				>
					<ActivityIcon
						class="h-4 w-4 {activeRuns > 0 ? 'text-primary' : 'text-muted-foreground'}"
					/>
				</div>
				<Card.Description class="text-sm font-medium">Active Assessments</Card.Description>
			</div>
			<Card.Title class="text-2xl font-bold tabular-nums font-mono @[250px]/card:text-3xl">
				{activeRuns}
			</Card.Title>
		</Card.Header>
		<Card.Footer class="flex-col items-start gap-1.5 text-sm">
			<div class="text-muted-foreground text-xs">Assessments currently in progress</div>
		</Card.Footer>
	</Card.Root>

	<Card.Root class="@container/card animate-fade-up stagger-4">
		<Card.Header>
			<div class="flex items-center gap-2">
				<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-muted">
					<FileTextIcon class="h-4 w-4 text-muted-foreground" />
				</div>
				<Card.Description class="text-sm font-medium">Saved Scenarios</Card.Description>
			</div>
			<Card.Title class="text-2xl font-bold tabular-nums font-mono @[250px]/card:text-3xl">
				{savedScenarios}
			</Card.Title>
		</Card.Header>
		<Card.Footer class="flex-col items-start gap-1.5 text-sm">
			<div class="text-muted-foreground text-xs">Configured scenario definitions</div>
		</Card.Footer>
	</Card.Root>
</div>
