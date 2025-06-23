import { defineConfig } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'
import { pluginSvgr } from '@rsbuild/plugin-svgr'

export default defineConfig({
	plugins: [pluginReact(), pluginSvgr()],
	html: {
		title: 'ChainLaunch',
		favicon: './public/favicon.png',
	},
	performance: {
		bundleAnalyze: {
			openAnalyzer: true,
		},
	},
})
