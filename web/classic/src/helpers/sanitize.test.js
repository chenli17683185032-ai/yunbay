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
import assert from 'node:assert/strict';
import test from 'node:test';
import { renderSafeMarkdown, sanitizeHtml, sanitizeUrl } from './sanitize';

test('sanitizeHtml removes scriptable HTML from admin-provided content', () => {
  const clean = sanitizeHtml(
    '<p>Hello</p><img src=x onerror=alert(1)><a href="javascript:alert(1)">bad</a><svg onload=alert(1)>x</svg><iframe src="https://evil.example"></iframe>',
  );

  assert.match(clean, /<p>Hello<\/p>/);
  assert.doesNotMatch(clean, /onerror/i);
  assert.doesNotMatch(clean, /javascript:/i);
  assert.doesNotMatch(clean, /onload/i);
  assert.doesNotMatch(clean, /<svg/i);
  assert.doesNotMatch(clean, /<iframe/i);
});

test('renderSafeMarkdown sanitizes marked output before rendering', () => {
  const clean = renderSafeMarkdown(
    '[bad](javascript:alert(1))\n\n<img src=x onerror=alert(1)>\n\n**safe**',
  );

  assert.match(clean, /<strong>safe<\/strong>/);
  assert.doesNotMatch(clean, /javascript:/i);
  assert.doesNotMatch(clean, /onerror/i);
});


test('sanitizeUrl blocks scriptable URL protocols', () => {
  assert.equal(sanitizeUrl('javascript:alert(1)'), undefined);
  assert.equal(sanitizeUrl('data:text/html,<svg onload=alert(1)>'), undefined);
  assert.equal(sanitizeUrl('https://example.com'), 'https://example.com');
  assert.equal(sanitizeUrl('/console'), '/console');
});
