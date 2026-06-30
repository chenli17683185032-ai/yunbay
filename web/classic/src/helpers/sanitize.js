/*
Copyright (C) 2025 QuantumNous

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
import { fromHtml } from 'hast-util-from-html';
import { defaultSchema, sanitize } from 'hast-util-sanitize';
import { toHtml } from 'hast-util-to-html';
import { marked } from 'marked';


const SAFE_PROTOCOLS = ['http', 'https', 'mailto'];

export function sanitizeUrl(value) {
  if (!value) return undefined;
  const trimmed = value.trim();
  const lower = trimmed.toLowerCase();

  if (
    lower.startsWith('#') ||
    lower.startsWith('/') ||
    lower.startsWith('./') ||
    lower.startsWith('../')
  ) {
    return trimmed;
  }

  try {
    const parsed = new URL(trimmed);
    if (SAFE_PROTOCOLS.includes(parsed.protocol.slice(0, -1))) {
      return trimmed;
    }
  } catch {
    return undefined;
  }

  return undefined;
}

const safeHtmlSchema = {
  ...defaultSchema,
  protocols: {
    ...defaultSchema.protocols,
    href: ['http', 'https', 'mailto'],
    src: ['http', 'https'],
    longDesc: ['http', 'https'],
    cite: ['http', 'https'],
  },
};

export function sanitizeHtml(html) {
  const tree = fromHtml(html || '', { fragment: true });
  const cleanTree = sanitize(tree, safeHtmlSchema);
  return toHtml(cleanTree);
}

export function renderSafeMarkdown(markdown) {
  return sanitizeHtml(marked.parse(markdown || ''));
}
