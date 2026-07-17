import { defineConfig } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'
import { pluginSvgr } from '@rsbuild/plugin-svgr'
const apiUrl = process.env.PUBLIC_API_URL
if (!apiUrl) {
	throw new Error('PUBLIC_API_URL is not set')
}
export default defineConfig({
	plugins: [pluginReact(), pluginSvgr()],
	html: {
		title: 'ChainLaunch',
		favicon: './public/favicon.svg',
	},
	server: {
		proxy: {
			'/api': {
				ws: true,
				target: apiUrl,
				changeOrigin: true,
				secure: false,
			},
		},
	},
})
