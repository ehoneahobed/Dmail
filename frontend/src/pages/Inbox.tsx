import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, Message } from '../api'
import Avatar from '../components/Avatar'
import SearchBar from '../components/SearchBar'
import DeliveryStatus from '../components/DeliveryStatus'

interface Props {
  folder: string
  resolveName: (pubkey: string) => string
}

function formatTime(ts: number): string {
  const d = new Date(ts * 1000)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

export default function Inbox({ folder, resolveName }: Props) {
  const [messages, setMessages] = useState<Message[]>([])
  const [searchResults, setSearchResults] = useState<Message[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const navigate = useNavigate()

  useEffect(() => {
    setLoading(true)
    setError('')
    setSearchResults(null)
    api
      .listMessages(folder)
      .then(setMessages)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [folder])

  const handleSearch = useCallback((query: string) => {
    if (!query) {
      setSearchResults(null)
      return
    }
    api
      .searchMessages(query)
      .then(setSearchResults)
      .catch(() => setSearchResults([]))
  }, [])

  const title = folder.charAt(0).toUpperCase() + folder.slice(1)
  const displayMessages = searchResults !== null ? searchResults : messages

  return (
    <div>
      <div className="page-header">
        <h2>{title}</h2>
        <button className="primary" onClick={() => navigate('/compose')}>
          Compose
        </button>
      </div>

      <SearchBar onSearch={handleSearch} />

      {error && <div className="error-banner">{error}</div>}

      {loading ? (
        <div className="empty-state-card">
          <div className="spinner" style={{ margin: '0 auto' }} />
        </div>
      ) : displayMessages.length === 0 ? (
        <div className="empty-state-card">
          <div className="empty-icon">{searchResults !== null ? '\uD83D\uDD0D' : '\u2709'}</div>
          <h3>{searchResults !== null ? 'No results found' : `No messages in ${folder}`}</h3>
          <p>{searchResults !== null ? 'Try a different search term.' : folder === 'inbox' ? 'Messages you receive will appear here.' : 'Messages you send will appear here.'}</p>
        </div>
      ) : (
        <ul className="message-list">
          {displayMessages.map(msg => {
            const displayAddr = folder === 'sent' ? msg.recipient : msg.sender
            return (
              <li
                key={msg.id}
                className={`message-item ${!msg.is_read && folder === 'inbox' ? 'unread' : ''}`}
                onClick={() => navigate(`/message/${msg.id}`)}
              >
                <Avatar address={displayAddr} size={36} />
                <span className="sender">
                  {resolveName(displayAddr)}
                </span>
                <span className="subject">
                  {msg.subject}
                  {msg.thread_id && msg.thread_id !== msg.id && (
                    <button
                      className="thread-badge"
                      onClick={e => {
                        e.stopPropagation()
                        navigate(`/thread/${msg.thread_id}`)
                      }}
                    >
                      Thread
                    </button>
                  )}
                </span>
                <span className="time">
                  {folder === 'sent' && msg.status && (
                    <DeliveryStatus status={msg.status} />
                  )}
                  {formatTime(msg.timestamp)}
                </span>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
