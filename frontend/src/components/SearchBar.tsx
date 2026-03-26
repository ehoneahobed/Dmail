import { useState, useEffect, useRef } from 'react'

interface Props {
  onSearch: (query: string) => void
  placeholder?: string
}

export default function SearchBar({ onSearch, placeholder = 'Search messages...' }: Props) {
  const [query, setQuery] = useState('')
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (timerRef.current) clearTimeout(timerRef.current)
    timerRef.current = setTimeout(() => {
      onSearch(query.trim())
    }, 300)
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, [query, onSearch])

  return (
    <div className="search-bar">
      <input
        type="text"
        value={query}
        onChange={e => setQuery(e.target.value)}
        placeholder={placeholder}
      />
      {query && (
        <button
          className="search-clear"
          onClick={() => {
            setQuery('')
            onSearch('')
          }}
        >
          x
        </button>
      )}
    </div>
  )
}
