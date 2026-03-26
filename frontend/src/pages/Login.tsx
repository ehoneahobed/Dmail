import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import { saveToken, saveAddress } from '../auth'

interface Props {
  onLogin: () => void
}

export default function Login({ onLogin }: Props) {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await api.login(username, password)
      saveToken(res.token)
      saveAddress(res.address)
      onLogin()
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="onboarding">
      <h1>Dmail</h1>
      <p className="subtitle">Sign in to your account</p>

      {error && <div className="error-banner" style={{ maxWidth: 400, marginBottom: '1rem' }}>{error}</div>}

      <form onSubmit={handleSubmit} style={{ width: '100%', maxWidth: 400 }}>
        <div className="field" style={{ marginBottom: '1rem' }}>
          <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
            Username
          </label>
          <input
            type="text"
            value={username}
            onChange={e => setUsername(e.target.value)}
            placeholder="username"
            autoFocus
          />
        </div>
        <div className="field" style={{ marginBottom: '1.5rem' }}>
          <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
            Password
          </label>
          <input
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            placeholder="password"
          />
        </div>
        <button className="primary" type="submit" disabled={!username || !password || loading} style={{ width: '100%' }}>
          {loading ? 'Signing in...' : 'Sign In'}
        </button>
      </form>

      <p style={{ marginTop: '1.5rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
        Don't have an account?{' '}
        <a
          href="#"
          onClick={e => { e.preventDefault(); navigate('/signup') }}
          style={{ color: 'var(--accent)' }}
        >
          Sign up
        </a>
      </p>
    </div>
  )
}
