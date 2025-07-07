import { useMemo, useState } from 'react'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger } from '@/components/ui/context-menu'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { FaChevronDown, FaChevronRight, FaFolder, FaFolderOpen } from 'react-icons/fa'
import { cn } from '@/lib/utils'
import { postProjectsByProjectIdDirsCreate, deleteProjectsByProjectIdFilesDelete, deleteProjectsByProjectIdDirsDelete, postProjectsByProjectIdFilesWrite } from '@/api/client'
import type { FileTreeProps } from './types'
import { getFileIcon } from './types'

export function FileTree({
	projectId,
	node,
	openFolders,
	setOpenFolders,
	selectedFile,
	handleFileClick,
	refetchTree,
	isRoot = false,
}: FileTreeProps) {
	const [creatingNode, setCreatingNode] = useState<{ path: string | null; type: 'file' | 'folder' | null }>({ path: null, type: null })
	const [newName, setNewName] = useState('')
	const [dialogOpen, setDialogOpen] = useState(false)
	const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
	const [deleteType, setDeleteType] = useState<'file' | 'folder' | null>(null)
	const [deletePath, setDeletePath] = useState<string | null>(null)

	// Sort children: folders first (case-insensitive), then files (case-insensitive)
	const sortedChildren = useMemo(() => node?.children?.sort((a, b) => {
		// Folders first, then files
		if (a.isDir && !b.isDir) return -1
		if (!a.isDir && b.isDir) return 1
		// If both are same type, sort alphabetically
		return (a.name || '').toLowerCase().localeCompare((b.name || '').toLowerCase())
	}), [node?.children])

	const startCreate = (type: 'file' | 'folder', path: string | null) => {
		setCreatingNode({ path, type })
		setNewName('')
		setDialogOpen(true)
	}

	const handleCreate = async (e: React.FormEvent) => {
		e.preventDefault()
		if (!creatingNode.type || !newName) return
		try {
			if (creatingNode.type === 'file') {
				await postProjectsByProjectIdFilesWrite({
					path: { projectId },
					body: { path: (creatingNode.path ? creatingNode.path + '/' : '') + newName, content: '' },
				})
			} else {
				await postProjectsByProjectIdDirsCreate({
					path: { projectId },
					body: { dir: (creatingNode.path ? creatingNode.path + '/' : '') + newName },
				})
			}
		} catch (err) {
			// Optionally show a toast here
		} finally {
			setDialogOpen(false)
			setCreatingNode({ path: null, type: null })
			setNewName('')
			refetchTree()
		}
	}

	const handleDelete = async () => {
		if (!deleteType || !deletePath) return
		try {
			if (deleteType === 'file') {
				await deleteProjectsByProjectIdFilesDelete({
					path: { projectId },
					query: { path: deletePath },
				})
			} else {
				await deleteProjectsByProjectIdDirsDelete({
					path: { projectId },
					query: { project: projectId.toString(), dir: deletePath },
				})
			}
		} catch (error) {
			// Optionally show a toast here
		} finally {
			setDeleteDialogOpen(false)
			setDeleteType(null)
			setDeletePath(null)
			refetchTree()
		}
	}

	const startDelete = (type: 'file' | 'folder', path: string) => {
		setDeleteType(type)
		setDeletePath(path)
		setDeleteDialogOpen(true)
	}

	if (!node) return null

	if (isRoot) {
		// Render only children, not the root node itself
		return (
			<div className="overflow-auto">
				<div className="ml-2">
					{node.children?.map((child) => (
						<FileTree
							key={child.path || child.name}
							projectId={projectId}
							node={child}
							openFolders={openFolders}
							setOpenFolders={setOpenFolders}
							selectedFile={selectedFile}
							handleFileClick={handleFileClick}
							refetchTree={refetchTree}
						/>
					))}
				</div>
			</div>
		)
	}

	const isOpen = openFolders[node.path || node.name || ''] || false
	const isDir = node.isDir
	const hasChildren = node.children && node.children.length > 0

	return (
		<div className="ml-2">
			<ContextMenu>
				<ContextMenuTrigger asChild>
					<div>
						{isDir ? (
							<div>
								<button
									className="flex items-center gap-2 text-left py-1 px-2 w-full hover:bg-muted rounded"
									onClick={() => setOpenFolders((f) => ({ ...f, [node.path || node.name || '']: !isOpen }))}
								>
									{isOpen ? <FaChevronDown className="text-xs" /> : <FaChevronRight className="text-xs" />}
									{isOpen ? <FaFolderOpen className="text-yellow-400" /> : <FaFolder className="text-yellow-400" />}
									<span className="ml-1 font-semibold text-sm">{node.name}</span>
								</button>
								{isOpen && hasChildren && (
									<div className="ml-4">
										{sortedChildren?.map((child) => (
											<FileTree
												key={child.path || child.name}
												projectId={projectId}
												node={child}
												openFolders={openFolders}
												setOpenFolders={setOpenFolders}
												selectedFile={selectedFile}
												handleFileClick={handleFileClick}
												refetchTree={refetchTree}
											/>
										))}
									</div>
								)}
							</div>
						) : (
							<button
								className={cn('flex items-center gap-2 text-left py-1 px-2 w-full hover:bg-muted rounded ', selectedFile?.name === node.name && 'bg-accent')}
								onClick={() => handleFileClick({ name: node.name!, path: node.path! })}
							>
								{(() => { const { icon: Icon, className } = getFileIcon(node.name!); return <Icon className={className} /> })()}
								<span className="ml-1 text-sm flex-1 truncate overflow-hidden whitespace-nowrap">{node.name}</span>
							</button>
						)}
					</div>
				</ContextMenuTrigger>
				{isDir ? (
					<ContextMenuContent className="bg-popover/100 text-popover-foreground border border-border rounded-md shadow-md py-2 px-0 min-w-[160px]">
						<ContextMenuItem className="hover:bg-accent hover:text-accent-foreground px-4 py-2 cursor-pointer" onClick={() => startCreate('file', node.path)}>
							New File
						</ContextMenuItem>
						<ContextMenuItem className="hover:bg-accent hover:text-accent-foreground px-4 py-2 cursor-pointer" onClick={() => startCreate('folder', node.path)}>
							New Folder
						</ContextMenuItem>
						<ContextMenuItem className="hover:bg-destructive hover:text-destructive-foreground px-4 py-2 cursor-pointer text-destructive" onClick={() => startDelete('folder', node.path!)}>
							Delete Folder
						</ContextMenuItem>
					</ContextMenuContent>
				) : (
					<ContextMenuContent className="bg-popover/100 text-popover-foreground border border-border rounded-md shadow-md py-2 px-0 min-w-[160px]">
						<ContextMenuItem className="hover:bg-destructive hover:text-destructive-foreground px-4 py-2 cursor-pointer text-destructive" onClick={() => startDelete('file', node.path!)}>
							Delete File
						</ContextMenuItem>
					</ContextMenuContent>
				)}
			</ContextMenu>

			{/* Dialog for creating file/folder */}
			<Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
				<DialogContent className="bg-popover/100 text-popover-foreground">
					<DialogHeader>
						<DialogTitle>Create {creatingNode.type === 'file' ? 'File' : 'Folder'}</DialogTitle>
					</DialogHeader>
					<form onSubmit={handleCreate}>
						<Input value={newName} onChange={(e) => setNewName(e.target.value)} autoFocus placeholder={`New ${creatingNode.type} name`} />
						<DialogFooter className="mt-4">
							<Button type="submit">Create</Button>
							<Button
								type="button"
								variant="outline"
								onClick={() => {
									setDialogOpen(false)
									setCreatingNode({ path: null, type: null })
								}}
							>
								Cancel
							</Button>
						</DialogFooter>
					</form>
				</DialogContent>
			</Dialog>

			{/* Dialog for delete confirmation */}
			<Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
				<DialogContent className="bg-popover/100 text-popover-foreground">
					<DialogHeader>
						<DialogTitle>Delete {deleteType === 'file' ? 'File' : 'Folder'}</DialogTitle>
					</DialogHeader>
					<div className="py-4">
						<p>Are you sure you want to delete this {deleteType}? This action cannot be undone.</p>
					</div>
					<DialogFooter>
						<Button variant="destructive" onClick={handleDelete}>
							Delete
						</Button>
						<Button
							variant="outline"
							onClick={() => {
								setDeleteDialogOpen(false)
								setDeleteType(null)
								setDeletePath(null)
							}}
						>
							Cancel
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	)
} 