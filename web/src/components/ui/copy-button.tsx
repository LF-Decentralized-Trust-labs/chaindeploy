import { Check, Copy } from 'lucide-react'
import { useState } from 'react'
import { Button, ButtonProps } from './button'
import { cn } from '@/lib/utils'

interface CopyButtonProps extends Omit<ButtonProps, 'onClick'> {
	/**
	 * The text content to copy to clipboard
	 */
	textToCopy: string
	/**
	 * Optional custom label for the button (defaults to "Copy")
	 */
	label?: string
	/**
	 * Optional custom label when copied (if not provided, only shows checkmark)
	 */
	copiedLabel?: string
	/**
	 * Whether to show the icon (defaults to true)
	 */
	showIcon?: boolean
	/**
	 * Duration in milliseconds to show the success state (defaults to 3000)
	 */
	successDuration?: number
	/**
	 * Callback function after successful copy
	 */
	onCopy?: () => void
	/**
	 * Additional class names for the icon
	 */
	iconClassName?: string
}

/**
 * A copy button component that shows visual feedback when content is copied.
 * Displays a green checkmark for 3 seconds after copying.
 *
 * @example
 * ```tsx
 * <CopyButton
 *   textToCopy="Hello World"
 *   label="Copy to clipboard"
 * />
 *
 * // Icon-only variant
 * <CopyButton
 *   textToCopy={yamlContent}
 *   variant="ghost"
 *   size="icon"
 *   label=""
 *   className="h-8 w-8"
 * />
 * ```
 */
export function CopyButton({
	textToCopy,
	label = 'Copy',
	copiedLabel,
	showIcon = true,
	successDuration = 3000,
	onCopy,
	iconClassName,
	className,
	...props
}: CopyButtonProps) {
	const [isCopied, setIsCopied] = useState(false)

	const handleCopy = async () => {
		try {
			await navigator.clipboard.writeText(textToCopy)
			setIsCopied(true)
			onCopy?.()

			setTimeout(() => {
				setIsCopied(false)
			}, successDuration)
		} catch (error) {
			console.error('Failed to copy text:', error)
		}
	}

	const isIconOnly = !label && !copiedLabel

	return (
		<Button
			onClick={handleCopy}
			className={className}
			{...props}
		>
			{showIcon && (
				isCopied ? (
					<Check className={cn(
						"h-4 w-4 text-green-500",
						!isIconOnly && label && "mr-2",
						iconClassName
					)} />
				) : (
					<Copy className={cn(
						"h-4 w-4",
						!isIconOnly && label && "mr-2",
						iconClassName
					)} />
				)
			)}
			{!isIconOnly && label && !isCopied && label}
			{!isIconOnly && copiedLabel && isCopied && copiedLabel}
		</Button>
	)
}
