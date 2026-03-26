const TOKEN_KEY = 'dmail_token'
const ADDRESS_KEY = 'dmail_address'

export function saveToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function saveAddress(address: string) {
  localStorage.setItem(ADDRESS_KEY, address)
}

export function getAddress(): string | null {
  return localStorage.getItem(ADDRESS_KEY)
}

export function clearAuth() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(ADDRESS_KEY)
}

export function isLoggedIn(): boolean {
  return !!getToken()
}
