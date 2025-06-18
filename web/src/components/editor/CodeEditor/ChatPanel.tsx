import {
	getAiByProjectIdConversations,
	getAiByProjectIdConversationsByConversationId,
	getChaincodeProjectsByIdCommits,
	getChaincodeProjectsByIdCommitsByCommitHash,
	getChaincodeProjectsByIdFileAtCommit,
	ProjectsCommitWithFileChangesApi,
} from '@/api/client'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Check, Code, Copy, GitCommit, History } from 'lucide-react'
import * as monaco from 'monaco-editor'
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Components } from 'react-markdown'
import ReactMarkdown from 'react-markdown'
import type { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { jsonrepair } from 'jsonrepair'
import { toast } from 'sonner'
import { getMonacoLanguage } from './types'
const SyntaxHighlighterComp = SyntaxHighlighter as unknown as React.ComponentType<SyntaxHighlighterProps>

function useAutoResizeTextarea() {
	const textareaRef = useRef<HTMLTextAreaElement>(null)

	const adjustHeight = useCallback(() => {
		const textarea = textareaRef.current
		if (textarea) {
			textarea.style.height = 'auto'
			textarea.style.height = `${textarea.scrollHeight}px`
		}
	}, [])

	useEffect(() => {
		adjustHeight()
	}, [adjustHeight])

	return { textareaRef, adjustHeight }
}

interface MessagePart {
	type: 'text' | 'tool'
	content?: string
	toolEvent?: ToolEvent
}

interface Message {
	role: 'user' | 'assistant'
	parts: MessagePart[]
}

interface ToolEvent {
	type: 'start' | 'update' | 'execute' | 'result'
	toolCallID: string
	name: string
	arguments?: string
	args?: Record<string, unknown>
	result?: unknown
	error?: string
}

interface ToolExecution {
	toolCallID: string
	name: string
	status: 'started' | 'updating' | 'executing' | 'completed' | 'error'
	error?: string
}

export interface UseStreamingChatResult {
	messages: Message[]
	input: string
	setInput: React.Dispatch<React.SetStateAction<string>>
	isLoading: boolean
	activeTool: ToolExecution | null
	handleSubmit: (e: React.FormEvent<HTMLFormElement>) => void
	partialArgsRef: React.MutableRefObject<string>
	setMessages: React.Dispatch<React.SetStateAction<Message[]>>
	onToolResult?: (toolName: string, result: unknown) => void
	onComplete?: () => void
}

export function useStreamingChat(projectId: number, onToolResult?: (toolName: string, result: unknown) => void, onComplete?: () => void): UseStreamingChatResult {
	const [messages, setMessages] = useState<Message[]>([])
	const [input, setInput] = useState('')
	const [isLoading, setIsLoading] = useState(false)
	const [activeTool, setActiveTool] = useState<ToolExecution | null>(null)
	const abortRef = useRef<AbortController | null>(null)
	const partialArgsRef = useRef<string>('')

	const handleSubmit = useCallback(
		async (e: React.FormEvent<HTMLFormElement>) => {
			e.preventDefault()
			setIsLoading(true)
			setActiveTool(null)
			partialArgsRef.current = ''
			try {
				setMessages((prev) => [...prev, { role: 'user', parts: [{ type: 'text', content: input }] }])
				setInput('')
				const controller = new AbortController()
				abortRef.current = controller
				const res = await fetch(`/api/v1/ai/${projectId}/chat`, {
					method: 'POST',
					body: JSON.stringify({
						projectId: projectId.toString(),
						messages: [
							{
								role: 'user',
								content: input,
							},
						],
					}),
					headers: {
						'Content-Type': 'application/json',
					},
					signal: controller.signal,
				})
				if (!res.body) throw new Error('No response body')
				const reader = res.body.getReader()
				let buffer = ''
				let done = false
				let assistantContent = ''

				while (!done) {
					const { value, done: doneReading } = await reader.read()
					done = doneReading
					if (value) {
						buffer += new TextDecoder().decode(value)
						let lineEnd
						while ((lineEnd = buffer.indexOf('\n')) !== -1) {
							const line = buffer.slice(0, lineEnd).trim()
							buffer = buffer.slice(lineEnd + 1)
							if (line.startsWith('data:')) {
								const dataStr = line.slice(5).trim()
								if (dataStr) {
									try {
										const event = JSON.parse(dataStr)
										switch (event.type) {
											case 'llm': {
												if (typeof event.content === 'string') {
													assistantContent += event.content
													setMessages((prev) => {
														const lastMsgIdx = prev.length - 1
														if (lastMsgIdx < 0 || prev[lastMsgIdx].role !== 'assistant') {
															return [...prev, { role: 'assistant', parts: [{ type: 'text', content: assistantContent }] }]
														}
														const lastMsg = prev[lastMsgIdx]
														const updatedParts = [...lastMsg.parts]
														// If last part is text, update it; else append
														if (updatedParts.length && updatedParts[updatedParts.length - 1].type === 'text') {
															updatedParts[updatedParts.length - 1] = { type: 'text', content: assistantContent }
														} else {
															updatedParts.push({ type: 'text', content: assistantContent })
														}
														return [...prev.slice(0, lastMsgIdx), { ...lastMsg, parts: updatedParts }]
													})
												}
												break
											}
											case 'tool_result':
											case 'tool_start':
											case 'tool_update':
											case 'tool_execute': {
												const toolEvent = {
													...event,
												}
												const mappingBetweenToolEvents = {
													tool_result: 'result',
													tool_start: 'start',
													tool_update: 'update',
													tool_execute: 'execute',
												}
												setMessages((prev) => {
													const lastMsgIdx = prev.length - 1
													if (lastMsgIdx < 0 || prev[lastMsgIdx].role !== 'assistant') {
														const updated = [...prev, { role: 'assistant', parts: [{ type: 'tool', toolEvent }] }] as Message[]
														return updated
													}
													const lastMsg = prev[lastMsgIdx]
													const updatedParts = [...lastMsg.parts]
													const toolPartIdx = updatedParts.findIndex((part) => part.type === 'tool' && part.toolEvent?.toolCallID === toolEvent.toolCallID)
													if (toolPartIdx !== -1) {
														const existingToolEvent = updatedParts[toolPartIdx].toolEvent
														let mergedArguments = existingToolEvent?.arguments || '{}'

														// For tool_update, merge the delta with existing arguments
														if (event.type === 'tool_update' && event.arguments) {
															console.log('=== TOOL_UPDATE DEBUG ===')
															console.log('Event type:', event.type)
															console.log('Tool name:', event.name)
															console.log('New delta arguments (raw):', event.arguments)
															console.log('Existing merged arguments (raw):', mergedArguments)
															
															// Always accumulate raw strings - this is the source of truth
															let accumulatedRaw = ''
															if (mergedArguments === '{}' || mergedArguments === '') {
																// If starting fresh, just use the new delta
																accumulatedRaw = event.arguments
																console.log('Starting fresh with delta:', accumulatedRaw)
															} else {
																// Always accumulate as raw strings, regardless of whether existing is valid JSON
																accumulatedRaw = mergedArguments + event.arguments
																console.log('Accumulated as raw strings:', accumulatedRaw)
															}
															
															// Try to parse the accumulated raw string for display/processing
															try {
																const repaired = jsonrepair(accumulatedRaw)
																console.log('Repaired accumulated string:', repaired)
																const parsed = JSON.parse(repaired)
																console.log('Parsed accumulated object:', parsed)
																
																// Store the raw accumulated string, not the parsed result
																mergedArguments = accumulatedRaw
																console.log('Final merged arguments (raw accumulated):', mergedArguments)
															} catch (error) {
																console.log('Error parsing accumulated string:', error)
																// If parsing fails, still store the raw accumulated string
																mergedArguments = accumulatedRaw
																console.log('Fallback: storing raw accumulated string:', mergedArguments)
															}
															console.log('=== END TOOL_UPDATE DEBUG ===')
														} else if (event.type === 'tool_execute') {
															mergedArguments = JSON.stringify(event.args)
														}

														const updatedToolEvent: any = {
															...existingToolEvent,
															type: mappingBetweenToolEvents[event.type],
															toolCallID: event.toolCallID,
															name: event.name,
															arguments: mergedArguments,
														}

														updatedParts[toolPartIdx] = {
															type: 'tool',
															toolEvent: updatedToolEvent,
														} as MessagePart
													} else {
														updatedParts.push({
															type: 'tool',
															toolEvent: {
																type: mappingBetweenToolEvents[event.type],
																toolCallID: toolEvent.toolCallID,
																name: toolEvent.name,
																arguments: event.type === 'tool_update' ? event.arguments : undefined,
															},
														} as MessagePart)
													}
													const updated = [...prev.slice(0, lastMsgIdx), { ...lastMsg, parts: updatedParts }]
													return updated
												})
												if (mappingBetweenToolEvents[event.type] === 'result') {
													partialArgsRef.current = ''
													setActiveTool(null)
													if (onToolResult && !event.error) {
														onToolResult(event.name, event.result)
													}
												}
												break
											}
											case 'max_steps_reached':
												setActiveTool(null)
												partialArgsRef.current = ''
												break
										}
									} catch {
										// ignore malformed JSON
									}
								}
							}
						}
					}
				}
				setIsLoading(false)
				setActiveTool(null)
				partialArgsRef.current = ''
				if (onComplete) {
					onComplete()
				}
			} catch (error) {
				if (!(error instanceof DOMException && error.name === 'AbortError')) {
					console.error('Error sending message:', error)
					toast.error('Failed to send message')
				}
				setIsLoading(false)
				setActiveTool(null)
				partialArgsRef.current = ''
			}
		},
		[input, projectId, onToolResult, onComplete]
	)

	return { messages, input, setInput, isLoading, activeTool, handleSubmit, partialArgsRef, setMessages }
}

export function ChatPanel({ projectId = 1, chatState }: { projectId: number; chatState: UseStreamingChatResult }) {
	const [partialArgs, setPartialArgs] = useState<Record<string, unknown> | null>(null)
	const [firstConversationId, setFirstConversationId] = useState<string | null>(null)
	const [historyDialogOpen, setHistoryDialogOpen] = useState(false)
	const messagesEndRef = useRef<HTMLDivElement>(null)
	const { textareaRef, adjustHeight } = useAutoResizeTextarea()
	const { data: conversations } = useQuery({
		queryKey: ['conversations', projectId],
		queryFn: () => getAiByProjectIdConversations({ path: { projectId } }),
	})
	const { data: conversationDetails } = useQuery({
		queryKey: ['conversation-details', conversations?.data?.map((c) => c.id)],
		queryFn: async () => {
			if (!conversations?.data) return []
			const details = await Promise.all(
				conversations.data.map((conv) =>
					getAiByProjectIdConversationsByConversationId({
						path: {
							projectId,
							conversationId: conv.id!,
						},
					})
				)
			)
			return details.map((d) => d.data)
		},
		enabled: !!conversations?.data,
	})
	const { data: conversationMessages } = useQuery({
		queryKey: ['conversation', firstConversationId],
		queryFn: () =>
			getAiByProjectIdConversationsByConversationId({
				path: {
					projectId,
					conversationId: parseInt(firstConversationId!, 10),
				},
			}),
		enabled: !!firstConversationId,
	})
	const { data: commits } = useQuery({
		queryKey: ['commits', projectId],
		queryFn: () => getChaincodeProjectsByIdCommits({ path: { id: projectId } }),
	})
	const { messages, input, setInput, isLoading, activeTool, handleSubmit, partialArgsRef, setMessages } = chatState
	const scrollToBottom = useCallback(() => {
		messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
	}, [])

	useEffect(() => {
		scrollToBottom()
	}, [messages, scrollToBottom])

	const handleInputChange = useCallback(
		(e: React.ChangeEvent<HTMLTextAreaElement>) => {
			setInput(e.target.value)
			adjustHeight()
		},
		[setInput, adjustHeight]
	)

	const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLTextAreaElement>) => {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault()
			const form = e.currentTarget.form
			if (form) {
				form.requestSubmit()
			}
		}
	}, [])

	const handleFormSubmit = useCallback(
		(e: React.FormEvent<HTMLFormElement>) => {
			e.preventDefault()
			handleSubmit(e)
			// Reset textarea height after submission
			if (textareaRef.current) {
				textareaRef.current.style.height = 'auto'
			}
		},
		[handleSubmit]
	)

	const setFirstConversation = useCallback((id: string) => {
		setFirstConversationId(id)
	}, [])

	const formatMessages = useCallback(
		(data: typeof conversationMessages.data) => {
			if (!data) return
			const formattedMessages = data
				.filter((msg) => msg.content || (msg.toolCalls && msg.toolCalls.length > 0))
				.map((msg) => ({
					role: (msg.sender === 'user' ? 'user' : 'assistant') as 'user' | 'assistant',
					parts: [
						...(msg.sender !== 'tool' && msg.content
							? [
									{
										type: 'text' as const,
										content: msg.content || '',
									},
							  ]
							: []),
						...(msg.toolCalls?.map((tool) => ({
							type: 'tool' as const,
							toolEvent: {
								type: 'result' as 'start' | 'result',
								toolCallID: tool.id?.toString() || '',
								name: tool.toolName || '',
								arguments: tool.arguments,
								result: tool.result && tool.result.valid ? tool.result.string ?? '' : '',
								error: tool.error && tool.error.valid ? tool.error.string ?? '' : '',
							},
						})) || []),
					],
				}))
			setMessages(formattedMessages)
		},
		[setMessages, conversationMessages]
	)

	const parsePartialArgs = useCallback(() => {
		if (!partialArgsRef.current) return

		console.log('parsePartialArgs - raw input:', partialArgsRef.current)

		try {
			// Try direct JSON parse first
			const args = JSON.parse(partialArgsRef.current)
			console.log('parsePartialArgs - direct parse success:', args)
			setPartialArgs(args)
		} catch (error) {
			console.log('parsePartialArgs - direct parse failed:', error)
			try {
				// Try to repair with jsonrepair
				const repaired = jsonrepair(partialArgsRef.current)
				console.log('parsePartialArgs - jsonrepair result:', repaired)
				const args = JSON.parse(repaired)
				console.log('parsePartialArgs - repaired parse success:', args)
				setPartialArgs(args)
			} catch (repairError) {
				console.log('parsePartialArgs - jsonrepair failed:', repairError)
				setPartialArgs({ raw: partialArgsRef.current })
			}
		}
	}, [partialArgsRef])

	// Effect to set first conversation ID
	useEffect(() => {
		if (conversations?.data && conversations.data.length > 0 && !firstConversationId) {
			setFirstConversation(conversations.data[0].id?.toString() || '')
		}
	}, [conversations?.data, firstConversationId, setFirstConversation])

	// Effect to set messages from conversation
	useEffect(() => {
		formatMessages(conversationMessages?.data)
	}, [conversationMessages?.data, formatMessages])

	// Effect to parse partial arguments every 500ms
	useEffect(() => {
		if (!activeTool) {
			setPartialArgs(null)
			return
		}

		const interval = setInterval(parsePartialArgs, 500)
		return () => clearInterval(interval)
	}, [activeTool, parsePartialArgs])

	const messagesContent = useMemo(
		() => (
			<div className="flex-1 overflow-auto p-2 space-y-2">
				{messages.map((msg, i) => (
					<Message key={i} message={msg} />
				))}
				{isLoading && <div className="text-sm text-muted-foreground">{activeTool ? <ActiveTool tool={activeTool} partialArgs={partialArgs} /> : <div>Thinking...</div>}</div>}
				<div ref={messagesEndRef} />
			</div>
		),
		[messages, isLoading, activeTool, partialArgs]
	)

	return (
		<div className="flex flex-col h-full bg-background border-r border-border text-foreground">
			<div className="p-2 border-b border-border font-semibold text-sm flex items-center justify-between">
				<span>Chat</span>
				<Dialog open={historyDialogOpen} onOpenChange={setHistoryDialogOpen}>
					<DialogTrigger asChild>
						<button className="p-1 hover:bg-accent rounded-sm">
							<History className="h-4 w-4" />
						</button>
					</DialogTrigger>
					<DialogContent className="max-w-2xl">
						<DialogHeader>
							<DialogTitle>Chat History</DialogTitle>
						</DialogHeader>
						<div className="grid grid-cols-2 gap-4">
							<div>
								<h3 className="text-sm font-medium mb-2">Conversations</h3>
								<ScrollArea className="h-[60vh]">
									<div className="space-y-4">
										{conversations?.data?.map((conversation, index) => (
											<div
												key={conversation.id}
												className={`p-3 rounded-lg cursor-pointer hover:bg-accent ${conversation.id?.toString() === firstConversationId ? 'bg-accent' : ''}`}
												onClick={() => {
													setFirstConversation(conversation.id?.toString() || '')
													setHistoryDialogOpen(false)
												}}
											>
												<div className="text-sm font-medium">{conversationDetails?.[index]?.[0]?.content?.slice(0, 100) || 'Empty conversation'}</div>
												<div className="text-xs text-muted-foreground mt-1">{new Date(conversation.startedAt || '').toLocaleString()}</div>
											</div>
										))}
									</div>
								</ScrollArea>
							</div>
							<div>
								<h3 className="text-sm font-medium mb-2">Commits</h3>
								<ScrollArea className="h-[60vh]">
									<div className="space-y-4">
										{commits?.data?.commits?.map((commit) => (
											<div key={commit.hash} className="p-3 rounded-lg border border-border hover:bg-accent/50 transition-colors cursor-pointer">
												<CommitDetails projectId={projectId} commit={commit} commitHash={commit.hash || ''} />
											</div>
										))}
									</div>
								</ScrollArea>
							</div>
						</div>
					</DialogContent>
				</Dialog>
			</div>
			{messagesContent}
			<form onSubmit={handleFormSubmit} className="flex p-2 border-t border-border gap-2">
				<textarea
					ref={textareaRef}
					className="flex-1 rounded border px-2 py-1 text-sm bg-background text-foreground resize-none min-h-[36px] max-h-[400px] overflow-y-auto"
					value={input}
					onChange={handleInputChange}
					onKeyDown={handleKeyDown}
					placeholder="Type a message... (Shift + Enter for new line)"
					disabled={isLoading}
					rows={3}
				/>
				<button type="submit" className="px-3 py-1 rounded bg-primary text-primary-foreground text-sm self-end" disabled={isLoading}>
					Send
				</button>
			</form>
		</div>
	)
}

