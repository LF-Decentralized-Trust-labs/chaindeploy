import { useEffect } from 'react'

export function usePageTitle(title: string) {
	useEffect(() => {
		const previousTitle = document.title
		document.title = `${title} | ChainLaunch`
		
		return () => {
			document.title = previousTitle
		}
	}, [title])
}
