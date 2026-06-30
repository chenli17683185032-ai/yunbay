/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { fromHtml } from 'hast-util-from-html'
import { defaultSchema, sanitize, type Schema } from 'hast-util-sanitize'
import { toHtml } from 'hast-util-to-html'

const SAFE_PROTOCOLS = ['http', 'https', 'mailto']
const SAFE_IMAGE_PROTOCOLS = ['http', 'https']

export const safeHtmlSchema: Schema = {
  ...defaultSchema,
  protocols: {
    ...defaultSchema.protocols,
    href: SAFE_PROTOCOLS,
    src: SAFE_IMAGE_PROTOCOLS,
    longDesc: SAFE_IMAGE_PROTOCOLS,
    cite: ['http', 'https'],
  },
}

export function sanitizeHtml(html: string): string {
  const tree = fromHtml(html, { fragment: true })
  const cleanTree = sanitize(tree, safeHtmlSchema)
  return toHtml(cleanTree)
}

export function sanitizeMarkdownUrl(value: string): string {
  return sanitizeUrl(value) ?? ''
}

export function sanitizeMarkdownHref(value?: string): string | undefined {
  if (!value) {
    return undefined
  }
  return sanitizeUrl(value)
}

function sanitizeUrl(value: string): string | undefined {
  const trimmed = value.trim()
  const lower = trimmed.toLowerCase()

  if (
    lower.startsWith('#') ||
    lower.startsWith('/') ||
    lower.startsWith('./') ||
    lower.startsWith('../')
  ) {
    return trimmed
  }

  try {
    const parsed = new URL(trimmed)
    if (SAFE_PROTOCOLS.includes(parsed.protocol.slice(0, -1))) {
      return trimmed
    }
  } catch {
    return undefined
  }

  return undefined
}
