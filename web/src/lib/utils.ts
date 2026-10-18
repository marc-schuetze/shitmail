import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** Merge Tailwind classes safely. */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}

/** Format a date as a human-readable relative string. */
export function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = Date.now()
  const diff = now - date.getTime()

  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days}d ago`
  return date.toLocaleDateString()
}

/** Format a future date as a precise countdown string. */
/** Splits "slack1-5i0ra" into the user-chosen name and the per-user tag. */
export function splitLocalPart(localPart: string): { name: string; tag: string } {
  const i = localPart.lastIndexOf('-')
  return i > 0 ? { name: localPart.slice(0, i), tag: localPart.slice(i + 1) } : { name: localPart, tag: '' }
}

/** The user-facing mailbox name without the tag suffix. */
export function mailboxName(localPart: string): string {
  return splitLocalPart(localPart).name
}

/** Mailboxes kept "forever" carry an expiry decades out. */
export function isForever(expiresAt: string): boolean {
  return new Date(expiresAt).getTime() - Date.now() > 50 * 365 * 24 * 3600 * 1000
}

export function formatCountdown(expiresAt: string): string {
  const diff = new Date(expiresAt).getTime() - Date.now()
  if (diff <= 0) return 'expired'
  if (isForever(expiresAt)) return 'forever'
  const totalSecs = Math.floor(diff / 1000)
  const days = Math.floor(totalSecs / 86400)
  if (days >= 2) return `${days}d ${Math.floor((totalSecs % 86400) / 3600)}h`
  const hours = Math.floor(totalSecs / 3600)
  const minutes = Math.floor((totalSecs % 3600) / 60)
  const seconds = totalSecs % 60
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${seconds}s`
  return `${seconds}s`
}

/** Format bytes to a readable string. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

/** Return the first letter of an email address sender for avatars. */
export function senderInitial(from: string): string {
  const match = from.match(/^([^<@\s])/i)
  return match ? match[1].toUpperCase() : '?'
}

/** Strip display name from "Name <email>" format. */
export function extractEmail(from: string): string {
  const match = from.match(/<([^>]+)>/)
  return match ? match[1] : from
}

/** Strip display name — return just the display portion. */
export function extractDisplayName(from: string): string {
  const match = from.match(/^([^<]+)</)
  return match ? match[1].trim() : from
}
