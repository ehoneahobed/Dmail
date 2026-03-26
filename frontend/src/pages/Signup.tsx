import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import { saveToken, saveAddress } from '../auth'

interface Props {
  onLogin: () => void
}

export default function Signup({ onLogin }: Props) {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [mnemonic, setMnemonic] = useState<string | null>(null)
  const [address, setAddress] = useState<string | null>(null)
  const [pendingToken, setPendingToken] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await api.signup(username, password, email)
      // Don't save token yet — wait until user confirms they saved the mnemonic.
      setPendingToken(res.token)
      setMnemonic(res.mnemonic)
      setAddress(res.address)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleConfirmMnemonic = () => {
    if (pendingToken && address) {
      saveToken(pendingToken)
      saveAddress(address)
    }
    onLogin()
  }

  // After signup, show the mnemonic once.
  if (mnemonic) {
    return (
      <div className="onboarding">
        <h1>Welcome to Dmail</h1>
        <p className="subtitle">Your account has been created!</p>

        <div className="address-display">
          Your address: <strong>{address}</strong>
        </div>

        <p className="warning">
          Write down your recovery phrase. This is the ONLY way to recover your identity if you lose access.
          It will not be shown again.
        </p>

        <div className="mnemonic-box">
          {mnemonic.split(' ').map((word, i) => (
            <span key={i}>
              <span style={{ color: 'var(--text-muted)', fontSize: '0.8em' }}>{i + 1}.</span> {word}{' '}
            </span>
          ))}
        </div>

        <button
          className="primary"
          style={{ marginTop: '1.5rem' }}
          onClick={handleConfirmMnemonic}
        >
          I've saved my recovery phrase — Enter Dmail
        </button>
      </div>
    )
  }

  return (
    <div className="onboarding">
      <h1>Dmail</h1>
      <p className="subtitle">Create a new account</p>

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
            placeholder="alice"
            autoFocus
          />
        </div>
        <div className="field" style={{ marginBottom: '1rem' }}>
          <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
            Email (optional)
          </label>
          <input
            type="email"
            value={email}
            onChange={e => setEmail(e.target.value)}
            placeholder="alice@example.com"
          />
        </div>
        <div className="field" style={{ marginBottom: '1.5rem' }}>
          <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
            Password (min 8 characters)
          </label>
          <input
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            placeholder="password"
          />
        </div>
        <button
          className="primary"
          type="submit"
          disabled={!username || password.length < 8 || loading}
          style={{ width: '100%' }}
        >
          {loading ? 'Creating account...' : 'Create Account'}
        </button>
      </form>

      <p style={{ marginTop: '1.5rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
        Already have an account?{' '}
        <a
          href="#"
          onClick={e => { e.preventDefault(); navigate('/login') }}
          style={{ color: 'var(--accent)' }}
        >
          Sign in
        </a>
      </p>
    </div>
  )
}
