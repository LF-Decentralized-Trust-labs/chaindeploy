export function slugify(text: string): string {
	return text
		.toString()
		.toLowerCase()
		.replace(/\s+/g, '-')
		.replace(/[^\w\-]+/g, '')
		.replace(/\-\-+/g, '-')
		.replace(/^-+/, '')
		.replace(/-+$/, '')
}

export function numberToHex(num: number): string {
	return `0x${num.toString(16)}`
}

export function numberToNonceHex(num: number): string {
	return `0x${num.toString(16).padStart(16, '0')}`
}

export function hexToNumber(hex: string): number {
	return parseInt(hex.replace('0x', ''), 16)
}

export function isValidHex(hex: string): boolean {
	return /^0x[0-9a-fA-F]+$/.test(hex)
}
