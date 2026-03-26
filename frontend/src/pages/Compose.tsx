import { useState, useEffect } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { api, Contact, Identity } from '../api'

interface Props {
  contacts: Contact[]
  identity: Identity
}

export default function Compose({ contacts, identity }: Props) {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()

  const [recipient, setRecipient] = useState(searchParams.get('to') || '')
  const [subject, setSubject] = useState('')
  const [body, setBody] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const [sent, setSent] = useState(false)

  const replyId = searchParams.get('reply') || undefined

  useEffect(() => {
    if (replyId) {
      setSubject(s => (s ? s : 'Re: '))
    }
  }, [replyId])

  // Basic address validation.
  const isValidAddress = recipient.startsWith('dmail:') && recipient.length > 20
  const isDmailName = recipient.endsWith('.dmail') || /^[a-z0-9][a-z0-9-]*[a-z0-9]$/.test(recipient)

  // Check if input matches a contact petname.
  const matchedContact = contacts.find(
    c => c.petname.toLowerCase() === recipient.toLowerCase(),
  )

  // .dmail name resolution state
  const [resolvedDmailName, setResolvedDmailName] = useState<string | null>(null)
  const [resolving, setResolving] = useState(false)

  useEffect(() => {
    if (recipient.endsWith('.dmail')) {
      setResolving(true)
      setResolvedDmailName(null)
      api.resolveName(recipient)
        .then(r => setResolvedDmailName(r.address))
        .catch(() => setResolvedDmailName(null))
        .finally(() => setResolving(false))
    } else {
      setResolvedDmailName(null)
    }
  }, [recipient])

  const resolvedRecipient = matchedContact
    ? matchedContact.pubkey
    : resolvedDmailName || recipient

  const handleSend = async () => {
    setError('')
    setSending(true)
    try {
      await api.sendMessage(resolvedRecipient, subject, body, replyId)
      setSent(true)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setSending(false)
    }
  }

  if (sent) {
    return (
      <div className="compose-form">
        <div style={{ textAlign: 'center', padding: '3rem 0' }}>
          <p style={{ fontSize: '1.25rem', marginBottom: '0.5rem' }}>Message accepted</p>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginBottom: '1.5rem' }}>
            Your message is being stamped and will be sent shortly.
          </p>
          <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'center' }}>
            <button className="primary" onClick={() => navigate('/')}>
              Go to Inbox
            </button>
            <button
              className="secondary"
              onClick={() => {
                setSent(false)
                setRecipient('')
                setSubject('')
                setBody('')
              }}
            >
              Send Another
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="compose-form">
      <div className="page-header">
        <h2>Compose</h2>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="field">
        <label>To (Dmail address, contact name, or alice.dmail)</label>
        <input
          type="text"
          placeholder="alice.dmail, dmail:abc123..., or Alice"
          value={recipient}
          onChange={e => setRecipient(e.target.value)}
          list="contacts-list"
        />
        <datalist id="contacts-list">
          {contacts.map(c => (
            <option key={c.pubkey} value={c.petname}>
              {c.pubkey}
            </option>
          ))}
        </datalist>
        {matchedContact && (
          <p style={{ fontSize: '0.75rem', color: 'var(--success)', marginTop: '0.25rem' }}>
            Resolved to: {matchedContact.pubkey}
          </p>
        )}
        {resolving && (
          <p style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>
            Resolving name...
          </p>
        )}
        {resolvedDmailName && !resolving && (
          <p style={{ fontSize: '0.75rem', color: 'var(--success)', marginTop: '0.25rem' }}>
            Resolved to: {resolvedDmailName}
          </p>
        )}
      </div>

      <div className="field">
        <label>Subject</label>
        <input
          type="text"
          placeholder="Message subject"
          value={subject}
          onChange={e => setSubject(e.target.value)}
        />
      </div>

      <div className="field">
        <label>Body</label>
        <textarea
          placeholder="Write your message..."
          value={body}
          onChange={e => setBody(e.target.value)}
          rows={8}
        />
      </div>

      {sending && (
        <div className="stamping">
          <div className="spinner" />
          Stamping your message... This prevents spam.
        </div>
      )}

      <div className="actions">
        <button
          className="primary"
          disabled={!subject || (!isValidAddress && !matchedContact && !resolvedDmailName) || sending}
          onClick={handleSend}
        >
          {sending ? 'Sending...' : 'Send'}
        </button>
        <button className="secondary" onClick={() => navigate(-1)}>
          Cancel
        </button>
      </div>

      <p style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '1rem' }}>
        Sending from: {identity.address}
      </p>
    </div>
  )
}
