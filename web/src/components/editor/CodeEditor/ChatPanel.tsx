import { ProjectsCommitWithFileChangesApi } from '@/api/client'
import {
	getAiByProjectIdConversationsByConversationIdOptions,
	getAiByProjectIdConversationsOptions,
	getChaincodeProjectsByIdCommitsByCommitHashOptions,
	getChaincodeProjectsByIdCommitsOptions,
	getChaincodeProjectsByIdFileAtCommitOptions,
	postAiByProjectIdConversationsMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useMutation, useQuery } from '@tanstack/react-query'
import { jsonrepair } from 'jsonrepair'
import { ArrowLeft, Check, ChevronDown, Copy, GitCommit, History, Plus, Square } from 'lucide-react'
import * as monaco from 'monaco-editor'
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Components } from 'react-markdown'
import ReactMarkdown from 'react-markdown'
import { useSearchParams } from 'react-router-dom'
import type { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { toast } from 'sonner'
import { ToolEventRenderer } from './tools/ToolEventRenderer'
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
	result?: unknown
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
	handleStop: () => void
}

export function useStreamingChat(projectId: number, conversationId: number, onToolResult?: (toolName: string, result: unknown) => void, onComplete?: () => void): UseStreamingChatResult {
	const [messages, setMessages] = useState<Message[]>([])
	const [input, setInput] = useState('')
	const [isLoading, setIsLoading] = useState(false)
	const [activeTool, setActiveTool] = useState<ToolExecution | null>(null)
	const abortRef = useRef<AbortController | null>(null)
	const partialArgsRef = useRef<string>('')

	const handleStop = useCallback(() => {
		if (abortRef.current) {
			abortRef.current.abort()
		}
	}, [])

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
						conversationId,
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
											case 'tool_start':
											case 'tool_update':
											case 'tool_execute':
											case 'tool_result': {
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
														const newPart = {
															type: 'tool' as const,
															toolEvent: {
																type: mappingBetweenToolEvents[event.type],
																toolCallID: toolEvent.toolCallID,
																name: toolEvent.name,
																arguments: event.type === 'tool_update' ? event.arguments : undefined,
																result: event.type === 'tool_result' ? event.result : undefined,
																error: event.type === 'tool_result' ? event.error : undefined,
															},
														}
														return [...prev, { role: 'assistant', parts: [newPart] }] as Message[]
													}
													const lastMsg = prev[lastMsgIdx]
													const updatedParts = [...lastMsg.parts]
													const toolPartIdx = updatedParts.findIndex((part) => part.type === 'tool' && part.toolEvent?.toolCallID === toolEvent.toolCallID)
													if (toolPartIdx !== -1) {
														const existingToolEvent = updatedParts[toolPartIdx].toolEvent
														let mergedArguments = existingToolEvent?.arguments || ''

														if (event.type === 'tool_update' && event.arguments) {
															if (mergedArguments === '{}' || mergedArguments === '') {
																mergedArguments = event.arguments
															} else {
																mergedArguments += event.arguments
															}
														} else if (event.type === 'tool_execute') {
															mergedArguments = JSON.stringify(event.args)
														}

														const updatedToolEvent: ToolEvent = {
															...(existingToolEvent as ToolEvent),
															type: mappingBetweenToolEvents[event.type],
															toolCallID: event.toolCallID,
															name: event.name,
															arguments: mergedArguments,
															result: event.type === 'tool_result' ? event.result : existingToolEvent?.result,
															error: event.type === 'tool_result' ? event.error : existingToolEvent?.error,
														}

														updatedParts[toolPartIdx] = {
															type: 'tool',
															toolEvent: updatedToolEvent,
														}
													} else {
														updatedParts.push({
															type: 'tool',
															toolEvent: {
																type: mappingBetweenToolEvents[event.type],
																toolCallID: toolEvent.toolCallID,
																name: toolEvent.name,
																arguments: event.type === 'tool_update' ? event.arguments : undefined,
																result: event.type === 'tool_result' ? event.result : undefined,
																error: event.type === 'tool_result' ? event.error : undefined,
															},
														})
													}
													return [...prev.slice(0, lastMsgIdx), { ...lastMsg, parts: updatedParts }]
												})

												if (event.type === 'tool_result') {
													setActiveTool((prev) => {
														if (prev && prev.toolCallID === event.toolCallID) {
															return {
																...prev,
																status: event.error ? 'error' : 'completed',
																result: event.result,
																error: event.error,
															}
														}
														return prev
													})
													partialArgsRef.current = ''
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
				// Only clear active tool if it doesn't have a result (i.e., it wasn't completed)
				setActiveTool((prev) => prev && (prev.result !== undefined || prev.error) ? prev : null)
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

	return { messages, input, setInput, isLoading, activeTool, handleSubmit, partialArgsRef, setMessages, handleStop }
}

