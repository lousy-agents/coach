export function parseFrontmatter(text: string): { data: Record<string, string>; body: string } | null {
  const normalized = text.replace(/^\uFEFF/, "")
  if (!normalized.startsWith("---\n") && !normalized.startsWith("---\r\n")) return null

  const afterOpen = normalized.startsWith("---\r\n") ? 5 : 4
  const rest = normalized.slice(afterOpen)
  const endMatch = rest.match(/\r?\n---\r?\n/)
  if (!endMatch || endMatch.index === undefined) return null

  const fm = rest.slice(0, endMatch.index)
  const body = rest.slice(endMatch.index + endMatch[0].length)
  return { data: parseFrontmatterBlock(fm), body }
}

class FrontmatterBlock {
  readonly data: Record<string, string> = {}
  private key: string | null = null
  private buf: string[] = []

  addLine(line: string): void {
    if (this.key !== null && (/^\s/.test(line) || line.startsWith("- "))) {
      this.buf.push(line)
      return
    }
    this.flush()
    const m = line.match(/^([A-Za-z0-9_-]+):\s*(.*)$/)
    if (!m) return
    const [, k, raw] = m
    if (raw === "" || raw === "|" || raw === ">") {
      this.key = k
      this.buf = []
      return
    }
    this.data[k] = raw.replace(/^["']|["']$/g, "")
  }

  finish(): Record<string, string> {
    this.flush()
    return this.data
  }

  private flush(): void {
    if (this.key === null) return
    this.data[this.key] = this.buf.join("\n").trim()
    this.key = null
    this.buf = []
  }
}

export function parseFrontmatterBlock(fm: string): Record<string, string> {
  const block = new FrontmatterBlock()
  for (const line of fm.split(/\r?\n/)) block.addLine(line)
  return block.finish()
}

export function parseTools(raw: string | undefined): string[] {
  if (!raw) return []
  return raw
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean)
}
