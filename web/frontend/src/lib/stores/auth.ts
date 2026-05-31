import { writable } from 'svelte/store';
import type { User } from '$lib/types';

interface AuthState {
	user: User | null;
	loading: boolean;
}

function createAuthStore() {
	const { subscribe, set, update } = writable<AuthState>({
		user: null,
		loading: true
	});

	return {
		subscribe,
		setUser: (user: User | null) => update((state) => ({ ...state, user, loading: false })),
		setLoading: (loading: boolean) => update((state) => ({ ...state, loading })),
		logout: async () => {
			await fetch('/api/auth/logout', { method: 'POST' });
			set({ user: null, loading: false });
			window.location.href = '/login';
		}
	};
}

export const auth = createAuthStore();
