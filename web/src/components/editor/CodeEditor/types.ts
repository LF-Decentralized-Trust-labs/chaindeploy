import type { editor } from 'monaco-editor'
import { FaCss3Alt, FaFile, FaFileAlt, FaFileCode, FaHtml5, FaJs } from 'react-icons/fa'

export interface File {
	name: string
	path: string
	content: string
	language: string
}

export interface FilesDirectoryTreeNode {
	children?: FilesDirectoryTreeNode[]
	isDir?: boolean
	name?: string
	path?: string
}

// Define file icons as a type-safe record
export const fileIcons = {
	js: { icon: FaJs, className: 'inline text-yellow-400 text-xl' },
	ts: { icon: FaJs, className: 'inline text-blue-400 text-xl' },
	tsx: { icon: FaJs, className: 'inline text-blue-400 text-xl' },
	css: { icon: FaCss3Alt, className: 'inline text-blue-500 text-xl' },
	html: { icon: FaHtml5, className: 'inline text-orange-500 text-xl' },
	json: { icon: FaFileCode, className: 'inline text-amber-400 text-xl' },
	md: { icon: FaFileAlt, className: 'inline text-gray-400 text-xl' },
	default: { icon: FaFile, className: 'inline text-gray-500 text-xl' },
} as const

// Helper to map file extensions to Monaco language IDs
export const extensionToMonacoLanguage = {
	js: 'javascript',
	ts: 'typescript',
	tsx: 'typescript',
	css: 'css',
	html: 'html',
	json: 'json',
	md: 'markdown',
	go: 'go',
	dockerfile: 'dockerfile',
	py: 'python',
	java: 'java',
	cpp: 'cpp',
	c: 'c',
	cs: 'csharp',
	php: 'php',
	rb: 'ruby',
	rs: 'rust',
	swift: 'swift',
	kt: 'kotlin',
	scala: 'scala',
	sh: 'shell',
	ps1: 'powershell',
	yml: 'yaml',
	yaml: 'yaml',
	toml: 'toml',
	ini: 'ini',
	xml: 'xml',
	sql: 'sql',
	r: 'r',
	scss: 'scss',
	sass: 'sass',
	less: 'less',
	vue: 'vue',
	svelte: 'svelte',
	jsx: 'javascript',
	default: 'plaintext',
} as const

export function getMonacoLanguage(filename: string): string {
	const ext = filename.split('.').pop()?.toLowerCase()
	return (ext && extensionToMonacoLanguage[ext as keyof typeof extensionToMonacoLanguage]) || extensionToMonacoLanguage.default
}

export function getFileIcon(filename: string): { icon: typeof FaFile; className: string } {
	const ext = filename.split('.').pop()?.toLowerCase()
	return (ext && fileIcons[ext as keyof typeof fileIcons]) || fileIcons.default
}

export interface FileTreeProps {
	projectId: number
	node: FilesDirectoryTreeNode
	openFolders: Record<string, boolean>
	setOpenFolders: React.Dispatch<React.SetStateAction<Record<string, boolean>>>
	selectedFile: File | null
	handleFileClick: (file: { name: string; path: string }) => void
	refetchTree: () => void
	isRoot?: boolean
}

export interface LogsPanelProps {
	projectId: number
}

export interface EditorTabsProps {
	openTabs: File[]
	selectedFile: File | null
	handleTabClick: (file: File) => void
	handleTabClose: (file: File, e: React.MouseEvent) => void
	dirtyFiles: string[]
	projectId: number
	projectName?: string
}

export interface EditorContentProps {
	selectedFile: File | null
	openTabs: File[]
	handleEditorChange: (value: string | undefined) => void
	handleEditorMount: (editor: editor.IStandaloneCodeEditor) => void
	handleSave: () => void
}
