<script lang="ts">
	import { onMount } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import YamlEditor from './YamlEditor.svelte';
	import ScenarioSection from './ScenarioSection.svelte';
	import WrenchIcon from '@lucide/svelte/icons/wrench';
	import CodeIcon from '@lucide/svelte/icons/code';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import PlayIcon from '@lucide/svelte/icons/play';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import {
		generateScenarioYAML,
		createEmptyTarget,
		createEmptyScenario
	} from '$lib/utils/yaml-generator';
	import type { FormScenario, FormTarget } from '$lib/utils/yaml-generator';
	import { parseScenarioYAML } from '$lib/utils/yaml-parser';
	import {
		listPacks,
		listConnectors,
		listElasticRulesAuto,
		getPackManifest,
		lintAssessment
	} from '$lib/api/client';
	import { scenarioTypeVariant } from '$lib/utils/format';
	import type { Pack, PackManifest, ElasticRule, Connector, ScenarioType } from '$lib/types';

	type SaveOptions = { run?: boolean };

	let {
		mode,
		type,
		initialName = '',
		initialScenarios,
		initialTarget,
		initialYaml = '',
		initialBuilderSupported = true,
		onsave,
		oncancel,
		onschedule
	}: {
		mode: 'create' | 'edit';
		type: ScenarioType;
		initialName?: string;
		initialScenarios?: FormScenario[];
		initialTarget?: FormTarget;
		initialYaml?: string;
		initialBuilderSupported?: boolean;
		onsave: (name: string, yaml: string, options: SaveOptions) => Promise<void>;
		oncancel: () => void;
		onschedule?: () => void;
	} = $props();

	const cloudTypes = ['aws', 'gcp', 'azure', 'kubernetes'] as const;

	/* svelte-ignore state_referenced_locally */
	let name = $state(initialName);
	/* svelte-ignore state_referenced_locally */
	let scenarios = $state<FormScenario[]>(
		initialScenarios && initialScenarios.length > 0 ? initialScenarios : [seedScenario(type)]
	);
	/* svelte-ignore state_referenced_locally */
	let target = $state<FormTarget>(initialTarget ?? createEmptyTarget());
	/* svelte-ignore state_referenced_locally */
	let yamlText = $state(initialYaml);
	/* svelte-ignore state_referenced_locally */
	let builderSupported = $state(initialBuilderSupported);
	/* svelte-ignore state_referenced_locally */
	let editorMode = $state<'builder' | 'yaml'>(initialBuilderSupported ? 'builder' : 'yaml');

	let packs = $state<Pack[]>([]);
	let packManifests = $state<Map<string, PackManifest>>(new Map());
	let elasticRules = $state<ElasticRule[]>([]);
	let connectors = $state<Connector[]>([]);
	let resourcesLoading = $state(true);
	let loadingManifests = new Set<string>();

	let saving = $state(false);
	let savingAndRunning = $state(false);
	let error = $state('');

	let isDirty = $state(false);
	let leaveDialogOpen = $state(false);
	let pendingNavigate = $state<(() => void) | null>(null);
	let skipGuard = false;

	let cloudConnectors = $derived(
		connectors.filter(
			(c) => cloudTypes.includes(c.type as (typeof cloudTypes)[number]) && c.enabled
		)
	);

	function seedScenario(t: ScenarioType): FormScenario {
		const s = createEmptyScenario();
		if (t === 'collect') {
			s.collection = { ...s.collection, enabled: true, type: 'elastic' };
		}
		return s;
	}

	onMount(async () => {
		try {
			const [packsResult, connectorsResult] = await Promise.all([
				listPacks(),
				listConnectors().catch(() => [] as Connector[])
			]);
			packs = packsResult;
			connectors = connectorsResult;

			try {
				const resp = await listElasticRulesAuto();
				elasticRules = resp.data;
			} catch {
				elasticRules = [];
			}

			const usedPacks = new Set(scenarios.map((s) => s.pack).filter(Boolean));
			await Promise.all([...usedPacks].map((p) => loadPackManifest(p)));
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load builder resources';
		} finally {
			resourcesLoading = false;
		}
	});

	beforeNavigate((nav) => {
		if (skipGuard || !isDirty || nav.willUnload) return;
		if (!nav.to) return;
		const target = nav.to.url;
		nav.cancel();
		pendingNavigate = () => {
			skipGuard = true;
			goto(target.pathname + target.search + target.hash);
		};
		leaveDialogOpen = true;
	});

	async function loadPackManifest(packName: string) {
		if (!packName || packManifests.has(packName) || loadingManifests.has(packName)) return;
		loadingManifests.add(packName);
		try {
			const manifest = await getPackManifest(packName);
			packManifests = new Map(packManifests).set(packName, manifest);
		} catch (e) {
			error = e instanceof Error ? e.message : `Failed to load manifest for "${packName}"`;
		} finally {
			loadingManifests.delete(packName);
		}
	}

	function currentYaml(): string {
		if (editorMode === 'yaml') return yamlText;
		return generateScenarioYAML(scenarios, target, type);
	}

	function markDirty() {
		isDirty = true;
	}

	function switchToYaml() {
		if (editorMode === 'builder') {
			yamlText = generateScenarioYAML(scenarios, target, type);
		}
		editorMode = 'yaml';
		error = '';
	}

	function switchToBuilder() {
		const parseResult = parseScenarioYAML(yamlText);
		if (!parseResult.success) {
			error = parseResult.error || 'Cannot parse YAML';
			return;
		}
		if (!parseResult.builderSupported) {
			error = 'This YAML uses detonator types not supported by the builder. Stay in YAML mode.';
			return;
		}
		scenarios = parseResult.scenarios || [];
		target = parseResult.target || createEmptyTarget();
		editorMode = 'builder';
		error = '';
		const usedPacks = new Set(scenarios.map((s) => s.pack).filter(Boolean));
		Promise.all([...usedPacks].map((p) => loadPackManifest(p)));
	}

	function handleScenarioUpdate(index: number, scenario: FormScenario) {
		const next = [...scenarios];
		next[index] = scenario;
		scenarios = next;
		markDirty();
	}

	function handleScenarioRemove(index: number) {
		scenarios = scenarios.filter((_, i) => i !== index);
		markDirty();
	}

	function addScenario() {
		scenarios = [...scenarios, seedScenario(type)];
		markDirty();
	}

	function duplicateScenario(index: number) {
		const source = scenarios[index];
		const copy = $state.snapshot(source) as FormScenario;
		copy.name = source.name ? `${source.name} (copy)` : '';
		scenarios = [...scenarios.slice(0, index + 1), copy, ...scenarios.slice(index + 1)];
		markDirty();
	}

	function validateBuilder(): string | null {
		if (!name.trim()) return 'Scenario file name is required.';
		for (let i = 0; i < scenarios.length; i++) {
			const s = scenarios[i];
			if (!s.name.trim()) return `Scenario ${i + 1}: Name is required.`;
			if (!s.pack) return `Scenario ${i + 1}: Pack is required.`;
			if (s.scenarioType === 'inject') {
				if (!s.template) return `Scenario ${i + 1}: Injection template is required.`;
				if (!s.injectIndex.trim())
					return `Scenario ${i + 1}: Datastream is required for injection scenarios.`;
			} else if (!s.simulation) {
				return `Scenario ${i + 1}: Simulation is required.`;
			}
			if (type === 'collect' && s.scenarioType !== 'inject' && !s.collection.index.trim()) {
				return `Scenario ${i + 1}: Elasticsearch index is required for collect scenarios.`;
			}
			if (type === 'standard') {
				for (let j = 0; j < s.expectations.length; j++) {
					const exp = s.expectations[j];
					if (!exp.alertName.trim())
						return `Scenario ${i + 1}, Expectation ${j + 1}: Alert name is required.`;
					if (exp.timeout && !/^\d+[smh]$/.test(exp.timeout))
						return `Scenario ${i + 1}, Expectation ${j + 1}: Timeout must be a valid duration (e.g., 5m).`;
				}
			}
		}
		return null;
	}

	async function attemptSave(opts: SaveOptions) {
		error = '';
		if (!name.trim()) {
			error = 'Scenario file name is required.';
			return;
		}
		if (editorMode === 'builder' && builderSupported) {
			const v = validateBuilder();
			if (v) {
				error = v;
				return;
			}
		}
		const yaml = currentYaml();
		const lint = await lintAssessment(yaml).catch((e) => ({
			valid: false,
			error: e instanceof Error ? e.message : 'Lint failed'
		}));
		if (!lint.valid) {
			error = lint.error || 'Generated YAML is invalid.';
			return;
		}

		if (opts.run) savingAndRunning = true;
		else saving = true;
		skipGuard = true;
		try {
			await onsave(name.trim(), yaml, opts);
			isDirty = false;
			if (!opts.run) {
				toast.success(mode === 'create' ? 'Assessment created' : 'Assessment saved');
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Save failed';
			skipGuard = false;
		} finally {
			saving = false;
			savingAndRunning = false;
		}
	}

	function handleCancel() {
		if (isDirty) {
			pendingNavigate = () => oncancel();
			leaveDialogOpen = true;
			return;
		}
		oncancel();
	}

	function discardAndLeave() {
		skipGuard = true;
		isDirty = false;
		leaveDialogOpen = false;
		const fn = pendingNavigate;
		pendingNavigate = null;
		fn?.();
	}
</script>

<div class="flex flex-col h-full">
	<!-- Sticky toolbar: page title + actions (matches app convention) -->
	<header
		class="sticky top-0 z-30 border-b border-border/60 bg-background/80 backdrop-blur supports-[backdrop-filter]:bg-background/60"
	>
		<div class="mx-auto max-w-5xl px-6 py-3 flex items-center gap-3 flex-wrap">
			<h1 class="text-2xl font-bold whitespace-nowrap">
				{mode === 'create' ? 'New Assessment' : 'Edit Assessment'}
			</h1>
			<Badge variant={scenarioTypeVariant(type)} class="shrink-0">{type}</Badge>

			<div class="ml-auto flex items-center gap-2">
				{#if mode === 'edit' && onschedule}
					<Button variant="ghost" size="sm" onclick={onschedule}>
						<CalendarIcon data-icon="inline-start" />
						Schedule
					</Button>
				{/if}
				<Button
					variant="outline"
					size="sm"
					onclick={handleCancel}
					disabled={saving || savingAndRunning}
				>
					Cancel
				</Button>
				<Button
					size="sm"
					variant="secondary"
					onclick={() => attemptSave({ run: true })}
					disabled={saving || savingAndRunning || !name.trim()}
				>
					{#if savingAndRunning}
						<LoaderIcon data-icon="inline-start" class="animate-spin" />
					{:else}
						<PlayIcon data-icon="inline-start" />
					{/if}
					Save & Run
				</Button>
				<Button
					size="sm"
					onclick={() => attemptSave({})}
					disabled={saving || savingAndRunning || !name.trim()}
				>
					{saving ? 'Saving…' : mode === 'create' ? 'Create' : 'Save'}
				</Button>
			</div>
		</div>
	</header>

	<!-- Body -->
	<main class="flex-1 overflow-y-auto">
		<div class="mx-auto max-w-5xl px-6 py-6 space-y-6">
			{#if error}
				<Alert.Root variant="destructive">
					<Alert.Description>{error}</Alert.Description>
				</Alert.Root>
			{/if}

			<!-- Name + view mode toggle -->
			<div class="flex flex-wrap items-end justify-between gap-4">
				<div class="flex flex-col gap-1.5 grow min-w-[260px]">
					<Label for="scenario-file-name" class="text-muted-foreground text-xs font-medium">
						Name
					</Label>
					<Input
						id="scenario-file-name"
						placeholder="test-assessment"
						bind:value={name}
						oninput={markDirty}
						class="h-9 max-w-md"
					/>
				</div>

				{#if builderSupported}
					<div class="inline-flex rounded-md border bg-muted/40 p-0.5">
						<button
							type="button"
							class="inline-flex items-center gap-1.5 rounded px-3 py-1 text-xs font-medium transition-colors {editorMode ===
							'builder'
								? 'bg-background shadow-sm text-foreground'
								: 'text-muted-foreground hover:text-foreground'}"
							onclick={switchToBuilder}
						>
							<WrenchIcon class="h-3.5 w-3.5" />
							Builder
						</button>
						<button
							type="button"
							class="inline-flex items-center gap-1.5 rounded px-3 py-1 text-xs font-medium transition-colors {editorMode ===
							'yaml'
								? 'bg-background shadow-sm text-foreground'
								: 'text-muted-foreground hover:text-foreground'}"
							onclick={switchToYaml}
						>
							<CodeIcon class="h-3.5 w-3.5" />
							YAML
						</button>
					</div>
				{:else}
					<Badge variant="secondary">YAML only — uses unsupported detonator</Badge>
				{/if}
			</div>

			{#if editorMode === 'builder' && builderSupported}
				{#if resourcesLoading}
					<div class="flex items-center gap-2 text-sm text-muted-foreground py-12 justify-center">
						<LoaderIcon class="h-4 w-4 animate-spin" />
						Loading packs, connectors, rules…
					</div>
				{:else}
					{#if cloudConnectors.length > 0}
						<section class="rounded-lg border bg-card/50 p-4">
							<div class="flex items-baseline justify-between mb-3">
								<Label class="text-sm font-medium">Target Connectors</Label>
								<span class="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
									shared by all scenarios
								</span>
							</div>
							<div class="grid grid-cols-2 md:grid-cols-4 gap-3">
								{#each cloudTypes as cloudType}
									{@const available = connectors.filter((c) => c.type === cloudType && c.enabled)}
									{#if available.length > 0}
										<div>
											<Label
												class="font-mono text-[10px] uppercase tracking-wider text-muted-foreground mb-1 block"
											>
												{cloudType}
											</Label>
											<Select.Root
												type="single"
												bind:value={target[cloudType]}
												onValueChange={() => markDirty()}
											>
												<Select.Trigger class="w-full h-9 text-sm">
													{target[cloudType] || 'None'}
												</Select.Trigger>
												<Select.Content>
													<Select.Item value="" label="None" />
													{#each available as conn}
														<Select.Item value={conn.name} label={conn.name} />
													{/each}
												</Select.Content>
											</Select.Root>
										</div>
									{/if}
								{/each}
							</div>
						</section>
					{/if}

					<div class="space-y-4">
						{#each scenarios as scenario, i}
							<ScenarioSection
								{scenario}
								index={i}
								{packs}
								{packManifests}
								{elasticRules}
								scenarioFileType={type}
								onupdate={(s) => handleScenarioUpdate(i, s)}
								onremove={() => handleScenarioRemove(i)}
								onduplicate={() => duplicateScenario(i)}
								canRemove={scenarios.length > 1}
								onpackchange={loadPackManifest}
								initiallyCollapsed={mode === 'edit' && scenarios.length > 1}
							/>
						{/each}

						<Button variant="outline" class="w-full" onclick={addScenario}>
							<PlusIcon data-icon="inline-start" />
							Add Scenario
						</Button>
					</div>
				{/if}
			{:else}
				<div class="rounded-lg border overflow-hidden">
					<YamlEditor bind:value={yamlText} onchange={() => markDirty()} />
				</div>
			{/if}
		</div>
	</main>
</div>

<!-- Unsaved changes confirmation -->
<Dialog.Root bind:open={leaveDialogOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Discard unsaved changes?</Dialog.Title>
			<Dialog.Description>
				You have unsaved edits. Leaving will discard them. This cannot be undone.
			</Dialog.Description>
		</Dialog.Header>
		<div class="flex justify-end gap-2 pt-4">
			<Button
				variant="outline"
				onclick={() => {
					leaveDialogOpen = false;
					pendingNavigate = null;
				}}
			>
				Stay
			</Button>
			<Button variant="destructive" onclick={discardAndLeave}>Discard</Button>
		</div>
	</Dialog.Content>
</Dialog.Root>
