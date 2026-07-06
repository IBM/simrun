<script lang="ts">
	import { onMount } from 'svelte';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import PackCard from '$lib/components/PackCard.svelte';
	import MitreCoverage from '$lib/components/MitreCoverage.svelte';
	import { packs, loadPacks } from '$lib/stores/packs';
	import { installPack, uploadPack } from '$lib/api/client';
	import { toast } from 'svelte-sonner';
	import PackageIcon from '@lucide/svelte/icons/package';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import SearchIcon from '@lucide/svelte/icons/search';

	let loading = $state(true);
	let error = $state('');
	let activeTab = $state('installed');
	let filterQuery = $state('');

	let filteredPacks = $derived.by(() => {
		const q = filterQuery.toLowerCase().trim();
		if (!q) return $packs;
		return $packs.filter(
			(p) => p.name.toLowerCase().includes(q) || p.source.toLowerCase().includes(q)
		);
	});

	let installDialogOpen = $state(false);
	let installType = $state('remote');
	let installSource = $state('');
	let installVersion = $state('');
	let installFile = $state<File | null>(null);
	let installing = $state(false);

	const typeOptions = [
		{ value: 'remote', label: 'Remote' },
		{ value: 'upload', label: 'Upload' }
	];

	let typeLabel = $derived(
		typeOptions.find((o) => o.value === installType)?.label ?? 'Select type'
	);

	onMount(async () => {
		try {
			await loadPacks();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load packs';
		} finally {
			loading = false;
		}
	});

	async function handleInstall() {
		installing = true;
		try {
			if (installType === 'upload') {
				if (!installFile) return;
				await uploadPack(installFile);
			} else {
				if (!installSource.trim()) return;
				await installPack({
					type: installType,
					source: installSource.trim(),
					version: installVersion.trim() || undefined
				});
			}
			await loadPacks();
			installDialogOpen = false;
			installSource = '';
			installVersion = '';
			installFile = null;
			toast.success('Pack installed');
		} catch (e) {
			toast.error('Install failed', {
				description: e instanceof Error ? e.message : 'Unexpected error'
			});
		} finally {
			installing = false;
		}
	}

	let installDisabled = $derived(
		installing || (installType === 'upload' ? !installFile : !installSource.trim())
	);

	async function handleDelete() {
		await loadPacks();
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold">Packs</h1>
		<Dialog.Root bind:open={installDialogOpen}>
			<Dialog.Trigger>
				{#snippet child({ props })}
					<Button {...props}>Install Pack</Button>
				{/snippet}
			</Dialog.Trigger>
			<Dialog.Content>
				<Dialog.Header>
					<Dialog.Title>Install Pack</Dialog.Title>
					<Dialog.Description>Install a new simulation pack.</Dialog.Description>
				</Dialog.Header>
				<div class="space-y-4">
					<div>
						<Label>Type</Label>
						<div class="mt-1">
							<Select.Root type="single" bind:value={installType}>
								<Select.Trigger class="w-full">
									{typeLabel}
								</Select.Trigger>
								<Select.Content>
									{#each typeOptions as opt}
										<Select.Item value={opt.value} label={opt.label} />
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
					</div>
					{#if installType === 'upload'}
						<div>
							<Label>Binary File</Label>
							<Input
								type="file"
								class="mt-1"
								onchange={(e: Event) => {
									const target = e.target as HTMLInputElement;
									installFile = target.files?.[0] ?? null;
								}}
							/>
						</div>
						<Alert.Root variant="destructive">
							<TriangleAlertIcon class="h-4 w-4" />
							<Alert.Title>Security Warning</Alert.Title>
							<Alert.Description>
								Pack binaries execute with server privileges. Only upload packs you trust.
							</Alert.Description>
						</Alert.Root>
					{:else}
						<Input placeholder="Source (github.com/org/repo)" bind:value={installSource} />
						<Input placeholder="Version (optional, latest if blank)" bind:value={installVersion} />
					{/if}
				</div>
				<div class="flex justify-end gap-2 pt-4">
					<Button variant="outline" onclick={() => (installDialogOpen = false)}>Cancel</Button>
					<Button onclick={handleInstall} disabled={installDisabled}>
						{installing ? 'Installing...' : installType === 'upload' ? 'Upload' : 'Install'}
					</Button>
				</div>
			</Dialog.Content>
		</Dialog.Root>
	</div>

	{#if error}
		<Alert.Root variant="destructive">
			<Alert.Description>{error}</Alert.Description>
		</Alert.Root>
	{/if}

	<Tabs.Root bind:value={activeTab}>
		<div class="flex flex-wrap items-center justify-between gap-3">
			<Tabs.List>
				<Tabs.Trigger value="installed">Installed Packs</Tabs.Trigger>
				<Tabs.Trigger value="coverage">MITRE Coverage</Tabs.Trigger>
			</Tabs.List>
			{#if activeTab === 'installed' && !loading && $packs.length > 0}
				<div class="relative w-full sm:w-64">
					<SearchIcon
						class="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
					/>
					<Input placeholder="Filter packs..." class="pl-9" bind:value={filterQuery} />
				</div>
			{/if}
		</div>

		<Tabs.Content value="installed" class="mt-4">
			{#if loading}
				<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
					{#each Array(3) as _}
						<Skeleton class="h-40 w-full rounded-xl" />
					{/each}
				</div>
			{:else if $packs.length === 0}
				<Empty.Root>
					<Empty.Header>
						<Empty.Media variant="icon">
							<PackageIcon />
						</Empty.Media>
						<Empty.Title>No packs installed</Empty.Title>
						<Empty.Description
							>Install simulation packs to extend available attack techniques.</Empty.Description
						>
					</Empty.Header>
					<Empty.Content>
						<Button onclick={() => (installDialogOpen = true)}>Install Pack</Button>
					</Empty.Content>
				</Empty.Root>
			{:else if filteredPacks.length === 0}
				<p class="py-12 text-center text-sm text-muted-foreground">
					No packs match "{filterQuery}".
				</p>
			{:else}
				<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
					{#each filteredPacks as pack, i (pack.id)}
						<div class="animate-fade-up h-full" style="animation-delay: {i * 60}ms">
							<PackCard {pack} ondelete={handleDelete} />
						</div>
					{/each}
				</div>
			{/if}
		</Tabs.Content>

		<Tabs.Content value="coverage" class="mt-4">
			{#if loading}
				<Skeleton class="h-96 w-full rounded-xl" />
			{:else if $packs.length === 0}
				<Empty.Root>
					<Empty.Header>
						<Empty.Media variant="icon">
							<PackageIcon />
						</Empty.Media>
						<Empty.Title>No packs installed</Empty.Title>
						<Empty.Description>Install packs to see MITRE ATT&CK coverage.</Empty.Description>
					</Empty.Header>
				</Empty.Root>
			{:else}
				{#key activeTab}
					<MitreCoverage />
				{/key}
			{/if}
		</Tabs.Content>
	</Tabs.Root>
</div>