function MarkdownRenderer({ content }: { content: string }) {
	const [copiedCode, setCopiedCode] = useState<string | null>(null)

	const copyToClipboard = async (code: string) => {
		try {
			await navigator.clipboard.writeText(code)
			setCopiedCode(code)
			setTimeout(() => setCopiedCode(null), 2000)
		} catch (err) {
			console.error('Failed to copy code:', err)
		}
	}

	const components: Components = {
		// Headers
		h1: ({ children, ...props }) => (
			<h1 className="text-2xl font-bold mb-4" {...props}>
				{children}
			</h1>
		),
		h2: ({ children, ...props }) => (
			<h2 className="text-xl font-bold mb-3" {...props}>
				{children}
			</h2>
		),
		h3: ({ children, ...props }) => (
			<h3 className="text-lg font-bold mb-2" {...props}>
				{children}
			</h3>
		),

		// Paragraphs and text
		p: ({ children, ...props }) => (
			<p className="mb-4 leading-relaxed" {...props}>
				{children}
			</p>
		),
		strong: ({ children, ...props }) => (
			<strong className="font-semibold" {...props}>
				{children}
			</strong>
		),
		em: ({ children, ...props }) => (
			<em className="italic" {...props}>
				{children}
			</em>
		),

		// Links
		a: ({ children, ...props }) => (
			<a className="text-blue-500 hover:text-blue-600 underline" {...props}>
				{children}
			</a>
		),

		// Code blocks
		code: ({ className, children, ...props }) => {
			const match = /language-(\w+)/.exec(className || '')
			const language = match ? match[1] : 'plaintext'
			const code = String(children).replace(/\n$/, '')
			const isInline = !className

			if (isInline) {
				return (
					<code className="bg-muted px-1.5 py-0.5 rounded text-sm font-mono" {...props}>
						{children}
					</code>
				)
			}

			const highlighterProps: SyntaxHighlighterProps = {
				language,
				style: vscDarkPlus,
				PreTag: 'div',
				className: 'rounded-lg !mt-0 !mb-4',
				showLineNumbers: true,
				wrapLines: true,
				wrapLongLines: true,
				customStyle: {
					margin: 0,
					padding: '1rem',
					background: 'rgb(30, 30, 30)',
				},
				children: code,
			}

			return (
				<div className="relative group">
					<div className="absolute right-2 top-2 opacity-0 group-hover:opacity-100 transition-opacity">
						<button onClick={() => copyToClipboard(code)} className="p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" title="Copy code">
							{copiedCode === code ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
						</button>
					</div>
					<SyntaxHighlighterComp {...highlighterProps} />
				</div>
			)
		},

		// Blockquotes
		blockquote: ({ children, ...props }) => (
			<blockquote className="border-l-4 border-muted pl-4 italic my-4" {...props}>
				{children}
			</blockquote>
		),

		// Horizontal rule
		hr: (props) => <hr className="my-6 border-t border-border" {...props} />,

		// Tables
		table: ({ children, ...props }) => (
			<div className="overflow-x-auto my-4">
				<table className="min-w-full divide-y divide-border" {...props}>
					{children}
				</table>
			</div>
		),
		thead: ({ children, ...props }) => (
			<thead className="bg-muted/50" {...props}>
				{children}
			</thead>
		),
		th: ({ children, ...props }) => (
			<th className="px-4 py-2 text-left font-semibold" {...props}>
				{children}
			</th>
		),
		td: ({ children, ...props }) => (
			<td className="px-4 py-2 border-t border-border" {...props}>
				{children}
			</td>
		),
	}

	return (
		<div className="prose prose-sm max-w-none dark:prose-invert prose-pre:bg-transparent prose-pre:p-0">
			<ReactMarkdown components={components}>{content}</ReactMarkdown>
		</div>
	)
}

interface MessageProps {
	message: Message
}

const Message = React.memo(({ message }: MessageProps) => {
	const messageContent = useMemo(
		() => (
			<div className={`flex flex-col gap-2 ${message.role === 'user' ? 'items-end' : 'items-start'}`}>
				{message.parts.map((part, j) => {
					if (part.type === 'text' && part.content) {
						return (
							<div key={j} className={`rounded-lg p-3 ${message.role === 'user' ? 'bg-background text-foreground border border-border' : 'bg-muted text-foreground'}`}>
								<MarkdownRenderer content={part.content} />
							</div>
						)
					} else if (part.type === 'tool' && part.toolEvent) {
						return (
							<div key={j} className="w-full">
								<ToolEventRenderer event={part.toolEvent} />
							</div>
						)
					}
					return null
				})}
			</div>
		),
		[message]
	)

	return <div className="py-2">{messageContent}</div>
})

interface ToolEventProps {
	event: ToolEvent
}
const ToolSummaryCard = ({ event, summary, children }: { event: ToolEvent; summary: string; children?: React.ReactNode }) => {
	if (event.name === 'read_file' || event.name === 'write_file') {
		return (
			<div className="bg-muted/70 rounded-lg p-4 my-2 shadow border border-border flex flex-col min-h-[140px]">
				<div className="flex items-center gap-2 mb-4">
					<div className="rounded-full p-2 flex items-center justify-center min-w-[44px] min-h-[44px]">
						<span className="font-semibold text-sm leading-tight text-center block">{event.name.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())}</span>
					</div>
				</div>
				<div className="text-sm mb-4">{summary}</div>
				<div className="flex-1">{children}</div>
			</div>
		)
	}
	return (
		<div className="bg-muted/70 rounded-lg p-4 my-2 shadow border border-border flex flex-col min-h-[140px]">
			<div className="flex items-center gap-2 mb-4">
				<div className="rounded-full p-2 flex items-center justify-center min-w-[44px] min-h-[44px]">
					<span className="font-semibold text-sm leading-tight text-center block">{event.name.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())}</span>
				</div>
			</div>
			<div className="text-sm mb-4">{summary}</div>
			<div className="flex-1">{children}</div>
			<div className="flex gap-2 mt-auto pt-2">
				<Button variant="secondary" size="sm" className="flex items-center gap-1">
					<Code className="h-3 w-3" />
					Code
				</Button>
			</div>
		</div>
	)
}

