<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import * as SidebarUI from '$lib/components/ui/sidebar/index.js';
	import { useSidebar } from '$lib/components/ui/sidebar/context.svelte.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { activeRuns } from '$lib/stores/runs';
	import { getVersion } from '$lib/api/client';
	import { health } from '$lib/stores/health';
	import LayoutDashboardIcon from '@lucide/svelte/icons/layout-dashboard';
	import PlayIcon from '@lucide/svelte/icons/play';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import PackageIcon from '@lucide/svelte/icons/package';
	import PlugIcon from '@lucide/svelte/icons/plug';
	import KeyRoundIcon from '@lucide/svelte/icons/key-round';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';

	import type { Component } from 'svelte';

	const sidebar = useSidebar();

	let version = $state('');

	const navItems: { href: string; label: string; icon: Component }[] = [
		{ href: '/', label: 'Dashboard', icon: LayoutDashboardIcon },
		{ href: '/runs', label: 'Runs', icon: PlayIcon },
		{ href: '/assessments', label: 'Assessments', icon: FileTextIcon },
		{ href: '/packs', label: 'Packs', icon: PackageIcon },
		{ href: '/rules/coverage', label: 'Rule Coverage', icon: ShieldCheckIcon },

		{ href: '/connectors', label: 'Connectors', icon: PlugIcon },
		{ href: '/secrets', label: 'Secrets', icon: KeyRoundIcon },
		{ href: '/config', label: 'Config', icon: SettingsIcon }
	];

	function isActive(href: string, pathname: string): boolean {
		if (href === '/') return pathname === '/';
		return pathname.startsWith(href);
	}

	onMount(async () => {
		try {
			const info = await getVersion();
			version = info.version;
		} catch {
			// Version display is non-critical
		}
	});
</script>

<SidebarUI.Sidebar collapsible="icon">
	<SidebarUI.SidebarHeader>
		{#if sidebar.state === 'collapsed'}
			<div class="flex items-center justify-center py-1">
				<div
					class="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-primary text-primary-foreground text-[10px] font-bold font-mono transition-shadow {!$health
						? 'shadow-[0_0_10px] shadow-status-error/60 ring-1 ring-status-error/40'
						: ''}"
				>
					SR
				</div>
			</div>
		{:else}
			<div class="flex items-center gap-2 px-2 py-2">
				<div
					class="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground text-sm font-bold font-mono tracking-tight transition-shadow {!$health
						? 'shadow-[0_0_12px] shadow-status-error/60 ring-1 ring-status-error/40'
						: ''}"
				>
					SR
				</div>
				<div class="flex flex-col overflow-hidden">
					<span class="text-sm font-semibold truncate">SimRun</span>
					<span class="text-xs text-muted-foreground truncate">Attack Simulation</span>
				</div>
			</div>
		{/if}
	</SidebarUI.SidebarHeader>
	<Separator />
	<SidebarUI.SidebarContent>
		<SidebarUI.SidebarGroup>
			<SidebarUI.SidebarGroupLabel>Navigation</SidebarUI.SidebarGroupLabel>
			<SidebarUI.SidebarGroupContent>
				<SidebarUI.SidebarMenu>
					{#each navItems as item}
						<SidebarUI.SidebarMenuItem>
							<SidebarUI.SidebarMenuButton
								isActive={isActive(item.href, $page.url.pathname)}
								tooltipContent={item.label}
							>
								{#snippet child({ props })}
									<a href={item.href} {...props}>
										<item.icon />
										<span>{item.label}</span>
									</a>
								{/snippet}
							</SidebarUI.SidebarMenuButton>
							{#if item.href === '/runs' && $activeRuns.length > 0}
								<SidebarUI.SidebarMenuBadge>
									<Badge variant="secondary" class="h-5 px-1.5 text-[10px] font-mono">
										{$activeRuns.length}
									</Badge>
								</SidebarUI.SidebarMenuBadge>
							{/if}
						</SidebarUI.SidebarMenuItem>
					{/each}
				</SidebarUI.SidebarMenu>
			</SidebarUI.SidebarGroupContent>
		</SidebarUI.SidebarGroup>
	</SidebarUI.SidebarContent>
	<SidebarUI.SidebarFooter>
		<div
			class="flex items-center gap-2 mx-1 rounded-md bg-sidebar-accent/40 px-2 py-1.5 group-data-[collapsible=icon]:hidden"
		>
			<span
				class="h-1.5 w-1.5 rounded-full shrink-0 {$health
					? 'bg-status-success animate-indicator-pulse'
					: 'bg-muted-foreground/40'}"
			></span>
			<span class="text-xs text-sidebar-foreground/70 truncate">
				{$health ? 'Connected' : 'Disconnected'}
			</span>
		</div>
		{#if version}
			<div
				class="px-2 pb-1 text-[10px] text-muted-foreground/60 font-mono truncate group-data-[collapsible=icon]:hidden"
			>
				{version}
			</div>
		{/if}
	</SidebarUI.SidebarFooter>
	<SidebarUI.SidebarRail />
</SidebarUI.Sidebar>
