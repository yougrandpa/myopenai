import { describe, expect, it } from 'vitest'
import { parseOpenAIRawRefreshTokens } from '@/utils/openaiRefreshTokenParser'

describe('parseOpenAIRawRefreshTokens', () => {
  it('parses refresh_token from a JSON object export', () => {
    const payload = JSON.stringify({
      id_token: 'id-1',
      access_token: 'at-1',
      refresh_token: 'rt-json-1',
      email: 'tester@example.com'
    })

    expect(parseOpenAIRawRefreshTokens(payload)).toEqual(['rt-json-1'])
  })

  it('parses refresh_token from nested objects and arrays', () => {
    const payload = JSON.stringify({
      items: [
        { label: 'first', credentials: { refresh_token: 'rt-nested-1' } },
        { label: 'second', credentials: { refreshToken: 'rt-nested-2' } }
      ]
    })

    expect(parseOpenAIRawRefreshTokens(payload)).toEqual(['rt-nested-1', 'rt-nested-2'])
  })

  it('supports plain refresh tokens one per line', () => {
    expect(parseOpenAIRawRefreshTokens('rt-1\nrt-2')).toEqual(['rt-1', 'rt-2'])
  })

  it('supports regex fallback for non-standard snippets', () => {
    const raw = "refresh_token: 'rt-snippet-1'\nrefreshToken = \"rt-snippet-2\""

    expect(parseOpenAIRawRefreshTokens(raw)).toEqual(['rt-snippet-1', 'rt-snippet-2'])
  })

  it('keeps tokens unique when duplicates appear in mixed inputs', () => {
    const raw = [
      'rt-dup',
      JSON.stringify({ refresh_token: 'rt-dup' }),
      '{"refresh_token":"rt-second"}'
    ].join('\n')

    expect(parseOpenAIRawRefreshTokens(raw)).toEqual(['rt-dup', 'rt-second'])
  })
})
