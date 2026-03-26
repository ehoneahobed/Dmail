interface Props {
  textareaRef: React.RefObject<HTMLTextAreaElement | null>
  value: string
  onChange: (value: string) => void
}

function insertAtCursor(
  textarea: HTMLTextAreaElement,
  value: string,
  before: string,
  after: string,
  onChange: (v: string) => void,
) {
  const start = textarea.selectionStart
  const end = textarea.selectionEnd
  const selected = value.substring(start, end)
  const replacement = before + (selected || 'text') + after
  const newValue = value.substring(0, start) + replacement + value.substring(end)
  onChange(newValue)
  // Restore cursor position after React re-render.
  requestAnimationFrame(() => {
    textarea.focus()
    const cursorPos = start + before.length + (selected ? selected.length : 4)
    textarea.setSelectionRange(cursorPos, cursorPos)
  })
}

export default function FormattingToolbar({ textareaRef, value, onChange }: Props) {
  const insert = (before: string, after: string) => {
    if (textareaRef.current) {
      insertAtCursor(textareaRef.current, value, before, after, onChange)
    }
  }

  return (
    <div className="formatting-toolbar">
      <button type="button" title="Bold" onClick={() => insert('**', '**')}>
        <strong>B</strong>
      </button>
      <button type="button" title="Italic" onClick={() => insert('_', '_')}>
        <em>I</em>
      </button>
      <button type="button" title="Code" onClick={() => insert('`', '`')}>
        {'</>'}
      </button>
      <button type="button" title="Link" onClick={() => insert('[', '](url)')}>
        Link
      </button>
      <button type="button" title="List" onClick={() => insert('\n- ', '')}>
        List
      </button>
    </div>
  )
}
