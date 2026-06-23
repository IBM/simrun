<script lang="ts" module>
	export interface AWSFormFields {
		roleArn: string;
	}
	export function emptyAWSFields(): AWSFormFields {
		return { roleArn: '' };
	}
</script>

<script lang="ts">
	import * as Select from '$lib/components/ui/select/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { secrets } from '$lib/stores/secrets';
	import type { Connector } from '$lib/types';

	let {
		fields = $bindable(),
		secretGroupId = $bindable(),
		idPrefix = ''
	}: {
		fields: AWSFormFields;
		secretGroupId: string;
		idPrefix?: string;
	} = $props();

	export function validate(): boolean {
		return !!fields.roleArn.trim();
	}

	export function buildConfig(): Record<string, unknown> {
		return { role_arn: fields.roleArn.trim() };
	}

	export function populate(connector: Connector): void {
		const cfg = connector.config as Record<string, unknown>;
		fields.roleArn = (cfg.role_arn as string) || '';
	}

	export function canTest(): boolean {
		return true;
	}

	const secretGroupLabel = $derived(
		secretGroupId
			? ($secrets.find((s) => s.id === secretGroupId)?.name ?? 'Unknown')
			: 'Select secret group'
	);

	const roleArnId = $derived(idPrefix ? `${idPrefix}RoleArn` : 'roleArn');
</script>

<div class="space-y-2">
	<Label for={roleArnId}>Role ARN</Label>
	<Input
		id={roleArnId}
		placeholder="arn:aws:iam::123456789012:role/SimulationRole"
		bind:value={fields.roleArn}
	/>
</div>
<div class="space-y-2">
	<Label>Secret Group</Label>
	<Select.Root type="single" bind:value={secretGroupId}>
		<Select.Trigger class="w-full">{secretGroupLabel}</Select.Trigger>
		<Select.Content>
			{#each $secrets as secret}
				<Select.Item value={secret.id} label={secret.name} />
			{/each}
		</Select.Content>
	</Select.Root>
	<p class="text-xs text-muted-foreground">Select a secret group containing SR_AWS_EXTERNAL_ID</p>
</div>
