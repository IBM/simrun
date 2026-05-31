<script lang="ts" module>
	export interface SSHFormFields {
		host: string;
		username: string;
		port: string;
	}
	export function emptySSHFields(): SSHFormFields {
		return { host: '', username: '', port: '' };
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
		fields: SSHFormFields;
		secretGroupId: string;
		idPrefix?: string;
	} = $props();

	export function validate(): boolean {
		if (!fields.host.trim() || !fields.username.trim()) return false;
		if (fields.port.trim()) {
			const parsed = Number(fields.port.trim());
			if (Number.isNaN(parsed) || parsed < 0 || parsed > 65535) return false;
		}
		return !!secretGroupId;
	}

	export function buildConfig(): Record<string, unknown> {
		const cfg: Record<string, unknown> = {
			host: fields.host.trim(),
			username: fields.username.trim()
		};
		const port = fields.port.trim();
		if (port) {
			const parsed = Number(port);
			if (!Number.isNaN(parsed) && parsed > 0) cfg.port = parsed;
		}
		return cfg;
	}

	export function populate(connector: Connector): void {
		const cfg = connector.config as Record<string, unknown>;
		fields.host = (cfg.host as string) || '';
		fields.username = (cfg.username as string) || '';
		const port = cfg.port;
		fields.port = typeof port === 'number' && port > 0 ? String(port) : '';
	}

	export function canTest(): boolean {
		return false;
	}

	const secretGroupLabel = $derived(
		secretGroupId
			? ($secrets.find((s) => s.id === secretGroupId)?.name ?? 'Unknown')
			: 'Select secret group'
	);

	const hostId = $derived(idPrefix ? `${idPrefix}SshHost` : 'sshHost');
	const userId = $derived(idPrefix ? `${idPrefix}SshUsername` : 'sshUsername');
	const portId = $derived(idPrefix ? `${idPrefix}SshPort` : 'sshPort');
</script>

<div class="space-y-2">
	<Label for={hostId}>Host</Label>
	<Input id={hostId} placeholder="ssh.example.com" bind:value={fields.host} />
</div>
<div class="space-y-2">
	<Label for={userId}>Username</Label>
	<Input id={userId} placeholder="ubuntu" bind:value={fields.username} />
</div>
<div class="space-y-2">
	<Label for={portId}>Port (optional)</Label>
	<Input id={portId} type="number" min="1" max="65535" placeholder="22" bind:value={fields.port} />
	<p class="text-xs text-muted-foreground">Leave empty to use the default SSH port (22)</p>
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
	<p class="text-xs text-muted-foreground">
		Select a secret group containing SR_SSH_KEY (private key, PEM-encoded)
	</p>
</div>
