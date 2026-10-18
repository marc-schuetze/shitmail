import { describe, expect, it } from 'vitest'
import { emailText, htmlToReadableText, safeLink } from '@/lib/emailText'

describe('text-only email reader', () => {
  it('prefers plain text and falls back for HTML-only messages', () => {
    expect(emailText('Your code: 123456', '<p>Other content</p>')).toBe('Your code: 123456')
    expect(emailText(' ', '<p>Your code: <b>123456</b></p>')).toBe('Your code: 123456')
  })
  it('preserves paragraphs, lists, tables, entities and link destinations', () => {
    const text = htmlToReadableText('<p>Hello &amp; welcome</p><p>Code<br>123456</p><ul><li>First</li><li>Second</li></ul><table><tr><td>Name</td><td>Value</td></tr></table><a href="https://example.org/verify?token=abc&amp;x=1">Confirm</a>')
    expect(text).toContain('Hello & welcome\n\nCode\n123456')
    expect(text).toContain('• First')
    expect(text).toContain('Name\tValue')
    expect(text).toContain('Confirm <https://example.org/verify?token=abc&x=1>')
  })
  it('discards active content, images, embeds, styles and hidden text', () => {
    const html = `<head><title>hidden title</title><link href="https://tracker.test/css"></head>
      <script>alert('script')</script><style>@import 'https://tracker.test/css';</style>
      <img src="https://tracker.test/pixel" onerror="alert(1)"><iframe src="https://tracker.test">iframe</iframe>
      <svg><text>svg</text></svg><object data="https://tracker.test">object</object>
      <video poster="https://tracker.test">video</video><span hidden>hidden</span>
      <span style="display:none">preheader</span><span aria-hidden="true">hidden</span>
      <p style="background:url(https://tracker.test)" onclick="alert(1)">Read me</p>`
    expect(htmlToReadableText(html)).toBe('Read me')
  })
  it('does not preserve sender link attributes or executable URLs', () => {
    expect(htmlToReadableText('<a href="javascript:alert(1)" ping="https://tracker.test">Click</a>')).toBe('Click')
    for (const url of ['javascript:alert(1)', 'data:text/html,test', '//tracker.test', '/api/v1/proxy', 'https://user:pass@example.org', 'https://exam\nple.org']) expect(safeLink(url)).toBeNull()
  })
  it('treats escaped markup as text without reparsing it', () => {
    expect(htmlToReadableText('<p>&lt;img src="https://tracker.test"&gt;</p>')).toBe('<img src="https://tracker.test">')
  })
})

import { createElement } from 'react'
import { render } from '@testing-library/react'
import { ReadableMessage } from '@/components/EmailDetail'

it('renders only text and deliberate web links, never sender markup', () => {
  const text = htmlToReadableText('<p>Your code: 123456</p><a href="https://example.org/confirm" ping="https://tracker.test/ping" onclick="alert(1)">Confirm</a><img src="https://tracker.test/pixel">')
  const { container } = render(createElement(ReadableMessage, { text }))
  expect(container.textContent).toContain('Your code: 123456')
  expect(container.querySelector('img, iframe, style, script, [src], [ping], [onclick]')).toBeNull()
  const link = container.querySelector('a')!
  expect(link.href).toBe('https://example.org/confirm')
  expect(link.rel).toBe('noopener noreferrer')
  expect(link.getAttribute('referrerpolicy')).toBe('no-referrer')
})
