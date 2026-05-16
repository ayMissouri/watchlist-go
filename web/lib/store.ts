import { create } from "zustand"

interface User {
  id: string
  username: string
  avatar?: string
}

interface AuthStore {
  token: string | null
  user: User | null
  setAuth: (token: string, user: User) => void
  clearAuth: () => void
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthStore>((set, get) => ({
  token: null,
  user: null,

  setAuth: (token, user) => set({ token, user }),

  clearAuth: () => set({ token: null, user: null }),

  isAuthenticated: () => get().token !== null,
}))