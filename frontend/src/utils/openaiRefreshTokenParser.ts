const refreshKeyNames = new Set(['refreshtoken', 'refresh_token', 'rt'])

const refreshRegexes = [
  /["'`]?refreshToken["'`]?\s*[:=]\s*["'`]([^"'`\s,}]+)["'`]?/gi,
  /["'`]?refresh_token["'`]?\s*[:=]\s*["'`]([^"'`\s,}]+)["'`]?/gi,
  /["'`]?rt["'`]?\s*[:=]\s*["'`]([^"'`\s,}]+)["'`]?/gi
]

function sanitizeToken(raw: string): string {
  return raw.trim().replace(/^["'`]+|["'`,;]+$/g, '')
}

function addUnique(list: string[], seen: Set<string>, rawValue: string): void {
  const token = sanitizeToken(rawValue)
  if (!token || seen.has(token)) {
    return
  }
  seen.add(token)
  list.push(token)
}

function collectFromObject(value: unknown, refreshTokens: string[], seen: Set<string>): void {
  if (Array.isArray(value)) {
    for (const item of value) {
      collectFromObject(item, refreshTokens, seen)
    }
    return
  }

  if (!value || typeof value !== 'object') {
    return
  }

  for (const [key, fieldValue] of Object.entries(value as Record<string, unknown>)) {
    if (typeof fieldValue === 'string') {
      const normalizedKey = key.toLowerCase()
      if (refreshKeyNames.has(normalizedKey)) {
        addUnique(refreshTokens, seen, fieldValue)
      }
      continue
    }

    collectFromObject(fieldValue, refreshTokens, seen)
  }
}

function collectFromJSONString(raw: string, refreshTokens: string[], seen: Set<string>): void {
  const trimmed = raw.trim()
  if (!trimmed) {
    return
  }

  const candidates = [trimmed]
  const firstObjectBrace = trimmed.indexOf('{')
  const lastObjectBrace = trimmed.lastIndexOf('}')
  if (firstObjectBrace >= 0 && lastObjectBrace > firstObjectBrace) {
    candidates.push(trimmed.slice(firstObjectBrace, lastObjectBrace + 1))
  }

  const firstArrayBracket = trimmed.indexOf('[')
  const lastArrayBracket = trimmed.lastIndexOf(']')
  if (firstArrayBracket >= 0 && lastArrayBracket > firstArrayBracket) {
    candidates.push(trimmed.slice(firstArrayBracket, lastArrayBracket + 1))
  }

  for (const candidate of candidates) {
    try {
      const parsed = JSON.parse(candidate)
      collectFromObject(parsed, refreshTokens, seen)
      if (refreshTokens.length > 0) {
        return
      }
    } catch {
      // ignore invalid JSON candidate and keep trying other extraction strategies
    }
  }
}

function collectByRegex(raw: string, refreshTokens: string[], seen: Set<string>): void {
  const matches: Array<{ index: number; value: string }> = []

  for (const regex of refreshRegexes) {
    regex.lastIndex = 0
    let match: RegExpExecArray | null = regex.exec(raw)
    while (match) {
      if (match[1]) {
        matches.push({ index: match.index, value: match[1] })
      }
      match = regex.exec(raw)
    }
  }

  matches
    .sort((left, right) => left.index - right.index)
    .forEach((match) => addUnique(refreshTokens, seen, match.value))
}

function collectPlainLines(raw: string, refreshTokens: string[], seen: Set<string>): void {
  for (const line of raw.split(/\r?\n/)) {
    const token = sanitizeToken(line)
    if (!token) {
      continue
    }

    if (/^rt[-_]/i.test(token)) {
      addUnique(refreshTokens, seen, token)
    }
  }
}

export function parseOpenAIRawRefreshTokens(raw: string): string[] {
  const refreshTokens: string[] = []
  const seen = new Set<string>()

  collectFromJSONString(raw, refreshTokens, seen)
  collectByRegex(raw, refreshTokens, seen)
  collectPlainLines(raw, refreshTokens, seen)

  return refreshTokens
}
