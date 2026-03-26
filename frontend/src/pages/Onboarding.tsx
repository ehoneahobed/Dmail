import { useState } from 'react'
import { Identity } from '../api'

interface Props {
  identity: Identity
  onComplete: () => void
}

export default function Onboarding({ identity, onComplete }: Props) {
  const [confirmed, setConfirmed] = useState(false)
  const [confirmInput, setConfirmInput] = useState('')

  // Extract the 3rd word from mnemonic for confirmation.
  const words = identity.mnemonic.split(' ')
  const confirmWord = words[2] // 3rd word (0-indexed)
  const isCorrect = confirmInput.toLowerCase().trim() === confirmWord.toLowerCase()

  return (
    <div className="onboarding">
      <h1>Welcome to Dmail</h1>
      <p className="subtitle">Your decentralized, encrypted messaging identity has been created.</p>

      <p className="address-display">
        Your address: <strong>{identity.address}</strong>
      </p>

      <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem', marginBottom: '1rem' }}>
        Write down your 12-word recovery phrase. This is the only way to restore your identity.
      </p>

      <div className="mnemonic-box">
        {words.map((w, i) => (
          <span key={i}>
            <span style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>{i + 1}.</span>{' '}
            {w}{' '}
          </span>
        ))}
      </div>

      <p className="warning">
        If you lose this phrase, your identity and all future messages are permanently lost.
        There is no recovery mechanism.
      </p>

      {!confirmed ? (
        <div style={{ maxWidth: 400, width: '100%' }}>
          <label style={{ display: 'block', fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '0.5rem' }}>
            To confirm you saved it, enter word #{3} from your phrase:
          </label>
          <input
            type="text"
            placeholder={`Word #3`}
            value={confirmInput}
            onChange={e => setConfirmInput(e.target.value)}
            style={{ marginBottom: '1rem' }}
          />
          <button
            className="primary"
            disabled={!isCorrect}
            onClick={() => setConfirmed(true)}
            style={{ width: '100%' }}
          >
            {isCorrect ? 'Confirmed — Continue' : 'Enter word #3 to continue'}
          </button>
        </div>
      ) : (
        <button className="primary" onClick={onComplete} style={{ padding: '0.75rem 2rem', fontSize: '1rem' }}>
          Enter Dmail
        </button>
      )}
    </div>
  )
}
