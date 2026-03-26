interface Props {
  address: string
  size?: number
}

function hashCode(str: string): number {
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash + str.charCodeAt(i)) | 0
  }
  return Math.abs(hash)
}

export default function Avatar({ address, size = 36 }: Props) {
  const hash = hashCode(address)
  const hue = hash % 360
  const initial = address.startsWith('dmail:') ? address[6]?.toUpperCase() || '?' : address[0]?.toUpperCase() || '?'

  return (
    <div
      className="avatar"
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        background: `hsl(${hue}, 60%, 45%)`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: '#fff',
        fontWeight: 600,
        fontSize: size * 0.4,
        flexShrink: 0,
      }}
    >
      {initial}
    </div>
  )
}
