<script lang="ts">
	import '../app.css';
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { ModeWatcher, toggleMode } from 'mode-watcher';
	import { Toaster } from '$lib/components/ui/sonner/index.js';
	import { websocket } from '$lib/stores/websocket';
	import { health } from '$lib/stores/health';
	import { auth } from '$lib/stores/auth';
	import { getCurrentUser } from '$lib/api/client';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import * as SidebarUI from '$lib/components/ui/sidebar/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Avatar from '$lib/components/ui/avatar/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import SunIcon from '@lucide/svelte/icons/sun';
	import MoonIcon from '@lucide/svelte/icons/moon';
	import LogOutIcon from '@lucide/svelte/icons/log-out';

	let { children } = $props();

	onMount(async () => {
		// Skip auth check on login page
		if ($page.url.pathname === '/login') {
			auth.setLoading(false);
			return;
		}

		try {
			const user = await getCurrentUser();
			auth.setUser(user);
			websocket.connect();
			health.start();
		} catch {
			auth.setUser(null);
			goto('/login');
		}
	});

	onDestroy(() => {
		websocket.disconnect();
		health.stop();
	});
</script>

<ModeWatcher defaultMode="dark" />
<Toaster richColors closeButton position="top-right" />

{#if $page.url.pathname === '/login'}
	{@render children()}
{:else if $auth.loading}
	<div class="flex min-h-screen items-center justify-center">
		<p class="text-muted-foreground">Loading...</p>
	</div>
{:else if $auth.user}
	<SidebarUI.SidebarProvider>
		<Sidebar />
		<SidebarUI.SidebarInset>
			<header class="flex h-12 items-center justify-between border-b border-border px-4">
				<SidebarUI.SidebarTrigger />
				<div class="flex items-center gap-2">
					<Button variant="ghost" size="icon" onclick={toggleMode} aria-label="Toggle dark mode">
						<SunIcon size={16} class="block dark:hidden" />
						<MoonIcon size={16} class="hidden dark:block" />
					</Button>
					<DropdownMenu.Root>
						<DropdownMenu.Trigger>
							{#snippet child({ props })}
								<button
									{...props}
									class="flex items-center gap-2 rounded-md px-2 py-1 hover:bg-accent transition-colors"
								>
									<span class="text-sm text-muted-foreground">{$auth.user?.email}</span>
									<Avatar.Root class="h-7 w-7 shrink-0">
										{#if $auth.user?.picture}
											<Avatar.Image src={$auth.user.picture} alt={$auth.user?.name ?? ''} />
										{/if}
										<Avatar.Fallback class="text-[10px]">
											{($auth.user?.name ?? '')
												.split(' ')
												.map((n) => n[0])
												.join('')
												.toUpperCase()
												.slice(0, 2)}
										</Avatar.Fallback>
									</Avatar.Root>
								</button>
							{/snippet}
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end" class="w-56">
							<DropdownMenu.Label>
								<div class="flex flex-col space-y-1">
									<p class="text-sm font-medium leading-none">
										{$auth.user.name}
									</p>
									<p class="text-xs leading-none text-muted-foreground">
										{$auth.user.email}
									</p>
								</div>
							</DropdownMenu.Label>
							<DropdownMenu.Separator />
							<DropdownMenu.Item onclick={() => auth.logout()}>
								<LogOutIcon class="mr-2 h-4 w-4" />
								Log out
							</DropdownMenu.Item>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				</div>
			</header>
			<main class="flex-1 min-w-0">
				<div class="mx-auto w-full max-w-[1536px] p-6">
					{@render children()}
				</div>
			</main>
		</SidebarUI.SidebarInset>
	</SidebarUI.SidebarProvider>
{/if}
