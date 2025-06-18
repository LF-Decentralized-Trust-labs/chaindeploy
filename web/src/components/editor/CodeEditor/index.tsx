import { getChaincodeProjectsByIdCommits, getProjectsByProjectIdFilesEntries, getProjectsByProjectIdFilesRead, postProjectsByProjectIdFilesWrite, ProjectsProject } from '@/api/client'
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable'
import { useQuery } from '@tanstack/react-query'
import type { editor } from 'monaco-editor'
import React, { useCallback, useEffect, useRef, useState } from 'react'
import type { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { toast } from 'sonner'
import { ChatPanel, useStreamingChat } from './ChatPanel'
import { EditorContent } from './EditorContent'
import { EditorTabs } from './EditorTabs'
import { FileTree } from './FileTree'
import { LogsPanel } from './LogsPanel'
import { Playground } from './Playground'
import type { File } from './types'
const SyntaxHighlighterComp = SyntaxHighlighter as unknown as React.ComponentType<SyntaxHighlighterProps>

interface CodeEditorProps {
	mode?: 'editor' | 'playground'
	projectId?: number
	chaincodeProject: ProjectsProject
}

export function CodeEditor({ mode = 'editor', projectId, chaincodeProject }: CodeEditorProps) {
	const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null)
	const [openFolders, setOpenFolders] = useState<Record<string, boolean>>({})
	const [selectedFile, setSelectedFile] = useState<File | null>(null)
	const [openTabs, setOpenTabs] = useState<File[]>([])
	const [dirtyFiles, setDirtyFiles] = useState<string[]>([])

	const { data: treeData, refetch: refetchTree } = useQuery({
		queryKey: ['files', projectId],
		queryFn: () => getProjectsByProjectIdFilesEntries({ path: { projectId: projectId } }),
	})

	const tree = treeData?.data

	const { refetch: refetchCommits } = useQuery({
		queryKey: ['commits', projectId],
		queryFn: () => getChaincodeProjectsByIdCommits({ path: { id: projectId } }),
	})

	const handleFileClick = useCallback(
		async (file: { name: string; path: string }) => {
			try {
				const response = await getProjectsByProjectIdFilesRead({
					path: { projectId },
					query: { path: file.path },
				})
				const fileData = {
					name: file.name,
					path: file.path,
					content: response.data.content,
					language: file.path.split('.').pop() || 'plaintext',
				}
				setSelectedFile(fileData)
				if (!openTabs.find((tab) => tab.name === file.name)) {
					setOpenTabs([...openTabs, fileData])
				}
			} catch {
				toast.error('Failed to load file')
			}
		},
		[openTabs, projectId]
	)

	const reloadCurrentFile = useCallback(async () => {
		if (selectedFile) {
			try {
				const response = await getProjectsByProjectIdFilesRead({
					path: { projectId },
					query: { path: selectedFile.path },
				})
				const updatedFile = {
					...selectedFile,
					content: response.data.content,
				}
				setSelectedFile(updatedFile)
				setOpenTabs(openTabs.map((tab) => (tab.name === selectedFile.name ? updatedFile : tab)))
			} catch {
				toast.error('Failed to reload file contents')
			}
		}
	}, [projectId, selectedFile, openTabs])

	const handleToolResult = useCallback(
		async (toolName: string) => {
			if (toolName === 'write_file') {
				// Reload the file tree
				await refetchTree()
				await refetchCommits()
				// Reload the current file if it exists
				await reloadCurrentFile()
			}
		},
		[projectId, refetchTree, reloadCurrentFile, refetchCommits]
	)

	const handleChatComplete = useCallback(async () => {
		// After the full response is received, update the file tree and reload the current file
		await refetchTree()
		await reloadCurrentFile()
		await refetchCommits()
	}, [refetchTree, reloadCurrentFile, refetchCommits])

	const chatState = useStreamingChat(projectId, handleToolResult, handleChatComplete)

	const handleEditorChange = useCallback(
		(value: string | undefined) => {
			if (selectedFile && value !== undefined) {
				setOpenTabs(openTabs.map((tab) => (tab.name === selectedFile.name ? { ...tab, content: value, hasUnsavedChanges: true } : tab)))
				if (!dirtyFiles.includes(selectedFile.name)) {
					setDirtyFiles([...dirtyFiles, selectedFile.name])
				}
			}
		},
		[selectedFile, openTabs, dirtyFiles, setDirtyFiles]
	)

	const handleEditorMount = useCallback((editor: editor.IStandaloneCodeEditor) => {
		editorRef.current = editor
	}, [])

	const handleSave = useCallback(async () => {
		if (!selectedFile || !editorRef.current) return

		try {
			const content = editorRef.current.getValue()
			await postProjectsByProjectIdFilesWrite({
				path: { projectId },
				body: {
					path: selectedFile.path,
					content,
				},
			})

			setOpenTabs(openTabs.map((tab) => (tab.name === selectedFile.name ? { ...tab, content, hasUnsavedChanges: false } : tab)))
			setDirtyFiles(dirtyFiles.filter((name) => name !== selectedFile.name))
			await refetchTree()

			toast.success('File saved', {
				description: `${selectedFile.path} has been saved successfully.`,
			})
		} catch (err) {
			console.error('Error saving file:', err)
			toast.error('Error saving file', {
				description: 'There was an error saving the file. Please try again.',
			})
		}
	}, [openTabs, selectedFile, editorRef])

	const handleTabClick = useCallback(
		(file: File) => {
			setSelectedFile(file)
		},
		[setSelectedFile]
	)

	const handleTabClose = useCallback(
		(file: File) => {
			setOpenTabs(openTabs.filter((tab) => tab.name !== file.name))
			setDirtyFiles(dirtyFiles.filter((name) => name !== file.name))
		},
		[openTabs, dirtyFiles]
	)

	useEffect(() => {
		const handleKeyDown = (e: KeyboardEvent) => {
			if ((e.metaKey || e.ctrlKey) && e.key === 's') {
				e.preventDefault()
				handleSave()
			}
		}

		window.addEventListener('keydown', handleKeyDown)
		return () => window.removeEventListener('keydown', handleKeyDown)
	}, [selectedFile])

	return (
		<div className="h-full max-h-[90vh] flex flex-col">
			<ResizablePanelGroup direction="horizontal">
				<ResizablePanel defaultSize={20} minSize={10} maxSize={40}>
					<ChatPanel projectId={projectId} chatState={chatState} />
				</ResizablePanel>
				<ResizableHandle />
				<ResizablePanel defaultSize={80} minSize={40} maxSize={90}>
					{mode === 'editor' ? (
						<ResizablePanelGroup direction="vertical">
							<ResizablePanel defaultSize={80} minSize={40}>
								<div className="grid h-full grid-rows-[auto_1fr] bg-background text-foreground">
									<EditorTabs openTabs={openTabs} selectedFile={selectedFile} handleTabClick={handleTabClick} handleTabClose={handleTabClose} dirtyFiles={dirtyFiles} />
									<div className="grid grid-rows-1">
										<ResizablePanelGroup direction="horizontal">
											<ResizablePanel defaultSize={20} minSize={15} maxSize={30}>
												<div className="h-full border-r border-border">
													<FileTree
														projectId={projectId}
														node={tree}
														openFolders={openFolders}
														setOpenFolders={setOpenFolders}
														selectedFile={selectedFile}
														handleFileClick={handleFileClick}
														refetchTree={refetchTree}
														isRoot={true}
													/>
												</div>
											</ResizablePanel>
											<ResizableHandle />
											<ResizablePanel defaultSize={80} minSize={70}>
												<EditorContent
													selectedFile={selectedFile}
													openTabs={openTabs}
													handleEditorChange={handleEditorChange}
													handleEditorMount={handleEditorMount}
													handleSave={handleSave}
												/>
											</ResizablePanel>
										</ResizablePanelGroup>
									</div>
								</div>
							</ResizablePanel>
							<ResizableHandle />
							<ResizablePanel defaultSize={20} minSize={10} maxSize={50}>
								<div className="bg-background text-foreground h-full">
									<LogsPanel projectId={projectId} />
								</div>
							</ResizablePanel>
						</ResizablePanelGroup>
					) : (
						<ResizablePanelGroup direction="vertical">
							<ResizablePanel defaultSize={75} minSize={40}>
								<div className="h-full flex flex-col">
									<Playground projectId={projectId} networkId={chaincodeProject.networkId} />
								</div>
							</ResizablePanel>
							<ResizableHandle />
							<ResizablePanel defaultSize={20} minSize={10} maxSize={50}>
								<div className="bg-background text-foreground h-full">
									<LogsPanel projectId={projectId} />
								</div>
							</ResizablePanel>
						</ResizablePanelGroup>
					)}
				</ResizablePanel>
			</ResizablePanelGroup>
		</div>
	)
}

export { useStreamingChat }
