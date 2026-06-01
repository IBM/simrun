import { writable } from 'svelte/store';
import type { Connector } from '$lib/types';
import { listConnectors } from '$lib/api/client';

export const connectors = writable<Connector[]>([]);

export async function loadConnectors() {
	const data = await listConnectors();
	connectors.set(data);
}
