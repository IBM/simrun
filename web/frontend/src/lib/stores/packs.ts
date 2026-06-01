import { writable } from 'svelte/store';
import type { Pack } from '$lib/types';
import { listPacks } from '$lib/api/client';

export const packs = writable<Pack[]>([]);

export async function loadPacks() {
	const data = await listPacks();
	packs.set(data);
}
