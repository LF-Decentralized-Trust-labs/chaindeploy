import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'
import { X509Certificate } from '@peculiar/x509'

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs))
}

export function isValidPEMCertificate(value: string): boolean {
	if (!value) return false
	try {
		const pemRegex = /-----BEGIN CERTIFICATE-----([\s\S]+?)-----END CERTIFICATE-----/g
		if (!pemRegex.test(value)) return false
		new X509Certificate(value)
		return true
	} catch {
		return false
	}
}

