import { HttpNodeResponse } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { TimeAgo } from '@/components/ui/time-ago'
import { CertificateViewer } from '@/components/ui/certificate-viewer'
import { FabricNodeChannels } from '@/components/nodes/FabricNodeChannels'
import { FabricOrdererConfig } from '@/components/nodes/FabricOrdererConfig'
import { FabricPeerConfig } from '@/components/nodes/FabricPeerConfig'
import { LogViewer } from '@/components/nodes/LogViewer'
import OrdererMetricsPage from '@/pages/metrics/orderer/[nodeId]'
import PeerMetricsPage from '@/pages/metrics/peer/[nodeId]'
import { Shield, Network, Key, Server, Database, Globe } from 'lucide-react'

interface FabricNodeDetailsProps {
  node: HttpNodeResponse
  logs: string
  events: any
  activeTab: string
  onTabChange: (value: string) => void
}

export function FabricNodeDetails({ node, logs, events, activeTab, onTabChange }: FabricNodeDetailsProps) {
  const isPeer = node.fabricPeer !== undefined
  const isOrderer = node.fabricOrderer !== undefined
  const nodeConfig = isPeer ? node.fabricPeer : node.fabricOrderer

  return (
    <div className="space-y-6">
      {/* Fabric-specific header */}
      <div className="grid gap-6 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center gap-2">
              <Server className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">Node Type</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <Badge variant="secondary" className="font-mono">
                {isPeer ? 'PEER' : 'ORDERER'}
              </Badge>
              <span className="text-sm text-muted-foreground">
                {nodeConfig?.mode || 'Standard'}
              </span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center gap-2">
              <Network className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">Organization</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            <div className="space-y-1">
              <p className="font-medium">{nodeConfig?.mspId || 'N/A'}</p>
              <p className="text-xs text-muted-foreground">MSP ID</p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center gap-2">
              <Shield className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">Security</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <Badge variant="outline" className="text-xs">
                TLS Enabled
              </Badge>
              <Badge variant="outline" className="text-xs">
                mTLS
              </Badge>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Configuration Cards */}
      <div className="grid gap-6 md:grid-cols-2">
        {isPeer && <FabricPeerConfig config={node.fabricPeer!} />}
        {isOrderer && <FabricOrdererConfig config={node.fabricOrderer!} />}
        
        {/* Endpoints Card */}
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Globe className="h-4 w-4 text-muted-foreground" />
              <CardTitle>Service Endpoints</CardTitle>
            </div>
            <CardDescription>Network addresses and ports</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div>
              <p className="text-sm font-medium text-muted-foreground">Listen Address</p>
              <p className="font-mono text-sm">{nodeConfig?.listenAddress || 'N/A'}</p>
            </div>
            {nodeConfig?.externalEndpoint && (
              <div>
                <p className="text-sm font-medium text-muted-foreground">External Endpoint</p>
                <p className="font-mono text-sm">{nodeConfig.externalEndpoint}</p>
              </div>
            )}
            {isPeer && node.fabricPeer?.chaincodeAddress && (
              <div>
                <p className="text-sm font-medium text-muted-foreground">Chaincode Address</p>
                <p className="font-mono text-sm">{node.fabricPeer.chaincodeAddress}</p>
              </div>
            )}
            {nodeConfig?.operationsListenAddress && (
              <div>
                <p className="text-sm font-medium text-muted-foreground">Operations Address</p>
                <p className="font-mono text-sm">{nodeConfig.operationsListenAddress}</p>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={onTabChange} className="space-y-4">
        <TabsList className="grid w-full grid-cols-5">
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="metrics">Metrics</TabsTrigger>
          <TabsTrigger value="crypto">Certificates</TabsTrigger>
          <TabsTrigger value="events">Events</TabsTrigger>
          <TabsTrigger value="channels">Channels</TabsTrigger>
        </TabsList>

        <TabsContent value="logs" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Container Logs</CardTitle>
              <CardDescription>Real-time logs from the Fabric node</CardDescription>
            </CardHeader>
            <CardContent>
              <LogViewer logs={logs} onScroll={() => {}} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="metrics" className="space-y-4">
          {isOrderer && <OrdererMetricsPage node={node} />}
          {isPeer && <PeerMetricsPage node={node} />}
        </TabsContent>

        <TabsContent value="crypto" className="space-y-4">
          <div className="grid gap-4">
            <Card>
              <CardHeader>
                <div className="flex items-center gap-2">
                  <Key className="h-4 w-4 text-muted-foreground" />
                  <CardTitle>Identity Certificates</CardTitle>
                </div>
                <CardDescription>MSP signing certificates for transaction endorsement</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <CertificateViewer 
                  label="Signing Certificate" 
                  certificate={nodeConfig?.signCert || ''} 
                />
                <CertificateViewer 
                  label="CA Certificate" 
                  certificate={nodeConfig?.signCaCert || ''} 
                />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <div className="flex items-center gap-2">
                  <Shield className="h-4 w-4 text-muted-foreground" />
                  <CardTitle>TLS Certificates</CardTitle>
                </div>
                <CardDescription>Transport layer security certificates</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <CertificateViewer 
                  label="TLS Certificate" 
                  certificate={nodeConfig?.tlsCert || ''} 
                />
                <CertificateViewer 
                  label="TLS CA Certificate" 
                  certificate={nodeConfig?.tlsCaCert || ''} 
                />
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="events">
          <Card>
            <CardHeader>
              <CardTitle>Event History</CardTitle>
              <CardDescription>Lifecycle events and operations</CardDescription>
            </CardHeader>
            <CardContent>
              {/* Events content will be passed from parent */}
              {events}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="channels">
          <FabricNodeChannels nodeId={node.id!} />
        </TabsContent>
      </Tabs>
    </div>
  )
}