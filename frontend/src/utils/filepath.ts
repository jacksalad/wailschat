/**
 * Extract the filename from a file path, handling both Windows backslash
 * and Unix forward slash separators.
 */
export function extractFilename(filePath: string): string {
  if (!filePath) return ''
  const normalized = filePath.replace(/\\/g, '/')
  const segments = normalized.split('/')
  return segments[segments.length - 1] || ''
}
