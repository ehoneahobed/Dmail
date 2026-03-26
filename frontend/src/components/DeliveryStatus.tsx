interface Props {
  status: string
}

export default function DeliveryStatus({ status }: Props) {
  if (!status) return null

  let icon: string
  let label: string
  let className: string

  switch (status) {
    case 'sending':
      icon = '\u2022'     // bullet
      label = 'Sending'
      className = 'status-sending'
      break
    case 'sent':
      icon = '\u2713'     // single check
      label = 'Sent'
      className = 'status-sent'
      break
    case 'delivered':
      icon = '\u2713\u2713' // double check
      label = 'Delivered'
      className = 'status-delivered'
      break
    case 'read':
      icon = '\u2713\u2713' // double check (colored)
      label = 'Read'
      className = 'status-read'
      break
    default:
      return null
  }

  return (
    <span className={`delivery-status ${className}`} title={label}>
      {icon}
    </span>
  )
}
