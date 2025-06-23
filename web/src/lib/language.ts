// Determine language for syntax highlighting based on file extension
export const getLanguage = (filePath: string) => {
	const ext = filePath.split('.').pop()?.toLowerCase()
	console.log('ext', ext)
	switch (ext) {
		case 'js': return 'javascript'
		case 'ts': return 'typescript'
		case 'jsx': return 'javascript'
		case 'tsx': return 'typescript'
		case 'json': return 'json'
		case 'html': return 'html'
		case 'css': return 'css'
		case 'md': return 'markdown'
		case 'py': return 'python'
		case 'go': return 'go'
		case 'rs': return 'rust'
		case 'java': return 'java'
		case 'c': return 'c'
		case 'cpp': return 'cpp'
		case 'h': return 'cpp'
		case 'hpp': return 'cpp'
		case 'sql': return 'sql'
		case 'sh': return 'bash'
		case 'yml': return 'yaml'
		case 'yaml': return 'yaml'
		case 'xml': return 'xml'
		case 'txt': return 'plaintext'
		case 'go': return 'go'
		default: return 'plaintext'
	}
} 