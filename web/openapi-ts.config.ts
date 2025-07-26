export default {
	client: '@hey-api/client-fetch',
	input: 'http://localhost:8100/swagger/doc.json',
	output: 'src/api/client',
	plugins: [
		'@tanstack/react-query',
		{
			name: '@hey-api/sdk',
			operationId: false,
		},
	],
	config: {
		operationId: false,
	},
	services: {
		operationId: false,
	},
}
