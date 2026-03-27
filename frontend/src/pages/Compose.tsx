import { useState, useEffect, useRef } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { api, Contact, Identity } from '../api'
import FormattingToolbar from '../components/FormattingToolbar'
import MarkdownRenderer from '../components/MarkdownRenderer'

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
  const [showPreview, setShowPreview] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const replyId = searchParams.get('reply') || undefined

  useEffect(() => {
    if (replyId) {
      setSubject(s => (s ? s : 'Re: '))
    }
  }, [replyId])

  const isFederatedAddress = /^[a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/.test(recipient)
  const isValidAddress = (recipient.startsWith('dmail:') && recipient.length > 20) || isFederatedAddress

  const matchedContact = contacts.find(
    c => c.petname.toLowerCase() === recipient.toLowerCase(),
  )

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

  const canSend = subject && (isValidAddress || !!matchedContact || !!resolvedDmailName) && !sending

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
      <div style={{ maxWidth: 600 }}>
        <div className="success-screen">
          <div className="success-icon">&#10003;</div>
          <h2>Message Accepted</h2>
          <p>Your message is being stamped with proof-of-work and will be delivered shortly.</p>
          <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'center', marginTop: '1.5rem' }}>
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
    <div style={{ maxWidth: 650 }}>
      <div className="page-header">
        <h2>{replyId ? 'Reply' : 'New Message'}</h2>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="card">
        <div className="field" style={{ marginBottom: '1rem' }}>
          <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem', fontWeight: 500 }}>
            To
          </label>
          <input
            type="text"
            placeholder="alice@server.com, alice.dmail, or contact name"
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
            <p className="field-hint field-hint-success">Resolved: {matchedContact.pubkey}</p>
          )}
          {resolving && (
            <p className="field-hint">Resolving name...</p>
          )}
          {resolvedDmailName && !resolving && (
            <p className="field-hint field-hint-success">Resolved: {resolvedDmailName}</p>
          )}
          {isFederatedAddress && (
            <p className="field-hint field-hint-success">
              Federation: will deliver to {recipient.split('@')[1]}
            </p>
          )}
        </div>

        <div className="field" style={{ marginBottom: '1rem' }}>
          <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem', fontWeight: 500 }}>
            Subject
          </label>
          <input
            type="text"
            placeholder="What's this about?"
            value={subject}
            onChange={e => setSubject(e.target.value)}
          />
        </div>

        <div className="field" style={{ marginBottom: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.25rem' }}>
            <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)', fontWeight: 500 }}>
              Message
            </label>
            <button
              type="button"
              className="secondary"
              style={{ padding: '0.15rem 0.5rem', fontSize: '0.7rem' }}
              onClick={() => setShowPreview(!showPreview)}
            >
              {showPreview ? 'Edit' : 'Preview'}
            </button>
          </div>
          {showPreview ? (
            <div className="preview-box">
              <MarkdownRenderer content={body || '*Nothing to preview*'} />
            </div>
          ) : (
            <>
              <FormattingToolbar textareaRef={textareaRef} value={body} onChange={setBody} />
              <textarea
                ref={textareaRef}
                placeholder="Write your message... (Markdown supported: **bold**, _italic_, `code`, [links](url))"
                value={body}
                onChange={e => setBody(e.target.value)}
                rows={10}
              />
            </>
          )}
        </div>

        {sending && (
          <div className="stamping">
            <div className="spinner" />
            Stamping your message with proof-of-work...
          </div>
        )}

        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '1rem' }}>
          <p style={{ fontSize: '0.75rem', color: 'var(--text-muted)', margin: 0 }}>
            From: {identity.address.slice(0, 20)}...
          </p>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button className="secondary" onClick={() => navigate(-1)}>
              Cancel
            </button>
            <button className="primary" disabled={!canSend} onClick={handleSend}>
              {sending ? 'Sending...' : 'Send Message'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
