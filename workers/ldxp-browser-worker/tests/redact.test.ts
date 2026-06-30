import test from 'node:test'
import assert from 'node:assert/strict'
import { redactValue } from '../src/redact.js'

test('redactValue masks card keys and qr data urls', () => {
  assert.equal(redactValue(''), '')
  assert.equal(redactValue('abcd1234efgh5678'), 'abcd...5678')
  assert.equal(redactValue('data:image/png;base64,AAAA'), 'data:image/png;base64,[redacted]')
})

test('redactValue masks short non-empty strings', () => {
  assert.equal(redactValue('abc'), '[redacted]')
  assert.equal(redactValue('1234567'), '[redacted]')
})

test('redactValue preserves boundary length strings with first and last four characters', () => {
  assert.equal(redactValue('12345678'), '1234...5678')
})
