// Parse in an inert template: its contents are never attached to the document.
// Only extracted strings leave this function; no sender elements or attributes
// are rendered, including CSS, event handlers and resource-loading attributes.
export function htmlToReadableText(html: string): string {
  const template = document.createElement('template')
  template.innerHTML = html
  const discarded = new Set(['SCRIPT', 'STYLE', 'HEAD', 'TITLE', 'META', 'LINK', 'BASE',
    'IMG', 'PICTURE', 'SOURCE', 'IFRAME', 'FRAME', 'FRAMESET', 'OBJECT', 'EMBED',
    'SVG', 'MATH', 'VIDEO', 'AUDIO', 'CANVAS', 'TEMPLATE', 'NOSCRIPT', 'INPUT', 'BUTTON', 'SELECT', 'TEXTAREA'])
  const blocks = new Set(['P', 'DIV', 'SECTION', 'ARTICLE', 'HEADER', 'FOOTER', 'H1', 'H2', 'H3',
    'H4', 'H5', 'H6', 'UL', 'OL', 'BLOCKQUOTE', 'TABLE', 'PRE'])
  const walk = (node: Node, pre = false): string => {
    if (node.nodeType === Node.TEXT_NODE) return pre ? node.textContent ?? '' : (node.textContent ?? '').replace(/\s+/g, ' ')
    if (!(node instanceof Element)) return ''
    const tag = node.tagName.toUpperCase()
    if (discarded.has(tag) || node.hasAttribute('hidden') || node.getAttribute('aria-hidden') === 'true') return ''
    const style = (node as HTMLElement).style
    if (style?.display === 'none' || style?.visibility === 'hidden' || style?.opacity === '0') return ''
    if (tag === 'BR') return '\n'
    if (tag === 'HR') return '\n\n'
    let text = Array.from(node.childNodes, child => walk(child, pre || tag === 'PRE')).join('')
    if (tag === 'A') {
      const href = safeLink(node.getAttribute('href') ?? '')
      if (href && text.trim() !== href) text = `${text.trim()} <${href}>`
    }
    if (tag === 'LI') return `\n• ${text.trim()}\n`
    if (tag === 'TD' || tag === 'TH') return `${text.trim()}\t`
    if (tag === 'TR') return `\n${text.trim()}\n`
    return blocks.has(tag) ? `\n\n${text.trim()}\n\n` : text
  }
  return Array.from(template.content.childNodes, node => walk(node)).join('')
    .replace(/[ \t]+\n/g, '\n').replace(/\n[ \t]+/g, '\n').replace(/\n{3,}/g, '\n\n').trim()
}

export function safeLink(value: string): string | null {
  // Require an explicit web URL, never relative URLs or executable schemes.
  if (!/^https?:\/\//i.test(value) || /[\u0000-\u0020\u007f]/.test(value)) return null
  try {
    const url = new URL(value)
    return url.username || url.password ? null : url.href
  } catch { return null }
}

export function emailText(bodyText: string, bodyHtml: string): string {
  return bodyText?.trim() ? bodyText : htmlToReadableText(bodyHtml || '')
}
