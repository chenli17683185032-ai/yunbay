const qrDataUrlPrefix = 'data:image/png;base64,'

export function redactValue(value: string): string {
  if (value === '') {
    return ''
  }

  if (value.startsWith(qrDataUrlPrefix)) {
    return `${qrDataUrlPrefix}[redacted]`
  }

  if (value.length >= 8) {
    return `${value.slice(0, 4)}...${value.slice(-4)}`
  }

  return '[redacted]'
}
