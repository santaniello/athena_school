import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// isHttpUrl is a typing convenience only (enable/disable the Add Link
// button) — the backend's CanonicalURL is the authority on what is valid.
export function isHttpUrl(value: string): boolean {
  return /^https?:\/\/[^\s/?#]+\S*$/i.test(value.trim())
}
