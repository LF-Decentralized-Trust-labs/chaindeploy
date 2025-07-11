import { cn } from '@/lib/utils'
import type { EditorTabsProps } from './types'
import { getFileIcon } from './types'
import { DownloadButton } from './DownloadButton'

export function EditorTabs({ openTabs, selectedFile, handleTabClick, handleTabClose, dirtyFiles, projectId, projectName }: EditorTabsProps) {
	return (
		<div className="h-12 bg-muted border-b border-border flex items-center px-3 overflow-x-auto gap-2">
			<div className="flex items-center gap-2 flex-shrink-0">
				{openTabs.length === 0 ? (
					<div className="text-muted-foreground text-base px-4">No files open</div>
				) : (
					openTabs.map((file) => (
						<div
							key={file.name}
							onClick={() => handleTabClick(file)}
							className={cn(
								'flex items-center px-6 py-2 mr-2 rounded-t cursor-pointer transition-colors text-lg',
								selectedFile?.name === file.name ? 'bg-background text-foreground' : 'bg-muted text-muted-foreground hover:bg-accent hover:text-accent-foreground'
							)}
						>
							{(() => { const { icon: Icon, className } = getFileIcon(file.name); return <Icon className={className} /> })()}
							<span className="ml-3 mr-3 flex items-center gap-2">
								{file.name}
								{dirtyFiles.includes(file.name) && (
									<span
										className="inline-block align-middle ml-1 bg-primary"
										style={{ width: '12px', height: '12px', borderRadius: '50%', display: 'inline-block' }}
										title="Unsaved changes"
									/>
								)}
							</span>
							<button onClick={(e) => handleTabClose(file, e)} className="ml-2 text-muted-foreground hover:text-foreground focus:outline-none text-2xl" title="Close tab">
								&times;
							</button>
						</div>
					))
				)}
			</div>
			<div className="flex-shrink-0 ml-auto">
				<DownloadButton projectId={projectId} projectName={projectName} />
			</div>
		</div>
	)
} 