const getToolSummary = (event: ToolEvent) => {
	// Custom summaries for known tools
	if (event.name === 'write_file') {
		let path = ''
		if (typeof event.arguments === 'string') {
			try {
				const args = JSON.parse(event.arguments)
				path = args.path || ''
			} catch {}
		}
		return `The file has been written to "${path}".`
	}
	if (event.name === 'read_file') {
		let path = ''
		if (typeof event.arguments === 'string') {
			try {
				const args = JSON.parse(event.arguments)
				path = args.path || ''
			} catch {}
		}
		return `The file "${path}" has been read successfully.`
	}
	// Default summary
	return `${event.name.replace(/_/g, ' ')} completed successfully.`
}

const ToolEventRenderer = React.memo(({ event }: ToolEventProps) => {
	const handleViewContents = useCallback(() => {}, [])
	const handleViewDetails = useCallback(() => {}, [])
	const [copiedCode, setCopiedCode] = useState<string | null>(null)
	const contentRef = useRef<HTMLDivElement>(null)

	// Add auto-scroll effect
	useEffect(() => {
		if (contentRef.current) {
			contentRef.current.scrollTop = contentRef.current.scrollHeight
		}
	}, [event.arguments]) // Scroll when arguments update

	// Function to parse delta JSON
	const parseDelta = useCallback((deltaString: string) => {
		try {
			return JSON.parse(deltaString)
		} catch (e) {
			console.error('Failed to parse JSON:', e, deltaString)
			// Try to repair partial JSON using jsonrepair library
			try {
				const repaired = jsonrepair(deltaString)
				console.log('repaired', repaired)
				return JSON.parse(repaired)
			} catch (e) {
				console.error('Failed to repair JSON:', e, deltaString)
				// If repair fails, return as raw string
				return { raw: deltaString }
			}
		}
	}, [])

	// Function to copy content to clipboard
	const copyToClipboard = useCallback(async (content: string) => {
		try {
			await navigator.clipboard.writeText(content)
			setCopiedCode(content)
			setTimeout(() => setCopiedCode(null), 2000)
			toast.success('Copied to clipboard')
		} catch (err) {
			console.error('Failed to copy:', err)
			toast.error('Failed to copy to clipboard')
		}
	}, [])

	// Function to handle copying delta content
	const handleCopyDelta = useCallback(
		async (delta: any) => {
			const contentToCopy = delta.content || JSON.stringify(delta, null, 2)
			await copyToClipboard(contentToCopy)
		},
		[copyToClipboard]
	)

	const content = useMemo(() => {
		if (event.type === 'result') {
			const summary = getToolSummary(event)
			let details = null
			if (event.name === 'write_file') {
				let path = ''
				let fileContent = ''
				if (typeof event.arguments === 'string') {
					try {
						const args = JSON.parse(event.arguments)
						path = args.path || ''
						fileContent = args.content || ''
					} catch {}
				}
				details = (
					<Dialog>
						<DialogTrigger asChild>
							<Button variant="ghost" size="sm" className="h-6 text-xs" onClick={handleViewContents}>
								View Contents
							</Button>
						</DialogTrigger>
						<DialogContent className="max-w-2xl">
							<DialogHeader>
								<DialogTitle>File: {path}</DialogTitle>
							</DialogHeader>
							<ScrollArea className="max-h-[60vh]">
								<div className="relative group">
									<div className="absolute right-2 top-2 opacity-0 group-hover:opacity-100 transition-opacity">
										<button onClick={() => copyToClipboard(fileContent)} className="p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" title="Copy code">
											{copiedCode === fileContent ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
										</button>
									</div>
									<SyntaxHighlighterComp
										language="plaintext"
										style={vscDarkPlus}
										PreTag="div"
										className="rounded-lg max-h-[60vh]"
										showLineNumbers={true}
										wrapLines={true}
										wrapLongLines={true}
										customStyle={{ margin: 0, padding: '1rem' }}
									>
										{fileContent}
									</SyntaxHighlighterComp>
								</div>
							</ScrollArea>
						</DialogContent>
					</Dialog>
				)
			} else if (event.name === 'read_file') {
				details = <></>
			} else if (event.result) {
				details = (
					<Dialog>
						<DialogTrigger asChild>
							<Button variant="ghost" size="sm" className="h-6 text-xs" onClick={handleViewDetails}>
								View Details
							</Button>
						</DialogTrigger>
						<DialogContent className="max-w-2xl">
							<DialogHeader>
								<DialogTitle>{event.name} Result</DialogTitle>
							</DialogHeader>
							<ScrollArea className="max-h-[60vh]">
								<pre className="p-4 bg-muted rounded-lg overflow-x-auto">{JSON.stringify(event.result, null, 2)}</pre>
							</ScrollArea>
						</DialogContent>
					</Dialog>
				)
			}
			return (
				<ToolSummaryCard event={event} summary={summary}>
					{details}
				</ToolSummaryCard>
			)
		}

		// Handle tool_start events
		if (event.type === 'start') {
			return (
				<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
					<div className="animate-spin h-4 w-4 border-2 border-primary border-t-transparent rounded-full" />
					<span className="font-medium">Starting {event.name.replace(/_/g, ' ')}...</span>
					{event.args && (
						<div className="mt-2 text-xs text-muted-foreground bg-background/50 p-2 rounded">
							<div className="font-semibold mb-1">Arguments:</div>
							<pre className="overflow-x-auto">{JSON.stringify(event.args, null, 2)}</pre>
						</div>
					)}
				</div>
			)
		}
		// Handle tool_update events (accumulated arguments)
		if (event.type === 'update') {
			let updateContent = null
			let accumulatedArgs: any = {}
			if (event.arguments) {
				accumulatedArgs = parseDelta(event.arguments)
			}

			if (event.name === 'write_file' && accumulatedArgs) {
				const path = accumulatedArgs.path || ''
				const content = accumulatedArgs.content || ''

				// Determine language for syntax highlighting based on file extension
				const getLanguage = (filePath: string) => {
					const ext = filePath.split('.').pop()?.toLowerCase()
					switch (ext) {
						case 'js':
							return 'javascript'
						case 'ts':
							return 'typescript'
						case 'jsx':
							return 'javascript'
						case 'tsx':
							return 'typescript'
						case 'json':
							return 'json'
						case 'html':
							return 'html'
						case 'css':
							return 'css'
						case 'md':
							return 'markdown'
						case 'py':
							return 'python'
						case 'go':
							return 'go'
						case 'rs':
							return 'rust'
						case 'java':
							return 'java'
						case 'c':
							return 'c'
						case 'cpp':
							return 'cpp'
						case 'h':
							return 'cpp'
						case 'hpp':
							return 'cpp'
						case 'sql':
							return 'sql'
						case 'sh':
							return 'bash'
						case 'yml':
							return 'yaml'
						case 'yaml':
							return 'yaml'
						case 'xml':
							return 'xml'
						case 'txt':
							return 'plaintext'
						default:
							return 'plaintext'
					}
				}

				const language = getLanguage(path)

				updateContent = (
					<div className="mt-2 text-xs bg-background/50 rounded border border-border">
						<div className="sticky top-0 z-10 bg-background/95 backdrop-blur-sm border-b border-border p-3">
							<div className="flex justify-between items-center">
								{path && (
									<div className="font-semibold text-green-600 flex items-center gap-2">
										<Code className="w-3 h-3" />
										Writing file: {path}
									</div>
								)}
								<button onClick={() => handleCopyDelta(accumulatedArgs)} className="p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" title="Copy content">
									<Copy className="w-3 h-3" />
								</button>
							</div>
						</div>
						{content && (
							<div ref={contentRef} className="h-[300px] overflow-y-auto w-full">
								<SyntaxHighlighterComp
									language={language}
									style={vscDarkPlus}
									PreTag="div"
									className="rounded text-xs w-full"
									showLineNumbers={true}
									wrapLines={true}
									wrapLongLines={true}
									customStyle={{
										margin: 0,
										padding: '0.5rem',
										background: 'rgb(20, 20, 20)',
										fontSize: '11px',
										width: '100%',
										minHeight: '100%'
									}}
								>
									{content}
								</SyntaxHighlighterComp>
							</div>
						)}
						{!content && path && <div className="p-3 text-muted-foreground italic">Waiting for content...</div>}
						{!path && !content && <pre className="p-3 overflow-x-auto text-xs w-full">{JSON.stringify(accumulatedArgs, null, 2)}</pre>}
					</div>
				)
			} else if (event.name === 'read_file' && accumulatedArgs) {
				const path = accumulatedArgs.path || ''
				updateContent = (
					<div className="mt-2 text-xs bg-background/50 p-2 rounded border border-border relative">
						<button onClick={() => handleCopyDelta(accumulatedArgs)} className="absolute top-2 right-2 p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" title="Copy arguments">
							<Copy className="w-3 h-3" />
						</button>
						<div className="font-semibold text-blue-600 flex items-center gap-2">
							<Code className="w-3 h-3" />
							Reading file: {path || 'Unknown file'}
						</div>
					</div>
				)
			} else {
				updateContent = (
					<div className="mt-2 text-xs bg-background/50 p-2 rounded border border-border relative">
						<button
							onClick={() => handleCopyDelta(accumulatedArgs)}
							className="absolute top-2 right-2 p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors"
							title="Copy accumulated arguments"
						>
							<Copy className="w-3 h-3" />
						</button>
						<div className="font-semibold mb-1">Updating {event.name.replace(/_/g, ' ')}:</div>
						<pre className="overflow-x-auto text-xs">{JSON.stringify(accumulatedArgs, null, 2)}</pre>
					</div>
				)
			}

			return (
				<div className="text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
					<div className="flex items-center gap-2 mb-3">
						<svg className="mr-3 -ml-1 size-5 animate-spin text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
							<circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
							<path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						<span className="font-medium">Updating {event.name.replace(/_/g, ' ')}...</span>
					</div>
					{updateContent}
				</div>
			)
		}

		// Handle tool_execute events
		if (event.type === 'execute') {
			return (
				<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
					<div className="animate-spin h-4 w-4 border-2 border-green-500 border-t-transparent rounded-full" />
					<span className="font-medium">Executing {event.name.replace(/_/g, ' ')}...</span>
					{event.arguments && (
						<div className="mt-2 text-xs bg-background/50 p-2 rounded">
							<div className="font-semibold mb-1">Final arguments:</div>
							<pre className="overflow-x-auto">{event.arguments}</pre>
						</div>
					)}
				</div>
			)
		}

		return null
	}, [event, handleViewContents, handleViewDetails, parseDelta, copyToClipboard, copiedCode, handleCopyDelta])

	return content
})

interface ActiveToolProps {
	tool: ToolExecution
	partialArgs: Record<string, unknown> | null
}

const ActiveTool = React.memo(({ tool, partialArgs }: ActiveToolProps) => {
	const statusText = useMemo(() => {
		switch (tool.status) {
			case 'started':
				return `Starting ${tool.name}...`
			case 'updating':
				if (tool.name === 'write_file') {
					const path = partialArgs?.path || ''
					return `Writing file to ${path}`
				} else if (tool.name === 'read_file') {
					const path = partialArgs?.path || ''
					return `Reading file ${path}`
				}
				return `Processing ${tool.name}...`
			case 'executing':
				if (tool.name === 'write_file') {
					const path = partialArgs?.path || ''
					return `Saving file to ${path}`
				} else if (tool.name === 'read_file') {
					const path = partialArgs?.path || ''
					return `Loading file ${path}`
				}
				return `Executing ${tool.name}...`
			case 'completed':
				if (tool.name === 'write_file') {
					const path = partialArgs?.path || ''
					return `File written to ${path}`
				} else if (tool.name === 'read_file') {
					const path = partialArgs?.path || ''
					return `File ${path} loaded successfully`
				}
				return `Completed ${tool.name}`
			case 'error':
				if (tool.name === 'write_file') {
					const path = partialArgs?.path || ''
					return `Failed to write file ${path}: ${tool.error}`
				} else if (tool.name === 'read_file') {
					const path = partialArgs?.path || ''
					return `Failed to read file ${path}: ${tool.error}`
				}
				return `Error in ${tool.name}: ${tool.error}`
			default:
				return null
		}
	}, [tool, partialArgs])

	return (
		<div className="flex flex-col gap-2 text-sm text-muted-foreground">
			<div className="flex items-center gap-2">
				<div className="animate-spin h-4 w-4 border-2 border-primary border-t-transparent rounded-full" />
				<span>{statusText}</span>
			</div>
			{partialArgs && (
				<div className="mt-1 text-xs text-gray-400">
					<div className="font-semibold mb-1">Arguments:</div>
					<pre className="bg-muted/50 p-2 rounded overflow-x-auto">{JSON.stringify(partialArgs, null, 2)}</pre>
				</div>
			)}
		</div>
	)
})

interface CommitDetailsProps {
	projectId: number
	commitHash: string
	onClose?: () => void
	commit: ProjectsCommitWithFileChangesApi
}

const CommitDetails = ({ projectId, commit, commitHash }: CommitDetailsProps) => {
	const [open, setOpen] = useState(false)
	const { data: commitDetails } = useQuery({
		queryKey: ['commit-details', projectId, commitHash],
		queryFn: () =>
			getChaincodeProjectsByIdCommitsByCommitHash({
				path: { id: projectId, commitHash },
			}),
	})

	// Fetch all commits to find the parent commit
	const { data: commitsData } = useQuery({
		queryKey: ['commits', projectId],
		queryFn: () => getChaincodeProjectsByIdCommits({ path: { id: projectId } }),
	})

	const [selectedFile, setSelectedFile] = useState<string | null>(null)
	const [parentCommitHash, setParentCommitHash] = useState<string | null>(null)

	// Find the parent commit hash for the current commit
	useEffect(() => {
		if (!selectedFile || !commitsData?.data?.commits) return
		const commits = commitsData.data.commits
		const commit = commits.find((c) => c.hash === commitHash)
		const parentHash = commit?.parent
		setParentCommitHash(parentHash)
	}, [selectedFile, commitHash, commitsData?.data?.commits])

	const { data: currentFileContent } = useQuery({
		queryKey: ['file-at-commit', projectId, commitHash, selectedFile],
		queryFn: () =>
			getChaincodeProjectsByIdFileAtCommit({
				path: { id: projectId },
				query: { commit: commitHash || '', file: selectedFile || '' },
			}),
		enabled: !!selectedFile,
	})

	const { data: parentFileContent } = useQuery({
		queryKey: ['file-at-commit', projectId, parentCommitHash, selectedFile],
		queryFn: () =>
			getChaincodeProjectsByIdFileAtCommit({
				path: { id: projectId },
				query: { commit: parentCommitHash || '', file: selectedFile || '' },
			}),
		enabled: !!selectedFile && !!parentCommitHash,
	})

	const diffEditorRef = useRef<monaco.editor.IStandaloneDiffEditor | null>(null)
	const diffContainerRef = useRef<HTMLDivElement>(null)
	useEffect(() => {
		if (!diffContainerRef.current || !selectedFile) return

		const language = getMonacoLanguage(selectedFile)

		// Dispose previous diff editor
		if (diffEditorRef.current) {
			diffEditorRef.current.dispose()
		}

		// Dispose previous models with the same URI if they exist
		const originalUri = monaco.Uri.parse(`inmemory://original/${selectedFile}`)
		const modifiedUri = monaco.Uri.parse(`inmemory://modified/${selectedFile}`)
		const originalModel = monaco.editor.getModel(originalUri)
		const modifiedModel = monaco.editor.getModel(modifiedUri)
		if (originalModel) originalModel.dispose()
		if (modifiedModel) modifiedModel.dispose()

		const diffEditor = monaco.editor.createDiffEditor(diffContainerRef.current, {
			readOnly: true,
			renderSideBySide: true,
			originalEditable: false,
			diffWordWrap: 'on',
			renderOverviewRuler: true,
			scrollBeyondLastLine: false,
			automaticLayout: true,
			// Enable diff navigation
			enableSplitViewResizing: true,
			ignoreTrimWhitespace: false,
			renderIndicators: true,
		})

		let originalContent = ''
		if (commitDetails?.data?.added?.includes(selectedFile)) {
			originalContent = ''
		} else {
			originalContent = parentFileContent?.data || ''
		}

		const newOriginalModel = monaco.editor.createModel(originalContent, language, originalUri)
		const newModifiedModel = monaco.editor.createModel(currentFileContent?.data || '', language, modifiedUri)

		diffEditor.setModel({
			original: newOriginalModel,
			modified: newModifiedModel,
		})

		diffEditorRef.current = diffEditor

		return () => {
			diffEditor.dispose()
			newOriginalModel.dispose()
			newModifiedModel.dispose()
		}
	}, [commitDetails, currentFileContent, selectedFile, parentFileContent])
	useEffect(() => {
		if (!open) {
			setSelectedFile(null)
		} else {
			const firstChangedFile = commitDetails?.data?.added?.[0] || commitDetails?.data?.modified?.[0] || commitDetails?.data?.removed?.[0] || null
			setSelectedFile(firstChangedFile)
		}
	}, [open, commitDetails?.data?.added])
	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger asChild>
				<div>
					<div className="text-sm font-medium">{commit.message}</div>
					<div className="text-xs text-muted-foreground mt-1">{new Date(commit.timestamp || '').toLocaleString()}</div>
					<div className="text-xs text-muted-foreground mt-1">{commit.author}</div>
					{(commit.added?.length || commit.modified?.length || commit.removed?.length) && (
						<div className="flex gap-4 text-xs text-muted-foreground mt-2">
							{commit.added?.length ? <div className="text-green-500">Added: {commit.added.length}</div> : null}
							{commit.modified?.length ? <div className="text-yellow-500">Modified: {commit.modified.length}</div> : null}
							{commit.removed?.length ? <div className="text-red-500">Removed: {commit.removed.length}</div> : null}
						</div>
					)}
				</div>
			</DialogTrigger>

			<DialogContent className="max-w-4xl h-[80vh] p-0">
				<div className="flex flex-col h-full">
					<DialogHeader className="px-6 pt-6 pb-2">
						<div className="flex items-center gap-2">
							<button onClick={() => setOpen(false)} className="mr-2 p-1 rounded hover:bg-accent">
								<ArrowLeft className="h-5 w-5" />
							</button>
							<GitCommit className="h-5 w-5" />
							<DialogTitle className="flex-1">Commit Details</DialogTitle>
						</div>
					</DialogHeader>
					<div className="px-6 pb-4">
						<div className="space-y-2">
							<div className="text-sm font-medium">{commitDetails?.data?.message}</div>
							<div className="text-xs text-muted-foreground">{new Date(commitDetails?.data?.timestamp || '').toLocaleString()}</div>
							<div className="text-xs text-muted-foreground">Author: {commitDetails?.data?.author}</div>
							{(commitDetails?.data?.added?.length || commitDetails?.data?.modified?.length || commitDetails?.data?.removed?.length) && (
								<div className="flex gap-4 text-xs text-muted-foreground">
									{commitDetails?.data?.added?.length ? <div className="text-green-500">Added: {commitDetails.data.added.length}</div> : null}
									{commitDetails?.data?.modified?.length ? <div className="text-yellow-500">Modified: {commitDetails.data.modified.length}</div> : null}
									{commitDetails?.data?.removed?.length ? <div className="text-red-500">Removed: {commitDetails.data.removed.length}</div> : null}
								</div>
							)}
						</div>
					</div>
					<div className="flex-1 min-h-0 flex gap-4 px-6 pb-6">
						{/* File list */}
						<ScrollArea className="flex-shrink-0 w-56 h-full border rounded-lg bg-muted/30">
							<div className="p-2 space-y-1">
								{commitDetails?.data?.added?.map((file) => (
									<div
										key={file}
										className={`text-sm cursor-pointer p-1 rounded ${selectedFile === file ? 'bg-accent font-bold' : 'text-green-500 hover:bg-accent'}`}
										onClick={() => setSelectedFile(file)}
									>
										+ {file}
									</div>
								))}
								{commitDetails?.data?.modified?.map((file) => (
									<div
										key={file}
										className={`text-sm cursor-pointer p-1 rounded ${selectedFile === file ? 'bg-accent font-bold' : 'text-yellow-500 hover:bg-accent'}`}
										onClick={() => setSelectedFile(file)}
									>
										~ {file}
									</div>
								))}
								{commitDetails?.data?.removed?.map((file) => (
									<div
										key={file}
										className={`text-sm cursor-pointer p-1 rounded ${selectedFile === file ? 'bg-accent font-bold' : 'text-red-500 hover:bg-accent'}`}
										onClick={() => setSelectedFile(file)}
									>
										- {file}
									</div>
								))}
							</div>
						</ScrollArea>
						{/* Diff area */}
						<div className="flex-1 min-h-0 flex flex-col">
							{selectedFile ? (
								<>
									<div className="text-sm font-medium mb-2">{selectedFile}</div>
									<div ref={diffContainerRef} className="flex-1 border rounded-lg bg-background" />
								</>
							) : (
								<div className="flex items-center justify-center h-full text-muted-foreground">Select a file to view its diff</div>
							)}
						</div>
					</div>
				</div>
			</DialogContent>
		</Dialog>
	)
}
