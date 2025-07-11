import cx from 'classnames'
import type React from 'react'
import { useCallback, useEffect, useRef } from 'react'
import { useLocalStorage, useWindowSize } from 'usehooks-ts'

import { useScrollToBottom } from '@/hooks/use-scroll-to-bottom'

import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { ArrowUpIcon, StopCircle } from 'lucide-react'

interface PureMultimodalInputProps {
	value: string
	onChange: (v: string) => void
	onSend: () => void
	disabled?: boolean
	placeholder?: string
	className?: string
	isLoading?: boolean
	handleStop?: () => void
}

export function MultimodalInput({ value, onChange, onSend, disabled, placeholder, className, isLoading, handleStop }: PureMultimodalInputProps) {
	const textareaRef = useRef<HTMLTextAreaElement>(null)
	const { width } = useWindowSize()

	useEffect(() => {
		if (textareaRef.current) {
			adjustHeight()
		}
	}, [])

	const adjustHeight = () => {
		if (textareaRef.current) {
			textareaRef.current.style.height = 'auto'
			textareaRef.current.style.height = `${textareaRef.current.scrollHeight + 2}px`
		}
	}

	const resetHeight = () => {
		if (textareaRef.current) {
			textareaRef.current.style.height = 'auto'
			textareaRef.current.style.height = '98px'
		}
	}

	const [localStorageInput, setLocalStorageInput] = useLocalStorage('input', '')

	useEffect(() => {
		if (textareaRef.current) {
			const domValue = textareaRef.current.value
			// Prefer DOM value over localStorage to handle hydration
			const finalValue = domValue || localStorageInput || ''
			onChange(finalValue)
			adjustHeight()
		}
		// Only run once after hydration
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [])

	useEffect(() => {
		setLocalStorageInput(value)
	}, [value, setLocalStorageInput])

	const handleInput = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
		onChange(event.target.value)
		adjustHeight()
	}

	const submitForm = useCallback(() => {
		onSend()
		setLocalStorageInput('')
		resetHeight()

		if (width && width > 768) {
			textareaRef.current?.focus()
		}
	}, [onSend, setLocalStorageInput, resetHeight, width])

	return (
		<div className="relative w-full flex flex-col gap-4">
			<Textarea
				data-testid="multimodal-input"
				ref={textareaRef}
				placeholder="Send a message..."
				value={value}
				onChange={handleInput}
				className={cx('min-h-[24px] max-h-[calc(75dvh)] overflow-hidden resize-none rounded-2xl !text-base bg-muted pb-10 dark:border-zinc-700', className)}
				rows={2}
				autoFocus
				onKeyDown={(event) => {
					if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
						event.preventDefault()

						submitForm()
					}
				}}
			/>

			<div className="absolute bottom-0 right-0 p-2 w-fit flex flex-row justify-end">{isLoading ? <StopButton stop={handleStop} /> : <SendButton input={value} submitForm={submitForm} />}</div>
		</div>
	)
}

export function StopButton({ stop }: { stop: () => void }) {
	return (
		<Button
			data-testid="stop-button"
			className="rounded-full p-1.5 h-fit border dark:border-zinc-600"
			onClick={(event) => {
				event.preventDefault()
				stop()
			}}
		>
			<StopCircle size={14} />
		</Button>
	)
}

export function SendButton({ submitForm, input }: { submitForm: () => void; input: string }) {
	return (
		<Button
			data-testid="send-button"
			className="rounded-full p-1.5 h-fit border dark:border-zinc-600"
			onClick={(event) => {
				event.preventDefault()
				submitForm()
			}}
			disabled={input.length === 0}
		>
			<ArrowUpIcon size={14} />
		</Button>
	)
}
