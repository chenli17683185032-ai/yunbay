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
import assert from 'node:assert/strict'
import test from 'node:test'
import {
  sanitizeHtml,
  sanitizeMarkdownHref,
  sanitizeMarkdownUrl,
} from './sanitize-html'

test('sanitizeHtml removes scriptable attributes and dangerous protocols', () => {
  const dirty = [
    '<p>Hello</p>',
    '<img src=x onerror=alert(1)>',
    '<a href="javascript:alert(1)">bad</a>',
    '<svg onload=alert(1)>x</svg>',
    '<iframe src="https://evil.example"></iframe>',
  ].join('')

  const clean = sanitizeHtml(dirty)

  assert.match(clean, /<p>Hello<\/p>/)
  assert.doesNotMatch(clean, /onerror/i)
  assert.doesNotMatch(clean, /javascript:/i)
  assert.doesNotMatch(clean, /onload/i)
  assert.doesNotMatch(clean, /<svg/i)
  assert.doesNotMatch(clean, /<iframe/i)
})

test('sanitizeHtml keeps ordinary formatting and safe links', () => {
  const clean = sanitizeHtml(
    '<p><strong>Bold</strong> <em>text</em> <a href="https://example.com/path?q=1">link</a></p><ul><li>item</li></ul>'
  )

  assert.match(clean, /<strong>Bold<\/strong>/)
  assert.match(clean, /<em>text<\/em>/)
  assert.match(clean, /href="https:\/\/example\.com\/path\?q=1"/)
  assert.match(clean, /<ul><li>item<\/li><\/ul>/)
})

test('sanitizeMarkdownUrl blocks scriptable markdown links and images', () => {
  assert.equal(sanitizeMarkdownHref('javascript:alert(1)'), undefined)
  assert.equal(
    sanitizeMarkdownHref('https://example.com'),
    'https://example.com'
  )
  assert.equal(
    sanitizeMarkdownHref('/docs/getting-started'),
    '/docs/getting-started'
  )
  assert.equal(sanitizeMarkdownUrl('data:text/html,<svg onload=alert(1)>'), '')
  assert.equal(
    sanitizeMarkdownUrl('https://example.com/logo.png'),
    'https://example.com/logo.png'
  )
})
