/**
 * Format a timestamp string for message bubbles (with time).
 * - Today:          "HH:MM"
 * - Yesterday:      "Yesterday HH:MM"
 * - This year:      "MM-DD HH:MM"
 * - Older:          "YYYY-MM-DD HH:MM"
 */
export function formatRelativeTime(dateStr: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return ''

  const now = new Date()
  const timeStr = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const target = new Date(d.getFullYear(), d.getMonth(), d.getDate())
  const diffDays = Math.floor((today.getTime() - target.getTime()) / (1000 * 60 * 60 * 24))

  if (diffDays === 0) {
    return timeStr
  } else if (diffDays === 1) {
    return `Yesterday ${timeStr}`
  } else if (d.getFullYear() === now.getFullYear()) {
    const month = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    return `${month}-${day} ${timeStr}`
  } else {
    const year = d.getFullYear()
    const month = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    return `${year}-${month}-${day} ${timeStr}`
  }
}

/**
 * Format a timestamp for the session sidebar (compact, no time).
 * - Today:     "HH:MM"
 * - Yesterday: "Yesterday"
 * - This year: "MM-DD"
 * - Older:     "YYYY-MM-DD"
 */
export function formatSessionTime(dateStr: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return ''

  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const target = new Date(d.getFullYear(), d.getMonth(), d.getDate())
  const diffDays = Math.floor((today.getTime() - target.getTime()) / (1000 * 60 * 60 * 24))

  if (diffDays === 0) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } else if (diffDays === 1) {
    return 'Yesterday'
  } else if (d.getFullYear() === now.getFullYear()) {
    const month = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    return `${month}-${day}`
  } else {
    const year = d.getFullYear()
    const month = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    return `${year}-${month}-${day}`
  }
}
