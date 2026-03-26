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

  if (mnemonic) {
    return (
      <div className="auth-page">
        <div className="auth-card" style={{ maxWidth: 520 }}>
          <div className="auth-header">
            <h1>Welcome!</h1>
            <p>Your account has been created</p>
          </div>

          <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', textAlign: 'center', marginBottom: '0.5rem' }}>
            Your address
          </div>
          <div style={{
            fontSize: '0.75rem', color: 'var(--text)', wordBreak: 'break-all',
            background: 'var(--bg)', padding: '0.5rem 0.75rem', borderRadius: 'var(--radius)',
            marginBottom: '1.25rem', textAlign: 'center'
          }}>
            {address}
          </div>

          <div className="warning-banner">
            Write down your recovery phrase below. This is the <strong>only way</strong> to recover your identity. It will not be shown again.
          </div>

          <div className="mnemonic-grid">
            {mnemonic.split(' ').map((word, i) => (
              <div key={i} className="mnemonic-word">
                <span className="mnemonic-num">{i + 1}</span>
                {word}
              </div>
            ))}
          </div>

          <button className="primary" style={{ width: '100%', marginTop: '1.5rem' }} onClick={handleConfirmMnemonic}>
            I've Saved My Recovery Phrase
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-header">
          <h1>Dmail</h1>
          <p>Create your encrypted mailbox</p>
        </div>

        {error && <div className="error-banner">{error}</div>}

        <form onSubmit={handleSubmit}>
          <div className="field" style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
              Username
            </label>
            <input
              type="text"
              value={username}
              onChange={e => setUsername(e.target.value)}
              placeholder="Choose a username"
              autoFocus
            />
          </div>
          <div className="field" style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
              Email <span style={{ opacity: 0.5 }}>(optional)</span>
            </label>
            <input
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              placeholder="your@email.com"
            />
          </div>
          <div className="field" style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
              Password <span style={{ opacity: 0.5 }}>(min 8 characters)</span>
            </label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              placeholder="Choose a strong password"
            />
          </div>
          <button className="primary" type="submit" disabled={!username || password.length < 8 || loading} style={{ width: '100%' }}>
            {loading ? 'Creating account...' : 'Create Account'}
          </button>
        </form>

        <div className="auth-footer">
          Already have an account?{' '}
          <a href="#" onClick={e => { e.preventDefault(); navigate('/login') }}>
            Sign in
          </a>
        </div>
      </div>
    </div>
  )
}
