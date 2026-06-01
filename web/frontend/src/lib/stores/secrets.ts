import { writable } from 'svelte/store';
import type { SecretGroup } from '$lib/types';
import { listSecrets } from '$lib/api/client';

export const secrets = writable<SecretGroup[]>([]);

export async function loadSecrets() {
	const data = await listSecrets();
	secrets.set(data);
}
