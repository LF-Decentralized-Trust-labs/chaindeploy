import MonacoEditor from '@monaco-editor/react'
import type { EditorContentProps } from './types'
import { getMonacoLanguage } from './types'

export function EditorContent({ selectedFile, openTabs, handleEditorChange, handleEditorMount }: EditorContentProps) {
	return (
		<div className="h-full w-full flex-1 p-0 font-mono text-lg">
			{selectedFile ? (
				<MonacoEditor
					key={selectedFile?.name}
					
					theme="vs-dark"
					language={getMonacoLanguage(selectedFile.name)}
					value={openTabs.find((tab) => tab.name === selectedFile?.name)?.content || selectedFile?.content}
					onChange={handleEditorChange}
					onMount={handleEditorMount}
					options={{
						theme: 'vs',
						fontSize: 18,
						minimap: { enabled: false },
						scrollbar: {
							alwaysConsumeMouseWheel: true,
							horizontal: 'visible',
							vertical: 'visible',
							verticalScrollbarSize: 10,
							horizontalScrollbarSize: 10,
							arrowSize: 10,
						},
						scrollBeyondLastLine: false,
						automaticLayout: true,
						domReadOnly: false,
						readOnly: false,
						contextmenu: true,
						wordWrap: 'on',
						lineNumbers: 'on',
						glyphMargin: true,
						folding: true,
						lineDecorationsWidth: 10,
						lineNumbersMinChars: 3,
					}}
				/>
			) : (
				<div className="text-muted-foreground p-8 text-xl">Select a file to edit</div>
			)}
		</div>
	)
} 