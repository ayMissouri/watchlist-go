import { useAuthStore } from "@/lib/store"

const API_URL = process.env.NEXT_PUBLIC_API_URL

// Core fetch wrapper
async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = useAuthStore.getState().token

  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `Request failed: ${res.status}`)
  }

  if (res.status === 204) return undefined as T

  return res.json()
}

// ---- Auth ----

export const authApi = {
  me: () => apiFetch<{ id: string; username: string; avatar?: string }>("/auth/me"),
}

// ---- Watchlist ----

export interface WatchlistQuery {
  page?: number
  per_page?: number
  type?: "tv" | "movie"
  sort?: "last_updated" | "title"
  order?: "asc" | "desc"
}

export interface Progress {
  watched: number
  duration: number
}

export interface EpisodeProgress {
  season: number
  episode: number
  progress: Progress
  last_updated: number
}

export interface WatchlistItem {
  id: string
  tmdb_id?: number
  type: "tv" | "movie"
  title: string
  poster_path?: string
  backdrop_path?: string
  progress: Progress
  last_season_watched?: number
  last_episode_watched?: number
  show_progress?: Record<string, EpisodeProgress>
  last_updated: number
}

export interface WatchlistResponse {
  items: WatchlistItem[]
  pagination: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

export interface UpdateProgressRequest {
  progress: Progress
  show_progress?: Record<string, EpisodeProgress>
  last_season_watched?: number
  last_episode_watched?: number
  last_updated?: number
}

export const watchlistApi = {
  getAll: (q: WatchlistQuery = {}) => {
    const params = new URLSearchParams()
    if (q.page) params.set("page", String(q.page))
    if (q.per_page) params.set("per_page", String(q.per_page))
    if (q.type) params.set("type", q.type)
    if (q.sort) params.set("sort", q.sort)
    if (q.order) params.set("order", q.order)
    return apiFetch<WatchlistResponse>(`/watchlist?${params}`)
  },

  getOne: (id: string) =>
    apiFetch<WatchlistItem>(`/watchlist/${id}`),

  upsert: (id: string, item: Omit<WatchlistItem, "id">) =>
    apiFetch<void>(`/watchlist/${id}`, {
      method: "PUT",
      body: JSON.stringify(item),
    }),

  updateProgress: (id: string, req: UpdateProgressRequest) =>
    apiFetch<void>(`/watchlist/${id}/progress`, {
      method: "PATCH",
      body: JSON.stringify(req),
    }),

  delete: (id: string) =>
    apiFetch<void>(`/watchlist/${id}`, { method: "DELETE" }),

  bulkDelete: (ids: string[]) =>
    apiFetch<{ deleted: number }>(`/watchlist`, {
      method: "DELETE",
      body: JSON.stringify({ ids }),
    }),
}

// ---- Discover ----

export interface DiscoverItem {
  id: string
  type: string
  title: string
  poster?: string
  imdb_rating?: string
  year?: string
}

export interface DiscoverAllResponse {
  popular_movies: DiscoverItem[]
  popular_shows: DiscoverItem[]
  top_rated_movies: DiscoverItem[]
  top_rated_shows: DiscoverItem[]
}

export const discoverApi = {
  all: () => apiFetch<DiscoverAllResponse>("/discover/all"),
  search: (query: string, type?: "movie" | "tv") => {
    const params = new URLSearchParams({ q: query })
    if (type) params.set("type", type)
    return apiFetch<{ items: DiscoverItem[]; query: string }>(`/search?${params}`)
  },
}

// ---- Meta ----

export const metaApi = {
  movie: (id: string) => apiFetch<{ meta: unknown }>(`/meta/movie/${id}`),
  series: (id: string) => apiFetch<{ meta: unknown }>(`/meta/series/${id}`),
}