type ChatPanelProps = {
	projectId: number
	handleToolResult: (toolName: string, result: unknown) => void
	handleChatComplete: () => void
}

export function ChatPanel({ projectId = 1, handleToolResult, handleChatComplete }: ChatPanelProps) {
	const [partialArgs, setPartialArgs] = useState<Record<string, unknown> | null>(null)
	const [firstConversationId, setFirstConversationId] = useState<string | null>(null)
	const [historyDialogOpen, setHistoryDialogOpen] = useState(false)
	const messagesEndRef = useRef<HTMLDivElement>(null)
	const { textareaRef, adjustHeight } = useAutoResizeTextarea()
	const { data: conversations, refetch: refetchConversations } = useQuery({
		...getAiByProjectIdConversationsOptions({ path: { projectId } }),
	})
	const [searchParams] = useSearchParams()
	useEffect(() => {
		const conversationId = searchParams.get('conversation')
		if (conversationId) {
			setFirstConversationId(conversationId)
		}
	}, [searchParams])
	const chatState = useStreamingChat(projectId, firstConversationId ? parseInt(firstConversationId, 10) : undefined, handleToolResult, handleChatComplete)
	const { data: conversationDetails } = useQuery({
		...getAiByProjectIdConversationsByConversationIdOptions({
			path: {
				projectId,
				conversationId: firstConversationId ? parseInt(firstConversationId, 10) : undefined,
			},
		}),
		enabled: !!conversations?.length,
	})
	const { data: conversationMessages } = useQuery({
		...getAiByProjectIdConversationsByConversationIdOptions({
			path: {
				projectId,
				conversationId: parseInt(firstConversationId!, 10),
			},
		}),
		enabled: !!firstConversationId,
	})
	const { data: commits } = useQuery({
		...getChaincodeProjectsByIdCommitsOptions({ path: { id: projectId } }),
	})
	const { messages, input, setInput, isLoading, activeTool, handleSubmit, partialArgsRef, setMessages, handleStop } = chatState
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
		(data: typeof conversationMessages) => {
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
								result: tool.result && tool.result ? (typeof tool.result === 'string' ? tool.result : JSON.stringify(tool.result)) : '',
								error: tool.error && tool.error ? (typeof tool.error === 'string' ? tool.error : JSON.stringify(tool.error)) : '',
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
		if (conversations?.length > 0 && !firstConversationId) {
			setFirstConversation(conversations[conversations.length - 1].id?.toString() || '')
		}
	}, [conversations?.length, firstConversationId, setFirstConversation])

	// Effect to set messages from conversation
	useEffect(() => {
		formatMessages(conversationMessages)
	}, [conversationMessages, formatMessages])

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
	const createConversationMutation = useMutation({
		...postAiByProjectIdConversationsMutation({}),
		onSuccess: (data) => {
			setFirstConversationId(data.id.toString())
			// Clear messages when creating new conversation
			setMessages([])
			// Invalidate conversations query to refresh the list
			refetchConversations()
		},
		onError: (error) => {
			console.error('Failed to create conversation:', error)
			toast.error('Failed to create new conversation')
		},
	})
	const createNewConversation = useCallback(async () => {
		createConversationMutation.mutate({
			path: { projectId },
			body: {
				title: `New Conversation @${new Date().toISOString()}`,
			},
		})
	}, [createConversationMutation])

	return (
		<div className="flex flex-col h-full bg-background border-r border-border text-foreground">
			<div className="p-2 border-b border-border font-semibold text-sm flex items-center justify-between">
				<div className="flex items-center gap-2">
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button variant="ghost" size="sm" className="h-8 gap-1 font-normal">
								{conversations?.find((c) => c.id?.toString() === firstConversationId)?.id ? `Chat #${firstConversationId}` : 'Select Chat'}
								<ChevronDown className="h-4 w-4" />
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="start" className="w-[200px]">
							<DropdownMenuItem className="gap-2" onClick={createNewConversation}>
								<Plus className="h-4 w-4" />
								New Conversation
							</DropdownMenuItem>
							{conversations?.map((conversation) => (
								<DropdownMenuItem key={conversation.id} className="gap-2" onClick={() => setFirstConversation(conversation.id?.toString() || '')}>
									{conversation.id?.toString() === firstConversationId && <Check className="h-4 w-4" />}
									<span className={conversation.id?.toString() === firstConversationId ? 'font-medium' : ''}>Chat #{conversation.id}</span>
								</DropdownMenuItem>
							))}
						</DropdownMenuContent>
					</DropdownMenu>
					<Button variant="ghost" size="sm" onClick={createNewConversation} className="h-8">
						<Plus className="h-4 w-4" />
					</Button>
				</div>
				<Dialog open={historyDialogOpen} onOpenChange={setHistoryDialogOpen}>
					<DialogTrigger asChild>
						<Button variant="ghost" size="sm" className="h-8">
							<History className="h-4 w-4" />
						</Button>
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
										{conversations?.map((conversation, index) => (
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
										{commits?.commits?.map((commit) => (
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
				{isLoading ? (
					<button
						type="button"
						onClick={handleStop}
						className="px-3 py-1 rounded bg-destructive text-destructive-foreground hover:bg-destructive/90 text-sm self-end flex items-center gap-1"
					>
						<Square className="h-4 w-4" />
						Stop
					</button>
				) : (
					<button
						type="submit"
						className="px-3 py-1 rounded bg-primary text-primary-foreground text-sm self-end"
						disabled={!input.trim()}
					>
						Send
					</button>
				)}
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
	const [copiedMessage, setCopiedMessage] = useState<string | null>(null)

	const copyToClipboard = async (content: string) => {
		try {
			await navigator.clipboard.writeText(content)
			setCopiedMessage(content)
			setTimeout(() => setCopiedMessage(null), 2000)
		} catch (err) {
			console.error('Failed to copy message:', err)
			toast.error('Failed to copy message')
		}
	}

	const getMessageContent = useCallback(() => {
		let content = ''
		
		// Add role indicator
		content += `${message.role === 'user' ? 'User' : 'Assistant'}:\n\n`
		
		// Add each part's content
		message.parts.forEach((part, index) => {
			if (part.type === 'text' && part.content) {
				content += part.content
				if (index < message.parts.length - 1) {
					content += '\n\n'
				}
			} else if (part.type === 'tool' && part.toolEvent) {
				content += `[Tool: ${part.toolEvent.name}]\n`
				if (part.toolEvent.arguments) {
					try {
						const args = JSON.parse(part.toolEvent.arguments)
						content += `Arguments: ${JSON.stringify(args, null, 2)}\n`
					} catch {
						content += `Arguments: ${part.toolEvent.arguments}\n`
					}
				}
				if (part.toolEvent.result) {
					content += `Result: ${JSON.stringify(part.toolEvent.result, null, 2)}\n`
				}
				if (part.toolEvent.error) {
					content += `Error: ${part.toolEvent.error}\n`
				}
				if (index < message.parts.length - 1) {
					content += '\n\n'
				}
			}
		})
		
		return content
	}, [message])

	const handleCopyMessage = () => {
		const messageContent = getMessageContent()
		copyToClipboard(messageContent)
	}

	const messageContent = useMemo(
		() => (
			<div className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'} relative group`}>
				<div className={`max-w-[80%] rounded-lg p-3 ${message.role === 'user' ? 'bg-muted' : 'bg-muted'} relative`}>
					{/* Copy button - positioned at top right */}
					<button
						onClick={handleCopyMessage}
						className="absolute top-2 right-2 p-1.5 rounded bg-background/80 hover:bg-background transition-colors opacity-0 group-hover:opacity-100 z-10"
						title="Copy message"
					>
						{copiedMessage === getMessageContent() ? (
							<Check className="w-3 h-3 text-green-500" />
						) : (
							<Copy className="w-3 h-3" />
						)}
					</button>
					
					{message.parts.map((part, index) => {
						if (part.type === 'text' && part.content) {
							return <MarkdownRenderer key={index} content={part.content} />
						}
						if (part.type === 'tool' && part.toolEvent) {
							return <ToolEventRenderer key={index} event={part.toolEvent} />
						}
						return null
					})}
				</div>
			</div>
		),
		[message, copiedMessage, getMessageContent, handleCopyMessage]
	)

	return messageContent
})

interface ActiveToolProps {
	tool: ToolExecution
	partialArgs: Record<string, unknown> | null
}

const ActiveTool = ({ tool, partialArgs }: ActiveToolProps) => {
	const isCompleted = tool.status === 'completed' || tool.status === 'error'
	const isError = tool.status === 'error'
	
	return (
		<div className="text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="flex items-center gap-2 mb-3">
				{!isCompleted ? (
					<svg className="mr-3 -ml-1 size-5 animate-spin text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
						<circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
						<path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
				) : isError ? (
					<div className="mr-3 -ml-1 size-5 rounded-full bg-red-500 flex items-center justify-center">
						<span className="text-white text-xs">!</span>
					</div>
				) : (
					<div className="mr-3 -ml-1 size-5 rounded-full bg-green-500 flex items-center justify-center">
						<Check className="h-3 w-3 text-white" />
					</div>
				)}
				<span className="font-medium">
					{isCompleted 
						? (isError ? `Error in ${tool.name.replace(/_/g, ' ')}` : `Completed ${tool.name.replace(/_/g, ' ')}`)
						: `Executing ${tool.name.replace(/_/g, ' ')}...`
					}
				</span>
			</div>
			{partialArgs && (
				<div className="mt-2 text-xs bg-background/50 p-2 rounded border border-border">
					<div className="font-semibold mb-1">Arguments:</div>
					<pre className="overflow-x-auto">{JSON.stringify(partialArgs, null, 2)}</pre>
				</div>
			)}
			{tool.result && (
				<div className="mt-2 text-xs bg-background/50 p-2 rounded border border-border">
					<div className="font-semibold mb-1">Result:</div>
					<pre className="overflow-x-auto">{JSON.stringify(tool.result, null, 2)}</pre>
				</div>
			)}
			{tool.error && (
				<div className="mt-2 text-xs bg-red-50 p-2 rounded border border-red-200">
					<div className="font-semibold mb-1 text-red-700">Error:</div>
					<pre className="overflow-x-auto text-red-600">{tool.error}</pre>
				</div>
			)}
		</div>
	)
}

interface CommitDetailsProps {
	projectId: number
	commitHash: string
	onClose?: () => void
	commit: ProjectsCommitWithFileChangesApi
}

const CommitDetails = ({ projectId, commit, commitHash }: CommitDetailsProps) => {
	const [open, setOpen] = useState(false)
	const { data: commitDetails } = useQuery({
		...getChaincodeProjectsByIdCommitsByCommitHashOptions({ path: { id: projectId, commitHash } }),
	})

	// Fetch all commits to find the parent commit
	const { data: commitsData } = useQuery({
		...getChaincodeProjectsByIdCommitsOptions({ path: { id: projectId } }),
	})

	const [selectedFile, setSelectedFile] = useState<string | null>(null)
	const [parentCommitHash, setParentCommitHash] = useState<string | null>(null)

	// Find the parent commit hash for the current commit
	useEffect(() => {
		if (!commitsData?.commits) return
		const commits = commitsData.commits
		const commit = commits.find((c) => c.hash === commitHash)
		const parentHash = commit?.parent
		setParentCommitHash(parentHash)
	}, [commitHash, commitsData?.commits])

	const { data: currentFileContent } = useQuery({
		...getChaincodeProjectsByIdFileAtCommitOptions({ path: { id: projectId }, query: { commit: commitHash || '', file: selectedFile || '' } }),
		enabled: !!selectedFile,
	})

	const { data: parentFileContent } = useQuery({
		...getChaincodeProjectsByIdFileAtCommitOptions({ path: { id: projectId }, query: { commit: parentCommitHash || '', file: selectedFile || '' } }),
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
		if (commitDetails?.added?.includes(selectedFile)) {
			originalContent = ''
		} else {
			originalContent = parentFileContent || ''
		}

		const newOriginalModel = monaco.editor.createModel(originalContent, language, originalUri)
		const newModifiedModel = monaco.editor.createModel(currentFileContent || '', language, modifiedUri)

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
	}, [commitDetails?.added, currentFileContent, selectedFile, parentFileContent])
	
	useEffect(() => {
		if (!open) {
			setSelectedFile(null)
		} else if (commitDetails && !selectedFile) {
			const firstChangedFile = commitDetails.added?.[0] || commitDetails.modified?.[0] || commitDetails.removed?.[0] || null
			setSelectedFile(firstChangedFile)
		}
	}, [open, commitDetails, selectedFile])
	
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
							<div className="text-sm font-medium">{commitDetails?.message}</div>
							<div className="text-xs text-muted-foreground">{new Date(commitDetails?.timestamp || '').toLocaleString()}</div>
							<div className="text-xs text-muted-foreground">Author: {commitDetails?.author}</div>
							{(commitDetails?.added?.length || commitDetails?.modified?.length || commitDetails?.removed?.length) && (
								<div className="flex gap-4 text-xs text-muted-foreground">
									{commitDetails?.added?.length ? <div className="text-green-500">Added: {commitDetails.added.length}</div> : null}
									{commitDetails?.modified?.length ? <div className="text-yellow-500">Modified: {commitDetails.modified.length}</div> : null}
									{commitDetails?.removed?.length ? <div className="text-red-500">Removed: {commitDetails.removed.length}</div> : null}
								</div>
							)}
						</div>
					</div>
					<div className="flex-1 min-h-0 flex gap-4 px-6 pb-6">
						{/* File list */}
						<ScrollArea className="flex-shrink-0 w-56 h-full border rounded-lg bg-muted/30">
							<div className="p-2 space-y-1">
								{commitDetails?.added?.map((file) => (
									<div
										key={file}
										className={`text-sm cursor-pointer p-1 rounded ${selectedFile === file ? 'bg-accent font-bold' : 'text-green-500 hover:bg-accent'}`}
										onClick={() => setSelectedFile(file)}
									>
										+ {file}
									</div>
								))}
								{commitDetails?.modified?.map((file) => (
									<div
										key={file}
										className={`text-sm cursor-pointer p-1 rounded ${selectedFile === file ? 'bg-accent font-bold' : 'text-yellow-500 hover:bg-accent'}`}
										onClick={() => setSelectedFile(file)}
									>
										~ {file}
									</div>
								))}
								{commitDetails?.removed?.map((file) => (
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
