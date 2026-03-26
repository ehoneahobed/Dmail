import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, Message } from '../api'
import Avatar from '../components/Avatar'
import MarkdownRenderer from '../components/MarkdownRenderer'

interface Props {
  resolveName: (pubkey: string) => string
}

export default function ThreadView({ resolveName }: Props) {
  const { id } = useParams<{ id: string }>()
  const [messages, setMessages] = useState<Message[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const navigate = useNavigate()

  useEffect(() => {
    if (!id) return
    setLoading(true)
    api
      .getThread(id)
      .then(setMessages)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [id])

  if (error) {
    return (
      <div>
        <div className="error-banner">{error}</div>
        <button className="secondary" onClick={() => navigate(-1)}>Back</button>
      </div>
    )
  }

  if (loading) {
    return (
      <div className="empty-state">
        <div className="spinner" style={{ margin: '0 auto' }} />
      </div>
    )
  }

  const subject = messages.length > 0 ? messages[0].subject : 'Thread'

  return (
    <div className="thread-view">
      <button className="secondary" onClick={() => navigate(-1)} style={{ marginBottom: '1rem' }}>
        Back
      </button>
      <h2 style={{ marginBottom: '1.5rem' }}>{subject}</h2>

      {messages.length === 0 ? (
        <div className="empty-state">No messages in this thread.</div>
      ) : (
        <div className="thread-messages">
          {messages.map(msg => {
            const date = new Date(msg.timestamp * 1000)
            return (
              <div key={msg.id} className="thread-message">
                <div className="thread-message-header">
                  <Avatar address={msg.sender} size={32} />
                  <div>
                    <strong>{resolveName(msg.sender)}</strong>
                    <span className="thread-time">{date.toLocaleString()}</span>
                  </div>
                </div>
                <div className="thread-message-body">
                  <MarkdownRenderer content={msg.body} />
                </div>
              </div>
            )
          })}
        </div>
      )}

      <div style={{ marginTop: '1.5rem' }}>
        <button
          className="primary"
          onClick={() => {
            const last = messages[messages.length - 1]
            navigate(`/compose?reply=${last.id}&to=${encodeURIComponent(last.sender)}`)
          }}
        >
          Reply
        </button>
      </div>
    </div>
  )
}
