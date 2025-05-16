import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from 'next-themes'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { client } from './api/client'
import { Header } from './components/dashboard/Header'
import AppSidebar from './components/dashboard/Sidebar'
import { ProtectedLayout } from './components/layout/ProtectedLayout'
import { ThemeWrapper } from './components/theme/ThemeWrapper'
import { SidebarInset, SidebarProvider } from './components/ui/sidebar'
import config from './config'
import { AuthProvider } from './contexts/AuthContext'
import { BreadcrumbProvider } from './contexts/BreadcrumbContext'
import './globals.css'

import SharedNetworksPage from '@/pages/networks/fabric/shared'
import ImportNetworkPage from '@/pages/networks/import'
import CreateBesuNodePage from '@/pages/nodes/besu/create'
import CreateFabricNodePage from '@/pages/nodes/fabric/create'
import EditFabricNodePage from '@/pages/nodes/fabric/edit'
import NodesLogsPage from '@/pages/nodes/logs'
import { Toaster } from './components/ui/sonner'
import CertificateTemplatesPage from './pages/identity/certificates'
import MonitoringPage from './pages/monitoring'
import UpdateProviderPage from './pages/monitoring/providers/[id]'
import CreateProviderPage from './pages/monitoring/providers/new'
import NetworksPage from './pages/networks'
import BesuPage from './pages/networks/besu'
import BesuNetworkDetailPage from './pages/networks/besu-page'
import CreateBesuNetworkPage from './pages/networks/besu/create'
import FabricPage from './pages/networks/fabric'
import FabricNetworkDetailPage from './pages/networks/fabric-page'
import FabricCreateChannel from './pages/networks/fabric/create'
import OrganizationsPage from './pages/networks/fabric/organizations'
import NodesPage from './pages/nodes'
import NodeDetailPage from './pages/nodes/[id]'
import BulkCreateNodesPage from './pages/nodes/fabric/bulk-create'
import NotFoundPage from './pages/not-found'
import OrganizationDetailPage from './pages/organizations/[id]'
import AccessControlPage from './pages/settings/access'
import BackupsPage from './pages/settings/backups'
import SettingsPage from './pages/settings/general'
import KeyManagementPage from './pages/settings/keys'
import KeyDetailPage from './pages/settings/keys/[id]'
import NetworkConfigPage from './pages/settings/network'
import SmartContractsPage from './pages/smart-contracts'
import { BlocksOverview } from '@/components/networks/blocks-overview'
import { BlockDetails } from '@/components/networks/block-details'
import ApiDocumentationPage from './pages/api-documentation'
import BulkCreateBesuNetworkPage from './pages/networks/besu/bulk-create'
import EditBesuNodePage from './pages/nodes/besu/edit'
import CreateNodePage from './pages/nodes/create'
import PluginsPage from './pages/plugins'
import PluginDetailPage from './pages/plugins/[name]'
import NewPluginPage from './pages/plugins/new'
import UsersPage from './pages/users'
import AccountPage from './pages/account'
import AuditLogsPage from '@/pages/settings/audit-logs'
import AuditLogDetailPage from '@/pages/settings/audit-logs/[id]'

const queryClient = new QueryClient({
	defaultOptions: {
		queries: {
			refetchOnWindowFocus: false,
			retry: false,
		},
	},
})

client.setConfig({ baseUrl: config.apiUrl })

const App = () => {
	return (
		<ThemeProvider defaultTheme="system" enableSystem attribute="class">
			<ThemeWrapper>
				<QueryClientProvider client={queryClient}>
					<BrowserRouter>
						<AuthProvider>
							<ProtectedLayout>
								<BreadcrumbProvider>
									<SidebarProvider>
										<AppSidebar />
										<SidebarInset>
											<Header />
											<div className="p-0">
												<Routes>
													<Route path="/">
														<Route path="/" element={<Navigate to="/nodes" replace />} />
														<Route path="account" element={<AccountPage />} />
														<Route path="nodes" element={<NodesPage />} />
														<Route path="smart-contracts" element={<SmartContractsPage />} />
														<Route path="monitoring" element={<MonitoringPage />} />
														<Route path="monitoring/providers/new" element={<CreateProviderPage />} />
														<Route path="monitoring/providers/:id" element={<UpdateProviderPage />} />
														<Route path="networks" element={<NetworksPage />} />
														<Route path="networks/import" element={<ImportNetworkPage />} />
														<Route path="network/fabric" element={<FabricPage />} />
														<Route path="network/besu" element={<BesuPage />} />
														<Route path="settings/access" element={<AccessControlPage />} />
														<Route path="settings/network" element={<NetworkConfigPage />} />
														<Route path="settings/keys" element={<KeyManagementPage />} />
														<Route path="settings/backups" element={<BackupsPage />} />
														<Route path="settings/general" element={<SettingsPage />} />
														<Route path="settings/monitoring" element={<MonitoringPage />} />
														<Route path="identity/certificates" element={<CertificateTemplatesPage />} />
														<Route path="fabric/organizations" element={<OrganizationsPage />} />
														<Route path="nodes/fabric/create" element={<CreateFabricNodePage />} />
														<Route path="nodes/fabric/edit/:id" element={<EditFabricNodePage />} />
														<Route path="nodes/besu/edit/:id" element={<EditBesuNodePage />} />
														<Route path="nodes/:id" element={<NodeDetailPage />} />
														<Route path="networks/fabric/create" element={<FabricCreateChannel />} />
														<Route path="networks/besu/create" element={<CreateBesuNetworkPage />} />
														<Route path="networks/:id/besu" element={<BesuNetworkDetailPage />} />
														<Route path="networks/:id/fabric" element={<FabricNetworkDetailPage />} />
														<Route path="networks/:id/blocks" element={<BlocksOverview />} />
														<Route path="networks/:id/blocks/:blockNumber" element={<BlockDetails />} />
														<Route path="organizations/:id" element={<OrganizationDetailPage />} />
														<Route path="settings/keys/:id" element={<KeyDetailPage />} />
														<Route path="nodes/create" element={<CreateNodePage />} />
														<Route path="nodes/fabric/bulk" element={<BulkCreateNodesPage />} />
														<Route path="nodes/logs" element={<NodesLogsPage />} />
														<Route path="nodes/besu/create" element={<CreateBesuNodePage />} />
														<Route path="networks/fabric/shared" element={<SharedNetworksPage />} />
														<Route path="docs" element={<ApiDocumentationPage />} />
														<Route path="networks/besu/bulk-create" element={<BulkCreateBesuNetworkPage />} />
														<Route path="plugins" element={<PluginsPage />} />
														<Route path="plugins/new" element={<NewPluginPage />} />
														<Route path="plugins/:name" element={<PluginDetailPage />} />
														<Route path="users" element={<UsersPage />} />
														<Route path="settings/audit-logs" element={<AuditLogsPage />} />
														<Route path="settings/audit-logs/:id" element={<AuditLogDetailPage />} />
													</Route>
													<Route path="*" element={<NotFoundPage />} />
												</Routes>
											</div>
										</SidebarInset>
									</SidebarProvider>
								</BreadcrumbProvider>
							</ProtectedLayout>
						</AuthProvider>
					</BrowserRouter>
				</QueryClientProvider>
			</ThemeWrapper>
			<Toaster position="top-center" />
		</ThemeProvider>
	)
}

export default App
