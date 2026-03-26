// Dynamic base URL: Electron IPC → web gateway override → same-origin fallback
let BASE = '/api/v1'
let baseResolved = false
let basePromise: Promise<void> | null = null

function resolveBase(): Promise<void> {
  if (baseResolved) return Promise.resolve()
  if (basePromise) return basePromise

  basePromise = (async () => {
    if ((window as any).dmailBridge) {
      try {
        const port = await (window as any).dmailBridge.getDaemonPort()
        BASE = `http://127.0.0.1:${port}/api/v1`
      } catch {
        BASE = 'http://127.0.0.1:7777/api/v1'
      }
    } else if ((window as any).__DMAIL_API_BASE__) {
      BASE = (window as any).__DMAIL_API_BASE__
    } else {
      // same-origin mode (web service) — try relative path, fall back to localhost
      try {
        const res = await fetch('/api/v1/status', { signal: AbortSignal.timeout(2000) })
        if (res.ok) {
          BASE = '/api/v1'
        } else {
          BASE = 'http://127.0.0.1:7777/api/v1'
        }
      } catch {
        BASE = 'http://127.0.0.1:7777/api/v1'
      }
    }
    baseResolved = true
  })()
  return basePromise
}

export interface Message {
  id: string
  folder: string
  sender: string
  recipient: string
  subject: string
  body: string
  timestamp: number
  is_read: boolean
  reply_to_id?: string
  thread_id?: string
  status?: string
}

export interface User {
  id: string
  username: string
  pubkey: string
  created_at: number
}

export interface Contact {
  pubkey: string
  petname: string
  created_at: number
}

export interface Status {
  connected_peers: number
  is_syncing: boolean
  pending_pow_tasks: number
  address: string
}

export interface Identity {
  address: string
  mnemonic: string
}

async function request<T>(path: string, opts?: RequestInit): Promise<T> {
  await resolveBase()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  // Inject auth token if available (for multi-tenant web mode)
  const token = typeof localStorage !== 'undefined' ? localStorage.getItem('dmail_token') : null
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  const res = await fetch(`${BASE}${path}`, {
    headers,
    ...opts,
  })
  if (!res.ok && res.status !== 204) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  getStatus: () => request<Status>('/status'),
  getIdentity: () => request<Identity>('/identity'),

  listMessages: (folder = 'inbox') =>
    request<{ messages: Message[] }>(`/messages?folder=${folder}`).then(r => r.messages),

  getMessage: (id: string) => request<Message>(`/messages/${id}`),

  sendMessage: (recipient: string, subject: string, body: string, replyToId?: string) =>
    request<{ status: string }>('/messages/send', {
      method: 'POST',
      body: JSON.stringify({ recipient, subject, body, reply_to_id: replyToId }),
    }),

  markRead: (id: string) =>
    request<void>(`/messages/${id}/read`, { method: 'PUT' }),

  deleteMessage: (id: string) =>
    request<void>(`/messages/${id}`, { method: 'DELETE' }),

  listContacts: () =>
    request<{ contacts: Contact[] }>('/contacts').then(r => r.contacts),

  saveContact: (pubkey: string, petname: string) =>
    request<Contact>('/contacts', {
      method: 'POST',
      body: JSON.stringify({ pubkey, petname }),
    }),

  deleteContact: (pubkey: string) =>
    request<void>(`/contacts/${encodeURIComponent(pubkey)}`, { method: 'DELETE' }),

  registerName: (name: string) =>
    request<{ status: string; name: string }>('/names/register', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  resolveName: (name: string) =>
    request<{ name: string; address: string }>(`/names/resolve/${encodeURIComponent(name.replace(/\.dmail$/, ''))}`),

  getMyNames: () =>
    request<{ names: Array<{ name: string; pubkey: string; timestamp: number; is_mine: boolean }> }>('/names/mine').then(r => r.names),

  getUnreadCount: (folder = 'inbox') =>
    request<{ count: number }>(`/messages/unread-count?folder=${folder}`).then(r => r.count),

  searchMessages: (query: string) =>
    request<{ messages: Message[] }>(`/messages/search?q=${encodeURIComponent(query)}`).then(r => r.messages),

  getThread: (id: string) =>
    request<{ messages: Message[] }>(`/messages/thread/${id}`).then(r => r.messages),

  listUsers: () =>
    request<{ users: User[] }>('/users').then(r => r.users),

  // Auth endpoints (multi-tenant mode).
  signup: (username: string, password: string, email?: string) =>
    request<{ token: string; address: string; mnemonic: string; user_id: string }>('/auth/signup', {
      method: 'POST',
      body: JSON.stringify({ username, password, email: email || '' }),
    }),

  login: (username: string, password: string) =>
    request<{ token: string; address: string; user_id: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
}
