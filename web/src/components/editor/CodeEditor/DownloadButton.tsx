import { useState } from 'react'
import { Download } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'

interface DownloadButtonProps {
	projectId: number
	projectName?: string
}

export function DownloadButton({ projectId, projectName }: DownloadButtonProps) {
	const [isDownloading, setIsDownloading] = useState(false)

	const handleDownload = async () => {
		if (isDownloading) return

		setIsDownloading(true)
		try {
			// Get the base URL from the current window location
			const baseUrl = window.location.origin
			const url = `${baseUrl}/api/v1/chaincode-projects/${projectId}/download`
			
			// Make a direct fetch request to get the zip file
			const response = await fetch(url, {
				method: 'GET',
				credentials: 'include', // Include cookies for authentication
				headers: {
					'Accept': 'application/zip',
				},
			})

			if (!response.ok) {
				throw new Error(`HTTP error! status: ${response.status}`)
			}

			// Get the blob from the response
			const blob = await response.blob()
			
			// Create a download link
			const downloadUrl = window.URL.createObjectURL(blob)
			const link = document.createElement('a')
			link.href = downloadUrl
			link.download = `${projectName || 'project'}-${projectId}.zip`
			
			// Trigger the download
			document.body.appendChild(link)
			link.click()
			
			// Cleanup
			document.body.removeChild(link)
			window.URL.revokeObjectURL(downloadUrl)

			toast.success('Project downloaded successfully', {
				description: 'Your project has been downloaded as a ZIP file.',
			})
		} catch (error) {
			console.error('Download error:', error)
			toast.error('Failed to download project', {
				description: 'There was an error downloading your project. Please try again.',
			})
		} finally {
			setIsDownloading(false)
		}
	}

	return (
		<Button
			variant="outline"
			size="sm"
			onClick={handleDownload}
			disabled={isDownloading}
			className="flex items-center gap-2"
		>
			<Download className="w-4 h-4" />
			{isDownloading ? 'Downloading...' : 'Download Project'}
		</Button>
	)
} 