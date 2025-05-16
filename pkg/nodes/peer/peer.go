package peer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	// add sprig/v3
	"github.com/Masterminds/sprig/v3"
	"github.com/hyperledger/fabric-admin-sdk/pkg/channel"
	"github.com/hyperledger/fabric-admin-sdk/pkg/identity"
	"github.com/hyperledger/fabric-admin-sdk/pkg/network"
	"github.com/hyperledger/fabric-config/protolator"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	gwidentity "github.com/hyperledger/fabric-gateway/pkg/identity"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"io"

	"github.com/chainlaunch/chainlaunch/internal/protoutil"
	"github.com/chainlaunch/chainlaunch/pkg/binaries"
	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	fabricservice "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	kmodels "github.com/chainlaunch/chainlaunch/pkg/keymanagement/models"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	settingsservice "github.com/chainlaunch/chainlaunch/pkg/settings/service"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type AddressOverridePath struct {
	From      string
	To        string
	TLSCAPath string
}

const coreYamlTemplate = `
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

###############################################################################
#
#    Peer section
#
###############################################################################
peer:

  # The peer id provides a name for this peer instance and is used when
  # naming docker resources.
  id: jdoe

  # The networkId allows for logical separation of networks and is used when
  # naming docker resources.
  networkId: dev

  # The Address at local network interface this Peer will listen on.
  # By default, it will listen on all network interfaces
  listenAddress: 0.0.0.0:7051

  # The endpoint this peer uses to listen for inbound chaincode connections.
  # If this is commented-out, the listen address is selected to be
  # the peer's address (see below) with port 7052
  # chaincodeListenAddress: 0.0.0.0:7052

  # The endpoint the chaincode for this peer uses to connect to the peer.
  # If this is not specified, the chaincodeListenAddress address is selected.
  # And if chaincodeListenAddress is not specified, address is selected from
  # peer address (see below). If specified peer address is invalid then it
  # will fallback to the auto detected IP (local IP) regardless of the peer
  # addressAutoDetect value.
  # chaincodeAddress: 0.0.0.0:7052

  # When used as peer config, this represents the endpoint to other peers
  # in the same organization. For peers in other organization, see
  # gossip.externalEndpoint for more info.
  # When used as CLI config, this means the peer's endpoint to interact with
  address: 0.0.0.0:7051

  # Whether the Peer should programmatically determine its address
  # This case is useful for docker containers.
  # When set to true, will override peer address.
  addressAutoDetect: false

  # Keepalive settings for peer server and clients
  keepalive:
    # Interval is the duration after which if the server does not see
    # any activity from the client it pings the client to see if it's alive
    interval: 7200s
    # Timeout is the duration the server waits for a response
    # from the client after sending a ping before closing the connection
    timeout: 20s
    # MinInterval is the minimum permitted time between client pings.
    # If clients send pings more frequently, the peer server will
    # disconnect them
    minInterval: 60s
    # Client keepalive settings for communicating with other peer nodes
    client:
      # Interval is the time between pings to peer nodes.  This must
      # greater than or equal to the minInterval specified by peer
      # nodes
      interval: 60s
      # Timeout is the duration the client waits for a response from
      # peer nodes before closing the connection
      timeout: 20s
    # DeliveryClient keepalive settings for communication with ordering
    # nodes.
    deliveryClient:
      # Interval is the time between pings to ordering nodes.  This must
      # greater than or equal to the minInterval specified by ordering
      # nodes.
      interval: 60s
      # Timeout is the duration the client waits for a response from
      # ordering nodes before closing the connection
      timeout: 20s


  # Gossip related configuration
  gossip:
    # Bootstrap set to initialize gossip with.
    # This is a list of other peers that this peer reaches out to at startup.
    # Important: The endpoints here have to be endpoints of peers in the same
    # organization, because the peer would refuse connecting to these endpoints
    # unless they are in the same organization as the peer.
    bootstrap: 127.0.0.1:7051

    # NOTE: orgLeader and useLeaderElection parameters are mutual exclusive.
    # Setting both to true would result in the termination of the peer
    # since this is undefined state. If the peers are configured with
    # useLeaderElection=false, make sure there is at least 1 peer in the
    # organization that its orgLeader is set to true.

    # Defines whenever peer will initialize dynamic algorithm for
    # "leader" selection, where leader is the peer to establish
    # connection with ordering service and use delivery protocol
    # to pull ledger blocks from ordering service.
    useLeaderElection: false
    # Statically defines peer to be an organization "leader",
    # where this means that current peer will maintain connection
    # with ordering service and disseminate block across peers in
    # its own organization. Multiple peers or all peers in an organization
    # may be configured as org leaders, so that they all pull
    # blocks directly from ordering service.
    orgLeader: true

    # Interval for membershipTracker polling
    membershipTrackerInterval: 5s

    # Overrides the endpoint that the peer publishes to peers
    # in its organization. For peers in foreign organizations
    # see 'externalEndpoint'
    endpoint:
    # Maximum count of blocks stored in memory
    maxBlockCountToStore: 10
    # Max time between consecutive message pushes(unit: millisecond)
    maxPropagationBurstLatency: 10ms
    # Max number of messages stored until a push is triggered to remote peers
    maxPropagationBurstSize: 10
    # Number of times a message is pushed to remote peers
    propagateIterations: 1
    # Number of peers selected to push messages to
    propagatePeerNum: 3
    # Determines frequency of pull phases(unit: second)
    # Must be greater than digestWaitTime + responseWaitTime
    pullInterval: 4s
    # Number of peers to pull from
    pullPeerNum: 3
    # Determines frequency of pulling state info messages from peers(unit: second)
    requestStateInfoInterval: 4s
    # Determines frequency of pushing state info messages to peers(unit: second)
    publishStateInfoInterval: 4s
    # Maximum time a stateInfo message is kept until expired
    stateInfoRetentionInterval:
    # Time from startup certificates are included in Alive messages(unit: second)
    publishCertPeriod: 10s
    # Should we skip verifying block messages or not (currently not in use)
    skipBlockVerification: false
    # Dial timeout(unit: second)
    dialTimeout: 3s
    # Connection timeout(unit: second)
    connTimeout: 2s
    # Buffer size of received messages
    recvBuffSize: 20
    # Buffer size of sending messages
    sendBuffSize: 200
    # Time to wait before pull engine processes incoming digests (unit: second)
    # Should be slightly smaller than requestWaitTime
    digestWaitTime: 1s
    # Time to wait before pull engine removes incoming nonce (unit: milliseconds)
    # Should be slightly bigger than digestWaitTime
    requestWaitTime: 1500ms
    # Time to wait before pull engine ends pull (unit: second)
    responseWaitTime: 2s
    # Alive check interval(unit: second)
    aliveTimeInterval: 5s
    # Alive expiration timeout(unit: second)
    aliveExpirationTimeout: 25s
    # Reconnect interval(unit: second)
    reconnectInterval: 25s
    # Max number of attempts to connect to a peer
    maxConnectionAttempts: 120
    # Message expiration factor for alive messages
    msgExpirationFactor: 20
    # This is an endpoint that is published to peers outside of the organization.
    # If this isn't set, the peer will not be known to other organizations.
    externalEndpoint:
    # Leader election service configuration
    election:
      # Longest time peer waits for stable membership during leader election startup (unit: second)
      startupGracePeriod: 15s
      # Interval gossip membership samples to check its stability (unit: second)
      membershipSampleInterval: 1s
      # Time passes since last declaration message before peer decides to perform leader election (unit: second)
      leaderAliveThreshold: 10s
      # Time between peer sends propose message and declares itself as a leader (sends declaration message) (unit: second)
      leaderElectionDuration: 5s

    pvtData:
      # pullRetryThreshold determines the maximum duration of time private data corresponding for a given block
      # would be attempted to be pulled from peers until the block would be committed without the private data
      pullRetryThreshold: 60s
      # As private data enters the transient store, it is associated with the peer's ledger's height at that time.
      # transientstoreMaxBlockRetention defines the maximum difference between the current ledger's height upon commit,
      # and the private data residing inside the transient store that is guaranteed not to be purged.
      # Private data is purged from the transient store when blocks with sequences that are multiples
      # of transientstoreMaxBlockRetention are committed.
      transientstoreMaxBlockRetention: 1000
      # pushAckTimeout is the maximum time to wait for an acknowledgement from each peer
      # at private data push at endorsement time.
      pushAckTimeout: 3s
      # Block to live pulling margin, used as a buffer
      # to prevent peer from trying to pull private data
      # from peers that is soon to be purged in next N blocks.
      # This helps a newly joined peer catch up to current
      # blockchain height quicker.
      btlPullMargin: 10
      # the process of reconciliation is done in an endless loop, while in each iteration reconciler tries to
      # pull from the other peers the most recent missing blocks with a maximum batch size limitation.
      # reconcileBatchSize determines the maximum batch size of missing private data that will be reconciled in a
      # single iteration.
      reconcileBatchSize: 10
      # reconcileSleepInterval determines the time reconciler sleeps from end of an iteration until the beginning
      # of the next reconciliation iteration.
      reconcileSleepInterval: 1m
      # reconciliationEnabled is a flag that indicates whether private data reconciliation is enable or not.
      reconciliationEnabled: true
      # skipPullingInvalidTransactionsDuringCommit is a flag that indicates whether pulling of invalid
      # transaction's private data from other peers need to be skipped during the commit time and pulled
      # only through reconciler.
      skipPullingInvalidTransactionsDuringCommit: false
      # implicitCollectionDisseminationPolicy specifies the dissemination  policy for the peer's own implicit collection.
      # When a peer endorses a proposal that writes to its own implicit collection, below values override the default values
      # for disseminating private data.
      # Note that it is applicable to all channels the peer has joined. The implication is that requiredPeerCount has to
      # be smaller than the number of peers in a channel that has the lowest numbers of peers from the organization.
      implicitCollectionDisseminationPolicy:
        # requiredPeerCount defines the minimum number of eligible peers to which the peer must successfully
        # disseminate private data for its own implicit collection during endorsement. Default value is 0.
        requiredPeerCount: 0
        # maxPeerCount defines the maximum number of eligible peers to which the peer will attempt to
        # disseminate private data for its own implicit collection during endorsement. Default value is 1.
        maxPeerCount: 1

    # Gossip state transfer related configuration
    state:
      # indicates whenever state transfer is enabled or not
      # default value is true, i.e. state transfer is active
      # and takes care to sync up missing blocks allowing
      # lagging peer to catch up to speed with rest network
      enabled: false
      # checkInterval interval to check whether peer is lagging behind enough to
      # request blocks via state transfer from another peer.
      checkInterval: 10s
      # responseTimeout amount of time to wait for state transfer response from
      # other peers
      responseTimeout: 3s
      # batchSize the number of blocks to request via state transfer from another peer
      batchSize: 10
      # blockBufferSize reflects the size of the re-ordering buffer
      # which captures blocks and takes care to deliver them in order
      # down to the ledger layer. The actual buffer size is bounded between
      # 0 and 2*blockBufferSize, each channel maintains its own buffer
      blockBufferSize: 20
      # maxRetries maximum number of re-tries to ask
      # for single state transfer request
      maxRetries: 3

  # TLS Settings
  tls:
    # Require server-side TLS
    enabled:  false
    # Require client certificates / mutual TLS.
    # Note that clients that are not configured to use a certificate will
    # fail to connect to the peer.
    clientAuthRequired: false
    # X.509 certificate used for TLS server
    cert:
      file: tls/server.crt
    # Private key used for TLS server (and client if clientAuthEnabled
    # is set to true
    key:
      file: tls/server.key
    # Trusted root certificate chain for tls.cert
    rootcert:
      file: tls/ca.crt
    # Set of root certificate authorities used to verify client certificates
    clientRootCAs:
      files:
        - tls/ca.crt
    # Private key used for TLS when making client connections.  If
    # not set, peer.tls.key.file will be used instead
    clientKey:
      file:
    # X.509 certificate used for TLS when making client connections.
    # If not set, peer.tls.cert.file will be used instead
    clientCert:
      file:

  # Authentication contains configuration parameters related to authenticating
  # client messages
  authentication:
    # the acceptable difference between the current server time and the
    # client's time as specified in a client request message
    timewindow: 15m

  # Path on the file system where peer will store data (eg ledger). This
  # location must be access control protected to prevent unintended
  # modification that might corrupt the peer operations.
  fileSystemPath: {{.DataPath}}

  # BCCSP (Blockchain crypto provider): Select which crypto implementation or
  # library to use
  BCCSP:
    Default: SW
    # Settings for the SW crypto provider (i.e. when DEFAULT: SW)
    SW:
      # TODO: The default Hash and Security level needs refactoring to be
      # fully configurable. Changing these defaults requires coordination
      # SHA2 is hardcoded in several places, not only BCCSP
      Hash: SHA2
      Security: 256
      # Location of Key Store
      FileKeyStore:
        # If "", defaults to 'mspConfigPath'/keystore
        KeyStore:
    # Settings for the PKCS#11 crypto provider (i.e. when DEFAULT: PKCS11)
    PKCS11:
      # Location of the PKCS11 module library
      Library:
      # Token Label
      Label:
      # User PIN
      Pin:
      Hash:
      Security:

  # Path on the file system where peer will find MSP local configurations
  mspConfigPath: msp

  # Identifier of the local MSP
  # ----!!!!IMPORTANT!!!-!!!IMPORTANT!!!-!!!IMPORTANT!!!!----
  # Deployers need to change the value of the localMspId string.
  # In particular, the name of the local MSP ID of a peer needs
  # to match the name of one of the MSPs in each of the channel
  # that this peer is a member of. Otherwise this peer's messages
  # will not be identified as valid by other nodes.
  localMspId: SampleOrg

  # CLI common client config options
  client:
    # connection timeout
    connTimeout: 3s

  # Delivery service related config
  deliveryclient:
    # It sets the total time the delivery service may spend in reconnection
    # attempts until its retry logic gives up and returns an error
    reconnectTotalTimeThreshold: 3600s

    # It sets the delivery service <-> ordering service node connection timeout
    connTimeout: 3s

    # It sets the delivery service maximal delay between consecutive retries
    reConnectBackoffThreshold: 3600s

    # A list of orderer endpoint addresses which should be overridden
    # when found in channel configurations.
{{- if .AddressOverrides }}
    addressOverrides:
{{- range $i, $override := .AddressOverrides }}
      - from: {{ $override.From }}
        to: {{ $override.To }}
        caCertsFile: {{ $override.TLSCAPath }}
{{- end }}
{{- else }}
    addressOverrides: []
{{- end }}

  # Type for the local MSP - by default it's of type bccsp
  localMspType: bccsp

  # Used with Go profiling tools only in none production environment. In
  # production, it should be disabled (eg enabled: false)
  profile:
    enabled:     false
    listenAddress: 0.0.0.0:6060

  # Handlers defines custom handlers that can filter and mutate
  # objects passing within the peer, such as:
  #   Auth filter - reject or forward proposals from clients
  #   Decorators  - append or mutate the chaincode input passed to the chaincode
  #   Endorsers   - Custom signing over proposal response payload and its mutation
  # Valid handler definition contains:
  #   - A name which is a factory method name defined in
  #     core/handlers/library/library.go for statically compiled handlers
  #   - library path to shared object binary for pluggable filters
  # Auth filters and decorators are chained and executed in the order that
  # they are defined. For example:
  # authFilters:
  #   -
  #     name: FilterOne
  #     library: /opt/lib/filter.so
  #   -
  #     name: FilterTwo
  # decorators:
  #   -
  #     name: DecoratorOne
  #   -
  #     name: DecoratorTwo
  #     library: /opt/lib/decorator.so
  # Endorsers are configured as a map that its keys are the endorsement system chaincodes that are being overridden.
  # Below is an example that overrides the default ESCC and uses an endorsement plugin that has the same functionality
  # as the default ESCC.
  # If the 'library' property is missing, the name is used as the constructor method in the builtin library similar
  # to auth filters and decorators.
  # endorsers:
  #   escc:
  #     name: DefaultESCC
  #     library: /etc/hyperledger/fabric/plugin/escc.so
  handlers:
    authFilters:
      -
        name: DefaultAuth
      -
        name: ExpirationCheck    # This filter checks identity x509 certificate expiration
    decorators:
      -
        name: DefaultDecorator
    endorsers:
      escc:
        name: DefaultEndorsement
        library:
    validators:
      vscc:
        name: DefaultValidation
        library:

  #    library: /etc/hyperledger/fabric/plugin/escc.so
  # Number of goroutines that will execute transaction validation in parallel.
  # By default, the peer chooses the number of CPUs on the machine. Set this
  # variable to override that choice.
  # NOTE: overriding this value might negatively influence the performance of
  # the peer so please change this value only if you know what you're doing
  validatorPoolSize:

  # The discovery service is used by clients to query information about peers,
  # such as - which peers have joined a certain channel, what is the latest
  # channel config, and most importantly - given a chaincode and a channel,
  # what possible sets of peers satisfy the endorsement policy.
  discovery:
    enabled: true
    # Whether the authentication cache is enabled or not.
    authCacheEnabled: true
    # The maximum size of the cache, after which a purge takes place
    authCacheMaxSize: 1000
    # The proportion (0 to 1) of entries that remain in the cache after the cache is purged due to overpopulation
    authCachePurgeRetentionRatio: 0.75
    # Whether to allow non-admins to perform non channel scoped queries.
    # When this is false, it means that only peer admins can perform non channel scoped queries.
    orgMembersAllowedAccess: false

  # Limits is used to configure some internal resource limits.
  limits:
    # Concurrency limits the number of concurrently running requests to a service on each peer.
    # Currently this option is only applied to endorser service and deliver service.
    # When the property is missing or the value is 0, the concurrency limit is disabled for the service.
    concurrency:
      # endorserService limits concurrent requests to endorser service that handles chaincode deployment, query and invocation,
      # including both user chaincodes and system chaincodes.
      endorserService: 2500
      # deliverService limits concurrent event listeners registered to deliver service for blocks and transaction events.
      deliverService: 2500

###############################################################################
#
#    VM section
#
###############################################################################
vm:

  # Endpoint of the vm management system.  For docker can be one of the following in general
  # unix:///var/run/docker.sock
  # http://localhost:2375
  # https://localhost:2376
  endpoint: ""

  # settings for docker vms
  docker:
    tls:
      enabled: false
      ca:
        file: docker/ca.crt
      cert:
        file: docker/tls.crt
      key:
        file: docker/tls.key

    # Enables/disables the standard out/err from chaincode containers for
    # debugging purposes
    attachStdout: false

    # Parameters on creating docker container.
    # Container may be efficiently created using ipam & dns-server for cluster
    # NetworkMode - sets the networking mode for the container. Supported
    # Dns - a list of DNS servers for the container to use.
    # Docker Host Config are not supported and will not be used if set.
    # LogConfig - sets the logging driver (Type) and related options
    # (Config) for Docker. For more info,
    # https://docs.docker.com/engine/admin/logging/overview/
    # Note: Set LogConfig using Environment Variables is not supported.
    hostConfig:
      NetworkMode: host
      Dns:
      # - 192.168.0.1
      LogConfig:
        Type: json-file
        Config:
          max-size: "50m"
          max-file: "5"
      Memory: 2147483648

###############################################################################
#
#    Chaincode section
#
###############################################################################
chaincode:

  # The id is used by the Chaincode stub to register the executing Chaincode
  # ID with the Peer and is generally supplied through ENV variables
  id:
    path:
    name:

  # Generic builder environment, suitable for most chaincode types
  builder: $(DOCKER_NS)/fabric-ccenv:$(TWO_DIGIT_VERSION)

  pull: false

  golang:
    # golang will never need more than baseos
    runtime: $(DOCKER_NS)/fabric-baseos:$(TWO_DIGIT_VERSION)

    # whether or not golang chaincode should be linked dynamically
    dynamicLink: false

  java:
    # This is an image based on java:openjdk-8 with addition compiler
    # tools added for java shim layer packaging.
    # This image is packed with shim layer libraries that are necessary
    # for Java chaincode runtime.
    runtime: $(DOCKER_NS)/fabric-javaenv:$(TWO_DIGIT_VERSION)

  node:
    # This is an image based on node:$(NODE_VER)-alpine
    runtime: $(DOCKER_NS)/fabric-nodeenv:$(TWO_DIGIT_VERSION)

  # List of directories to treat as external builders and launchers for
  # chaincode. The external builder detection processing will iterate over the
  # builders in the order specified below.
  externalBuilders:
    - name: ccaas_builder
      path: {{.ExternalBuilderPath}}
  # The maximum duration to wait for the chaincode build and install process
  # to complete.
  installTimeout: 8m0s

  # Timeout duration for starting up a container and waiting for Register
  # to come through.
  startuptimeout: 5m0s

  # Timeout duration for Invoke and Init calls to prevent runaway.
  # This timeout is used by all chaincodes in all the channels, including
  # system chaincodes.
  # Note that during Invoke, if the image is not available (e.g. being
  # cleaned up when in development environment), the peer will automatically
  # build the image, which might take more time. In production environment,
  # the chaincode image is unlikely to be deleted, so the timeout could be
  # reduced accordingly.
  executetimeout: 30s

  # There are 2 modes: "dev" and "net".
  # In dev mode, user runs the chaincode after starting peer from
  # command line on local machine.
  # In net mode, peer will run chaincode in a docker container.
  mode: net

  # keepalive in seconds. In situations where the communication goes through a
  # proxy that does not support keep-alive, this parameter will maintain connection
  # between peer and chaincode.
  # A value <= 0 turns keepalive off
  keepalive: 0

  # enabled system chaincodes
  system:
    _lifecycle: enable
    cscc: enable
    lscc: enable
    escc: enable
    vscc: enable
    qscc: enable

  # Logging section for the chaincode container
  logging:
    # Default level for all loggers within the chaincode container
    level:  info
    # Override default level for the 'shim' logger
    shim:   warning
    # Format for the chaincode container logs
    format: '%{color}%{time:2006-01-02 15:04:05.000 MST} [%{module}] %{shortfunc} -> %{level:.4s} %{id:03x}%{color:reset} %{message}'

###############################################################################
#
#    Ledger section - ledger configuration encompasses both the blockchain
#    and the state
#
###############################################################################
ledger:

  blockchain:
  snapshots:
    rootDir: {{.DataPath}}/snapshots

  state:
    # stateDatabase - options are "goleveldb", "CouchDB"
    # goleveldb - default state database stored in goleveldb.
    # CouchDB - store state database in CouchDB
    stateDatabase: goleveldb
    # Limit on the number of records to return per query
    totalQueryLimit: 100000
    couchDBConfig:
      # It is recommended to run CouchDB on the same server as the peer, and
      # not map the CouchDB container port to a server port in docker-compose.
      # Otherwise proper security must be provided on the connection between
      # CouchDB client (on the peer) and server.
      couchDBAddress: 127.0.0.1:5984
      # This username must have read and write authority on CouchDB
      username:
      # The password is recommended to pass as an environment variable
      # during start up (eg CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD).
      # If it is stored here, the file must be access control protected
      # to prevent unintended users from discovering the password.
      password:
      # Number of retries for CouchDB errors
      maxRetries: 3
      # Number of retries for CouchDB errors during peer startup.
      # The delay between retries doubles for each attempt.
      # Default of 10 retries results in 11 attempts over 2 minutes.
      maxRetriesOnStartup: 10
      # CouchDB request timeout (unit: duration, e.g. 20s)
      requestTimeout: 35s
      # Limit on the number of records per each CouchDB query
      # Note that chaincode queries are only bound by totalQueryLimit.
      # Internally the chaincode may execute multiple CouchDB queries,
      # each of size internalQueryLimit.
      internalQueryLimit: 1000
      # Limit on the number of records per CouchDB bulk update batch
      maxBatchUpdateSize: 1000
      # Warm indexes after every N blocks.
      # This option warms any indexes that have been
      # deployed to CouchDB after every N blocks.
      # A value of 1 will warm indexes after every block commit,
      # to ensure fast selector queries.
      # Increasing the value may improve write efficiency of peer and CouchDB,
      # but may degrade query response time.
      warmIndexesAfterNBlocks: 1
      # Create the _global_changes system database
      # This is optional.  Creating the global changes database will require
      # additional system resources to track changes and maintain the database
      createGlobalChangesDB: false
      # CacheSize denotes the maximum mega bytes (MB) to be allocated for the in-memory state
      # cache. Note that CacheSize needs to be a multiple of 32 MB. If it is not a multiple
      # of 32 MB, the peer would round the size to the next multiple of 32 MB.
      # To disable the cache, 0 MB needs to be assigned to the cacheSize.
      cacheSize: 64

  history:
    # enableHistoryDatabase - options are true or false
    # Indicates if the history of key updates should be stored.
    # All history 'index' will be stored in goleveldb, regardless if using
    # CouchDB or alternate database for the state.
    enableHistoryDatabase: true

  pvtdataStore:
    # the maximum db batch size for converting
    # the ineligible missing data entries to eligible missing data entries
    collElgProcMaxDbBatchSize: 5000
    # the minimum duration (in milliseconds) between writing
    # two consecutive db batches for converting the ineligible missing data entries to eligible missing data entries
    collElgProcDbBatchesInterval: 1000

###############################################################################
#
#    Operations section
#
###############################################################################
operations:
  # host and port for the operations server
  listenAddress: 127.0.0.1:9443

  # TLS configuration for the operations endpoint
  tls:
    # TLS enabled
    enabled: false

    # path to PEM encoded server certificate for the operations server
    cert:
      file:

    # path to PEM encoded server key for the operations server
    key:
      file:

    # most operations service endpoints require client authentication when TLS
    # is enabled. clientAuthRequired requires client certificate authentication
    # at the TLS layer to access all resources.
    clientAuthRequired: false

    # paths to PEM encoded ca certificates to trust for client authentication
    clientRootCAs:
      files: []

###############################################################################
#
#    Metrics section
#
###############################################################################
metrics:
  # metrics provider is one of statsd, prometheus, or disabled
  provider: disabled

  # statsd configuration
  statsd:
    # network type: tcp or udp
    network: udp

    # statsd server address
    address: 127.0.0.1:8125

    # the interval at which locally cached counters and gauges are pushed
    # to statsd; timings are pushed immediately
    writeInterval: 10s

    # prefix is prepended to all emitted statsd metrics
    prefix:

`

// LocalPeer represents a local Fabric peer node
type LocalPeer struct {
	mspID           string
	db              *db.Queries
	opts            StartPeerOpts
	mode            string
	org             *fabricservice.OrganizationDTO
	organizationID  int64
	orgService      *fabricservice.OrganizationService
	keyService      *keymanagement.KeyManagementService
	nodeID          int64
	logger          *logger.Logger
	configService   *config.ConfigService
	settingsService *settingsservice.SettingsService
}

// NewLocalPeer creates a new LocalPeer instance
func NewLocalPeer(
	mspID string,
	db *db.Queries,
	opts StartPeerOpts,
	mode string,
	org *fabricservice.OrganizationDTO,
	organizationID int64,
	orgService *fabricservice.OrganizationService,
	keyService *keymanagement.KeyManagementService,
	nodeID int64,
	logger *logger.Logger,
	configService *config.ConfigService,
	settingsService *settingsservice.SettingsService,
) *LocalPeer {
	return &LocalPeer{
		mspID:           mspID,
		db:              db,
		opts:            opts,
		mode:            mode,
		org:             org,
		organizationID:  organizationID,
		orgService:      orgService,
		keyService:      keyService,
		nodeID:          nodeID,
		logger:          logger,
		configService:   configService,
		settingsService: settingsService,
	}
}

// getServiceName returns the systemd service name
func (p *LocalPeer) getServiceName() string {
	return fmt.Sprintf("fabric-peer-%s", strings.ReplaceAll(strings.ToLower(p.opts.ID), " ", "-"))
}

// getLaunchdServiceName returns the launchd service name
func (p *LocalPeer) getLaunchdServiceName() string {
	return fmt.Sprintf("dev.chainlaunch.peer.%s.%s",
		strings.ToLower(p.org.MspID),
		strings.ReplaceAll(strings.ToLower(p.opts.ID), " ", "-"))
}

// getServiceFilePath returns the systemd service file path
func (p *LocalPeer) getServiceFilePath() string {
	return fmt.Sprintf("/etc/systemd/system/%s.service", p.getServiceName())
}

// getLaunchdPlistPath returns the launchd plist file path
func (p *LocalPeer) getLaunchdPlistPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Library/LaunchAgents", p.getLaunchdServiceName()+".plist")
}

// GetStdOutPath returns the path to the stdout log file
func (p *LocalPeer) GetStdOutPath() string {
	dirPath := filepath.Join(p.configService.GetDataPath(), "peers",
		strings.ReplaceAll(strings.ToLower(p.opts.ID), " ", "-"))
	return filepath.Join(dirPath, p.getServiceName()+".log")
}

func (p *LocalPeer) getPeerPath() string {
	return filepath.Join(p.configService.GetDataPath(), "peers",
		strings.ReplaceAll(strings.ToLower(p.opts.ID), " ", "-"))
}

// getContainerName returns the docker container name
func (p *LocalPeer) getContainerName() (string, error) {
	org, err := p.orgService.GetOrganization(context.Background(), p.organizationID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization: %w", err)
	}
	return fmt.Sprintf("%s-%s",
		strings.ToLower(org.MspID),
		strings.ReplaceAll(strings.ToLower(p.opts.ID), " ", "-")), nil
}

// findPeerBinary finds the peer binary in PATH
func (p *LocalPeer) findPeerBinary() (string, error) {

	downloader, err := binaries.NewBinaryDownloader(p.configService)
	if err != nil {
		return "", fmt.Errorf("failed to create binary downloader: %w", err)
	}

	return downloader.GetBinaryPath(binaries.PeerBinary, p.opts.Version)
}

// Init initializes the peer configuration
func (p *LocalPeer) Init() (nodetypes.NodeDeploymentConfig, error) {
	ctx := context.Background()
	// Get node from database
	node, err := p.db.GetNode(ctx, p.nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	p.logger.Info("Initializing peer",
		"opts", p.opts,
		"node", node,
		"orgID", p.organizationID,
		"nodeID", p.nodeID,
	)

	// Get organization
	org, err := p.orgService.GetOrganization(ctx, p.organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	signCAKeyDB, err := p.keyService.GetKey(ctx, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve sign CA cert: %w", err)
	}

	tlsCAKeyDB, err := p.keyService.GetKey(ctx, int(org.TlsRootKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve TLS CA cert: %w", err)
	}
	isCA := 0
	description := "Sign key for " + p.opts.ID
	curveP256 := kmodels.ECCurveP256
	providerID := 1

	// Create Sign Key
	signKeyDB, err := p.keyService.CreateKey(ctx, kmodels.CreateKeyRequest{
		Algorithm:   kmodels.KeyAlgorithmEC,
		Name:        p.opts.ID,
		IsCA:        &isCA,
		Description: &description,
		Curve:       &curveP256,
		ProviderID:  &providerID,
	}, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to create sign key: %w", err)
	}

	// Sign Sign Key
	signKeyDB, err = p.keyService.SignCertificate(ctx, signKeyDB.ID, signCAKeyDB.ID, kmodels.CertificateRequest{
		CommonName:         p.opts.ID,
		Organization:       []string{org.MspID},
		OrganizationalUnit: []string{"peer"},
		DNSNames:           []string{p.opts.ID},
		IsCA:               true,
		KeyUsage:           x509.KeyUsageCertSign,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign sign key: %w", err)
	}

	signKey, err := p.keyService.GetDecryptedPrivateKey(int(signKeyDB.ID))
	if err != nil {
		return nil, fmt.Errorf("failed to get sign private key: %w", err)
	}

	// Create TLS key
	tlsKeyDB, err := p.keyService.CreateKey(ctx, kmodels.CreateKeyRequest{
		Algorithm:   kmodels.KeyAlgorithmEC,
		Name:        p.opts.ID,
		IsCA:        &isCA,
		Description: &description,
		Curve:       &curveP256,
		ProviderID:  &providerID,
	}, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to create sign key: %w", err)
	}
	domainNames := p.opts.DomainNames

	// Ensure localhost and 127.0.0.1 are included in domain names
	hasLocalhost := false
	hasLoopback := false
	var ipAddresses []net.IP
	var domains []string
	for _, domain := range domainNames {
		if domain == "localhost" {
			hasLocalhost = true
			domains = append(domains, domain)
			continue
		}
		if domain == "127.0.0.1" {
			hasLoopback = true
			ipAddresses = append(ipAddresses, net.ParseIP(domain))
			continue
		}
		if ip := net.ParseIP(domain); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			domains = append(domains, domain)
		}
	}
	if !hasLocalhost {
		domains = append(domains, "localhost")
	}
	if !hasLoopback {
		ipAddresses = append(ipAddresses, net.ParseIP("127.0.0.1"))
	}
	p.opts.DomainNames = domains

	// Sign TLS certificates
	validFor := kmodels.Duration(time.Hour * 24 * 365)
	tlsKeyDB, err = p.keyService.SignCertificate(ctx, tlsKeyDB.ID, tlsCAKeyDB.ID, kmodels.CertificateRequest{
		CommonName:         p.opts.ID,
		Organization:       []string{org.MspID},
		OrganizationalUnit: []string{"peer"},
		DNSNames:           domains,
		IPAddresses:        ipAddresses,
		IsCA:               true,
		ValidFor:           validFor,
		KeyUsage:           x509.KeyUsageCertSign,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign TLS certificate: %w", err)
	}
	tlsKey, err := p.keyService.GetDecryptedPrivateKey(int(tlsKeyDB.ID))
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS private key: %w", err)
	}
	// Create directory structure
	slugifiedID := strings.ReplaceAll(strings.ToLower(p.opts.ID), " ", "-")
	dirPath := filepath.Join(p.configService.GetDataPath(), "peers", slugifiedID)
	dataConfigPath := filepath.Join(dirPath, "data")
	mspConfigPath := filepath.Join(dirPath, "config")

	// Create directories
	if err := os.MkdirAll(dataConfigPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.MkdirAll(mspConfigPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create msp directory: %w", err)
	}

	// Write certificates and keys
	if err := p.writeCertificatesAndKeys(mspConfigPath, tlsKeyDB, signKeyDB, tlsKey, signKey, signCAKeyDB, tlsCAKeyDB); err != nil {
		return nil, fmt.Errorf("failed to write certificates and keys: %w", err)
	}

	// Create external builders
	if err := p.setupExternalBuilders(mspConfigPath); err != nil {
		return nil, fmt.Errorf("failed to setup external builders: %w", err)
	}

	// Write config files
	if err := p.writeConfigFiles(mspConfigPath, dataConfigPath); err != nil {
		return nil, fmt.Errorf("failed to write config files: %w", err)
	}

	return &nodetypes.FabricPeerDeploymentConfig{
		BaseDeploymentConfig: nodetypes.BaseDeploymentConfig{
			Type: "fabric-peer",
			Mode: p.mode,
		},
		OrganizationID:          p.organizationID,
		MSPID:                   p.mspID,
		SignKeyID:               int64(signKeyDB.ID),
		TLSKeyID:                int64(tlsKeyDB.ID),
		ListenAddress:           p.opts.ListenAddress,
		ChaincodeAddress:        p.opts.ChaincodeAddress,
		EventsAddress:           p.opts.EventsAddress,
		OperationsListenAddress: p.opts.OperationsListenAddress,
		ExternalEndpoint:        p.opts.ExternalEndpoint,
		DomainNames:             p.opts.DomainNames,
		SignCert:                *signKeyDB.Certificate,
		TLSCert:                 *tlsKeyDB.Certificate,
		CACert:                  *signCAKeyDB.Certificate,
		TLSCACert:               *tlsCAKeyDB.Certificate,
	}, nil
}

const DefaultPeerCmdTemplate = "{{.Cmd}}"

// Start starts the peer node
func (p *LocalPeer) Start() (interface{}, error) {
	p.logger.Info("Starting peer", "opts", p.opts)
	slugifiedID := strings.ReplaceAll(strings.ToLower(p.opts.ID), " ", "-")

	dirPath := filepath.Join(p.configService.GetDataPath(), "peers", slugifiedID)
	mspConfigPath := filepath.Join(dirPath, "config")
	dataConfigPath := filepath.Join(dirPath, "data")

	// Find peer binary
	peerBinary, err := p.findPeerBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to find peer binary: %w", err)
	}
	setting, err := p.settingsService.GetSetting(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get setting: %w", err)
	}
	var peerTemplateCMD string
	if setting.Config.PeerTemplateCMD == "" {
		peerTemplateCMD = DefaultPeerCmdTemplate
	} else {
		peerTemplateCMD = setting.Config.PeerTemplateCMD
	}

	// Parse template and build command
	tmpl, err := template.New("peer").Parse(peerTemplateCMD)
	if err != nil {
		return nil, fmt.Errorf("failed to parse peer template: %w", err)
	}

	var cmdBuf bytes.Buffer
	if err := tmpl.Execute(&cmdBuf, struct{ Cmd string }{
		Cmd: fmt.Sprintf("%s node start", peerBinary),
	}); err != nil {
		return nil, fmt.Errorf("failed to execute peer template: %w", err)
	}
	cmd := cmdBuf.String()
	env := p.buildPeerEnvironment(mspConfigPath)

	p.logger.Debug("Starting peer",
		"mode", p.mode,
		"cmd", cmd,
		"env", env,
		"dirPath", dirPath,
	)

	switch p.mode {
	case "service":
		return p.startService(cmd, env, dirPath)
	case "docker":
		return p.startDocker(env, mspConfigPath, dataConfigPath)
	default:
		return nil, fmt.Errorf("invalid mode: %s", p.mode)
	}
}

// buildPeerEnvironment builds the environment variables for the peer
func (p *LocalPeer) buildPeerEnvironment(mspConfigPath string) map[string]string {
	env := make(map[string]string)

	// Add custom environment variables from opts
	for k, v := range p.opts.Env {
		env[k] = v
	}

	// Add required environment variables
	// Default: use host paths
	env["CORE_PEER_MSPCONFIGPATH"] = mspConfigPath
	env["FABRIC_CFG_PATH"] = mspConfigPath
	env["CORE_PEER_TLS_ROOTCERT_FILE"] = filepath.Join(mspConfigPath, "tlscacerts/cacert.pem")
	env["CORE_PEER_TLS_KEY_FILE"] = filepath.Join(mspConfigPath, "tls.key")
	env["CORE_PEER_TLS_CLIENTCERT_FILE"] = filepath.Join(mspConfigPath, "tls.crt")
	env["CORE_PEER_TLS_CLIENTKEY_FILE"] = filepath.Join(mspConfigPath, "tls.key")
	env["CORE_PEER_TLS_CERT_FILE"] = filepath.Join(mspConfigPath, "tls.crt")
	env["CORE_PEER_TLS_CLIENTAUTHREQUIRED"] = "false"
	env["CORE_PEER_TLS_CLIENTROOTCAS_FILES"] = filepath.Join(mspConfigPath, "tlscacerts/cacert.pem")
	env["CORE_PEER_ADDRESS"] = p.opts.ExternalEndpoint
	env["CORE_PEER_GOSSIP_EXTERNALENDPOINT"] = p.opts.ExternalEndpoint
	env["CORE_PEER_GOSSIP_ENDPOINT"] = p.opts.ExternalEndpoint
	env["CORE_PEER_LISTENADDRESS"] = p.opts.ListenAddress
	env["CORE_PEER_CHAINCODELISTENADDRESS"] = p.opts.ChaincodeAddress
	env["CORE_PEER_EVENTS_ADDRESS"] = p.opts.EventsAddress
	env["CORE_OPERATIONS_LISTENADDRESS"] = p.opts.OperationsListenAddress
	env["CORE_PEER_NETWORKID"] = "peer01-nid"
	env["CORE_PEER_LOCALMSPID"] = p.mspID
	env["CORE_PEER_ID"] = p.opts.ID
	env["CORE_OPERATIONS_TLS_ENABLED"] = "false"
	env["CORE_OPERATIONS_TLS_CLIENTAUTHREQUIRED"] = "false"
	env["CORE_PEER_GOSSIP_ORGLEADER"] = "true"
	env["CORE_PEER_GOSSIP_BOOTSTRAP"] = p.opts.ExternalEndpoint
	env["CORE_PEER_PROFILE_ENABLED"] = "false"
	env["CORE_PEER_ADDRESSAUTODETECT"] = "false"
	env["CORE_LOGGING_GOSSIP"] = "info"
	env["FABRIC_LOGGING_SPEC"] = "info"
	env["CORE_LOGGING_LEDGER"] = "info"
	env["CORE_LOGGING_MSP"] = "info"
	env["CORE_PEER_COMMITTER_ENABLED"] = "true"
	env["CORE_PEER_DISCOVERY_TOUCHPERIOD"] = "60s"
	env["CORE_PEER_GOSSIP_USELEADERELECTION"] = "false"
	env["CORE_PEER_DISCOVERY_PERIOD"] = "60s"
	env["CORE_METRICS_PROVIDER"] = "prometheus"
	env["CORE_LOGGING_CAUTHDSL"] = "info"
	env["CORE_LOGGING_POLICIES"] = "info"
	env["CORE_LEDGER_STATE_STATEDATABASE"] = "goleveldb"
	env["CORE_PEER_TLS_ENABLED"] = "true"
	env["CORE_LOGGING_GRPC"] = "info"
	env["CORE_LOGGING_PEER"] = "info"

	// If running in docker mode, override file paths to container paths
	if p.mode == "docker" {
		env["CORE_PEER_MSPCONFIGPATH"] = "/etc/hyperledger/fabric/msp"
		env["FABRIC_CFG_PATH"] = "/etc/hyperledger/fabric/msp"
		env["CORE_PEER_TLS_ROOTCERT_FILE"] = "/etc/hyperledger/fabric/msp/tlscacerts/cacert.pem"
		env["CORE_PEER_TLS_KEY_FILE"] = "/etc/hyperledger/fabric/msp/tls.key"
		env["CORE_PEER_TLS_CLIENTCERT_FILE"] = "/etc/hyperledger/fabric/msp/tls.crt"
		env["CORE_PEER_TLS_CLIENTKEY_FILE"] = "/etc/hyperledger/fabric/msp/tls.key"
		env["CORE_PEER_TLS_CERT_FILE"] = "/etc/hyperledger/fabric/msp/tls.crt"
		env["CORE_PEER_TLS_CLIENTROOTCAS_FILES"] = "/etc/hyperledger/fabric/msp/tlscacerts/cacert.pem"
	}

	return env
}

// startDocker starts the peer in a docker container
func (p *LocalPeer) startDocker(env map[string]string, mspConfigPath, dataConfigPath string) (*StartDockerResponse, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	// Pull the image first
	imageName := fmt.Sprintf("hyperledger/fabric-peer:%s", p.opts.Version)
	reader, err := cli.ImagePull(context.Background(), imageName, image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader) // Wait for pull to complete

	containerName, err := p.getContainerName()
	if err != nil {
		return nil, fmt.Errorf("failed to get container name: %w", err)
	}

	// Helper to extract port from address (host:port or just :port)
	extractPort := func(addr string) string {
		parts := strings.Split(addr, ":")
		if len(parts) > 1 {
			return parts[len(parts)-1]
		}
		return addr
	}

	peerPort := extractPort(p.opts.ListenAddress)
	chaincodePort := extractPort(p.opts.ChaincodeAddress)
	eventsPort := extractPort(p.opts.EventsAddress)
	operationsPort := extractPort(p.opts.OperationsListenAddress)

	// Configure port bindings
	portBindings := map[nat.Port][]nat.PortBinding{
		nat.Port(peerPort):       {{HostIP: "0.0.0.0", HostPort: peerPort}},
		nat.Port(chaincodePort):  {{HostIP: "0.0.0.0", HostPort: chaincodePort}},
		nat.Port(eventsPort):     {{HostIP: "0.0.0.0", HostPort: eventsPort}},
		nat.Port(operationsPort): {{HostIP: "0.0.0.0", HostPort: operationsPort}},
	}

	// Configure volume bindings
	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: mspConfigPath,
			Target: "/etc/hyperledger/fabric/msp",
		},
		{
			Type:   mount.TypeBind,
			Source: dataConfigPath,
			Target: "/var/hyperledger/production",
		},
	}

	// Create container
	resp, err := cli.ContainerCreate(context.Background(),
		&container.Config{
			Image: imageName,
			Cmd:   []string{"peer", "node", "start"},
			Env:   mapToEnvSlice(env),
		},
		&container.HostConfig{
			PortBindings: portBindings,
			Mounts:       mounts,
		},
		nil,
		nil,
		containerName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := cli.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	return &StartDockerResponse{
		Mode:          "docker",
		ContainerName: containerName,
	}, nil
}

// Helper function to convert map to env slice
func mapToEnvSlice(m map[string]string) []string {
	var env []string
	for k, v := range m {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

// Stop stops the peer node
func (p *LocalPeer) Stop() error {
	if p.mode == "service" {
		var cmd *exec.Cmd
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("launchctl", "unload", p.getLaunchdPlistPath())
		} else {
			cmd = exec.Command("systemctl", "stop", p.getServiceName())
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to stop service: %w: %s", err, output)
		}
		return nil
	}
	p.logger.Info("Stopping peer", "opts", p.opts)

	switch p.mode {
	case "service":
		platform := runtime.GOOS
		switch platform {
		case "linux":
			return p.stopSystemdService()
		case "darwin":
			return p.stopLaunchdService()
		default:
			return fmt.Errorf("unsupported platform for service mode: %s", platform)
		}
	case "docker":
		return p.stopDocker()
	default:
		return fmt.Errorf("invalid mode: %s", p.mode)
	}
}

// stopDocker stops the peer docker container
func (p *LocalPeer) stopDocker() error {
	containerName, err := p.getContainerName()
	if err != nil {
		return fmt.Errorf("failed to get container name: %w", err)
	}

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	ctx := context.Background()

	// Stop and remove container
	if err := cli.ContainerRemove(ctx, containerName, container.RemoveOptions{
		Force: true,
	}); err != nil {
		p.logger.Warn("Failed to remove docker container", "error", err)
		// Don't return error as container might not exist
	}

	return nil
}

// stopSystemdService stops the systemd service
func (p *LocalPeer) stopSystemdService() error {
	serviceName := p.getServiceName()

	// Stop the service
	if err := p.execSystemctl("stop", serviceName); err != nil {
		return fmt.Errorf("failed to stop systemd service: %w", err)
	}

	// Disable the service
	if err := p.execSystemctl("disable", serviceName); err != nil {
		p.logger.Warn("Failed to disable systemd service", "error", err)
		// Don't return error as this is not critical
	}

	// Remove the service file
	if err := os.Remove(p.getServiceFilePath()); err != nil {
		if !os.IsNotExist(err) {
			p.logger.Warn("Failed to remove service file", "error", err)
			// Don't return error as this is not critical
		}
	}

	// Reload systemd daemon
	if err := p.execSystemctl("daemon-reload"); err != nil {
		p.logger.Warn("Failed to reload systemd daemon", "error", err)
		// Don't return error as this is not critical
	}

	return nil
}

// stopLaunchdService stops the launchd service
func (p *LocalPeer) stopLaunchdService() error {
	// Stop the service
	stopCmd := exec.Command("launchctl", "stop", p.getLaunchdServiceName())
	if err := stopCmd.Run(); err != nil {
		p.logger.Warn("Failed to stop launchd service", "error", err)
		// Continue anyway as we want to make sure it's unloaded
	}

	// Unload the service
	unloadCmd := exec.Command("launchctl", "unload", p.getLaunchdPlistPath())
	if err := unloadCmd.Run(); err != nil {
		return fmt.Errorf("failed to unload launchd service: %w", err)
	}

	return nil
}

// execSystemctl executes a systemctl command
func (p *LocalPeer) execSystemctl(command string, args ...string) error {
	cmdArgs := append([]string{command}, args...)

	// Check if sudo is available
	sudoPath, err := exec.LookPath("sudo")
	if err == nil {
		// sudo is available, use it
		cmdArgs = append([]string{"systemctl"}, cmdArgs...)
		cmd := exec.Command(sudoPath, cmdArgs...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("systemctl %s failed: %w", command, err)
		}
	} else {
		// sudo is not available, run directly
		cmd := exec.Command("systemctl", cmdArgs...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("systemctl %s failed: %w", command, err)
		}
	}

	return nil
}

// RenewCertificates renews the peer's TLS and signing certificates
func (p *LocalPeer) RenewCertificates(peerDeploymentConfig *nodetypes.FabricPeerDeploymentConfig) error {

	ctx := context.Background()
	p.logger.Info("Starting certificate renewal for peer", "peerID", p.opts.ID)

	// Get organization details
	org, err := p.orgService.GetOrganization(ctx, p.organizationID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	if peerDeploymentConfig.SignKeyID == 0 || peerDeploymentConfig.TLSKeyID == 0 {
		return fmt.Errorf("peer node does not have required key IDs")
	}

	// Get the CA certificates
	signCAKey, err := p.keyService.GetKey(ctx, int(org.SignKeyID.Int64))
	if err != nil {
		return fmt.Errorf("failed to get sign CA key: %w", err)
	}

	tlsCAKey, err := p.keyService.GetKey(ctx, int(org.TlsRootKeyID.Int64))
	if err != nil {
		return fmt.Errorf("failed to get TLS CA key: %w", err)
	}
	// In case the sign key is not signed by the CA, set the signing key ID to the CA key ID
	signKeyDB, err := p.keyService.GetKey(ctx, int(peerDeploymentConfig.SignKeyID))
	if err != nil {
		return fmt.Errorf("failed to get sign private key: %w", err)
	}
	if signKeyDB.SigningKeyID == nil || *signKeyDB.SigningKeyID == 0 {
		// Set the signing key ID to the organization's sign CA key ID
		err = p.keyService.SetSigningKeyIDForKey(ctx, int(peerDeploymentConfig.SignKeyID), int(signCAKey.ID))
		if err != nil {
			return fmt.Errorf("failed to set signing key ID for sign key: %w", err)
		}
	}

	tlsKeyDB, err := p.keyService.GetKey(ctx, int(peerDeploymentConfig.TLSKeyID))
	if err != nil {
		return fmt.Errorf("failed to get TLS private key: %w", err)
	}

	if tlsKeyDB.SigningKeyID == nil || *tlsKeyDB.SigningKeyID == 0 {
		// Set the signing key ID to the organization's sign CA key ID
		err = p.keyService.SetSigningKeyIDForKey(ctx, int(peerDeploymentConfig.TLSKeyID), int(tlsCAKey.ID))
		if err != nil {
			return fmt.Errorf("failed to set signing key ID for TLS key: %w", err)
		}
	}
	// Renew signing certificate
	validFor := kmodels.Duration(time.Hour * 24 * 365) // 1 year validity
	_, err = p.keyService.RenewCertificate(ctx, int(peerDeploymentConfig.SignKeyID), kmodels.CertificateRequest{
		CommonName:         p.opts.ID,
		Organization:       []string{org.MspID},
		OrganizationalUnit: []string{"peer"},
		DNSNames:           []string{p.opts.ID},
		IsCA:               false,
		ValidFor:           validFor,
		KeyUsage:           x509.KeyUsageCertSign,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		return fmt.Errorf("failed to renew signing certificate: %w", err)
	}

	// Renew TLS certificate
	domainNames := p.opts.DomainNames
	var ipAddresses []net.IP
	var domains []string

	// Ensure localhost and 127.0.0.1 are included
	hasLocalhost := false
	hasLoopback := false
	for _, domain := range domainNames {
		if domain == "localhost" {
			hasLocalhost = true
			domains = append(domains, domain)
			continue
		}
		if domain == "127.0.0.1" {
			hasLoopback = true
			ipAddresses = append(ipAddresses, net.ParseIP(domain))
			continue
		}
		if ip := net.ParseIP(domain); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			domains = append(domains, domain)
		}
	}
	if !hasLocalhost {
		domains = append(domains, "localhost")
	}
	if !hasLoopback {
		ipAddresses = append(ipAddresses, net.ParseIP("127.0.0.1"))
	}

	_, err = p.keyService.RenewCertificate(ctx, int(peerDeploymentConfig.TLSKeyID), kmodels.CertificateRequest{
		CommonName:         p.opts.ID,
		Organization:       []string{org.MspID},
		OrganizationalUnit: []string{"peer"},
		DNSNames:           domains,
		IPAddresses:        ipAddresses,
		IsCA:               false,
		ValidFor:           validFor,
		KeyUsage:           x509.KeyUsageCertSign,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		return fmt.Errorf("failed to renew TLS certificate: %w", err)
	}

	// Get the private keys
	signKey, err := p.keyService.GetDecryptedPrivateKey(int(peerDeploymentConfig.SignKeyID))
	if err != nil {
		return fmt.Errorf("failed to get sign private key: %w", err)
	}

	tlsKey, err := p.keyService.GetDecryptedPrivateKey(int(peerDeploymentConfig.TLSKeyID))
	if err != nil {
		return fmt.Errorf("failed to get TLS private key: %w", err)
	}

	// Update the certificates in the MSP directory
	peerPath := p.getPeerPath()
	mspConfigPath := filepath.Join(peerPath, "config")

	err = p.writeCertificatesAndKeys(
		mspConfigPath,
		tlsKeyDB,
		signKeyDB,
		tlsKey,
		signKey,
		signCAKey,
		tlsCAKey,
	)
	if err != nil {
		return fmt.Errorf("failed to write renewed certificates: %w", err)
	}

	// Restart the peer
	_, err = p.Start()
	if err != nil {
		return fmt.Errorf("failed to restart peer after certificate renewal: %w", err)
	}

	p.logger.Info("Successfully renewed peer certificates", "peerID", p.opts.ID)
	p.logger.Info("Restarting peer after certificate renewal")
	// Stop the peer before renewing certificates
	if err := p.Stop(); err != nil {
		return fmt.Errorf("failed to stop peer before certificate renewal: %w", err)
	}
	p.logger.Info("Successfully stopped peer before certificate renewal")
	p.logger.Info("Starting peer after certificate renewal")
	_, err = p.Start()
	if err != nil {
		return fmt.Errorf("failed to start peer after certificate renewal: %w", err)
	}
	p.logger.Info("Successfully started peer after certificate renewal")
	return nil
}

type NetworkConfigResponse struct {
	NetworkConfig string
}
type Org struct {
	MSPID     string
	CertAuths []string
	Peers     []string
	Orderers  []string
}
type Peer struct {
	Name      string
	URL       string
	TLSCACert string
}
type CA struct {
	Name         string
	URL          string
	TLSCert      string
	EnrollID     string
	EnrollSecret string
}

type Orderer struct {
	URL       string
	Name      string
	TLSCACert string
}

const tmplGoConfig = `
name: hlf-network
version: 1.0.0
client:
  organization: "{{ .Organization }}"
{{- if not .Organizations }}
organizations: {}
{{- else }}
organizations:
  {{ range $org := .Organizations }}
  {{ $org.MSPID }}:
    mspid: {{ $org.MSPID }}
    cryptoPath: /tmp/cryptopath
    users: {}
{{- if not $org.CertAuths }}
    certificateAuthorities: []
{{- else }}
    certificateAuthorities: 
      {{- range $ca := $org.CertAuths }}
      - {{ $ca.Name }}
 	  {{- end }}
{{- end }}
{{- if not $org.Peers }}
    peers: []
{{- else }}
    peers:
      {{- range $peer := $org.Peers }}
      - {{ $peer }}
 	  {{- end }}
{{- end }}
{{- if not $org.Orderers }}
    orderers: []
{{- else }}
    orderers:
      {{- range $orderer := $org.Orderers }}
      - {{ $orderer }}
 	  {{- end }}

    {{- end }}
{{- end }}
{{- end }}

{{- if not .Orderers }}
{{- else }}
orderers:
{{- range $orderer := .Orderers }}
  {{$orderer.Name}}:
    url: {{ $orderer.URL }}
    grpcOptions:
      allow-insecure: false
    tlsCACerts:
      pem: |
{{ $orderer.TLSCACert | indent 8 }}
{{- end }}
{{- end }}

{{- if not .Peers }}
{{- else }}
peers:
  {{- range $peer := .Peers }}
  {{$peer.Name}}:
    url: {{ $peer.URL }}
    tlsCACerts:
      pem: |
{{ $peer.TLSCACert | indent 8 }}
{{- end }}
{{- end }}

{{- if not .CertAuths }}
{{- else }}
certificateAuthorities:
{{- range $ca := .CertAuths }}
  {{ $ca.Name }}:
    url: https://{{ $ca.URL }}
{{if $ca.EnrollID }}
    registrar:
        enrollId: {{ $ca.EnrollID }}
        enrollSecret: "{{ $ca.EnrollSecret }}"
{{ end }}
    caName: {{ $ca.CAName }}
    tlsCACerts:
      pem: 
       - |
{{ $ca.TLSCert | indent 12 }}

{{- end }}
{{- end }}

channels:
  _default:
{{- if not .Orderers }}
    orderers: []
{{- else }}
    orderers:
{{- range $orderer := .Orderers }}
      - {{$orderer.Name}}
{{- end }}
{{- end }}
{{- if not .Peers }}
    peers: {}
{{- else }}
    peers:
{{- range $peer := .Peers }}
       {{$peer.Name}}:
        discover: true
        endorsingPeer: true
        chaincodeQuery: true
        ledgerQuery: true
        eventSource: true
{{- end }}
{{- end }}

`

func (p *LocalPeer) generateNetworkConfigForPeer(
	peerUrl string, peerMspID string, peerTlsCACert string, ordererUrl string, ordererTlsCACert string) (*NetworkConfigResponse, error) {

	tmpl, err := template.New("networkConfig").Funcs(sprig.HermeticTxtFuncMap()).Parse(tmplGoConfig)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	orgs := []*Org{}
	var peers []*Peer
	var certAuths []*CA
	var ordererNodes []*Orderer

	org := &Org{
		MSPID:     peerMspID,
		CertAuths: []string{},
		Peers:     []string{},
		Orderers:  []string{},
	}
	orgs = append(orgs, org)
	if peerTlsCACert != "" {
		peer := &Peer{
			Name:      "peer0",
			URL:       peerUrl,
			TLSCACert: peerTlsCACert,
		}
		org.Peers = append(org.Peers, "peer0")
		peers = append(peers, peer)
	}
	if ordererTlsCACert != "" && ordererUrl != "" {
		orderer := &Orderer{
			URL:       ordererUrl,
			Name:      "orderer0",
			TLSCACert: ordererTlsCACert,
		}
		ordererNodes = append(ordererNodes, orderer)
	}
	err = tmpl.Execute(&buf, map[string]interface{}{
		"Peers":         peers,
		"Orderers":      ordererNodes,
		"Organizations": orgs,
		"CertAuths":     certAuths,
		"Organization":  peerMspID,
		"Internal":      false,
	})
	if err != nil {
		return nil, err
	}
	p.logger.Debugf("Network config: %s", buf.String())
	return &NetworkConfigResponse{
		NetworkConfig: buf.String(),
	}, nil
}

// JoinChannel joins the peer to a channel
func (p *LocalPeer) JoinChannel(genesisBlock []byte) error {
	p.logger.Info("Joining peer to channel", "peer", p.opts.ID)
	var genesisBlockProto cb.Block
	err := proto.Unmarshal(genesisBlock, &genesisBlockProto)
	if err != nil {
		return fmt.Errorf("failed to unmarshal genesis block: %w", err)
	}
	ctx := context.Background()
	tlsCACert, err := p.GetTLSRootCACert(ctx)
	if err != nil {
		return fmt.Errorf("failed to get TLS root CA cert: %w", err)
	}
	peerConn, err := p.CreatePeerConnection(ctx, p.opts.ExternalEndpoint, tlsCACert)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConn.Close()

	adminIdentity, _, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get admin identity: %w", err)
	}

	err = channel.JoinChannel(ctx, peerConn, adminIdentity, &genesisBlockProto)
	if err != nil {
		return fmt.Errorf("failed to join channel: %w", err)
	}

	return nil

}

// writeCertificatesAndKeys writes the certificates and keys to the MSP directory structure
func (p *LocalPeer) writeCertificatesAndKeys(
	mspConfigPath string,
	tlsCert *kmodels.KeyResponse,
	signCert *kmodels.KeyResponse,
	tlsKey string,
	signKey string,
	signCACert *kmodels.KeyResponse,
	tlsCACert *kmodels.KeyResponse,
) error {
	// Write TLS certificates and keys
	if err := os.WriteFile(filepath.Join(mspConfigPath, "tls.crt"), []byte(*tlsCert.Certificate), 0644); err != nil {
		return fmt.Errorf("failed to write TLS certificate: %w", err)
	}
	if err := os.WriteFile(filepath.Join(mspConfigPath, "tls.key"), []byte(tlsKey), 0600); err != nil {
		return fmt.Errorf("failed to write TLS key: %w", err)
	}

	// Create and write to signcerts directory
	signcertsPath := filepath.Join(mspConfigPath, "signcerts")
	if err := os.MkdirAll(signcertsPath, 0755); err != nil {
		return fmt.Errorf("failed to create signcerts directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(signcertsPath, "cert.pem"), []byte(*signCert.Certificate), 0644); err != nil {
		return fmt.Errorf("failed to write signing certificate: %w", err)
	}

	// Write root CA certificate
	if err := os.WriteFile(filepath.Join(mspConfigPath, "cacert.pem"), []byte(*signCACert.Certificate), 0644); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	// Create and write to cacerts directory
	cacertsPath := filepath.Join(mspConfigPath, "cacerts")
	if err := os.MkdirAll(cacertsPath, 0755); err != nil {
		return fmt.Errorf("failed to create cacerts directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cacertsPath, "cacert.pem"), []byte(*signCACert.Certificate), 0644); err != nil {
		return fmt.Errorf("failed to write CA certificate to cacerts: %w", err)
	}

	// Create and write to tlscacerts directory
	tlscacertsPath := filepath.Join(mspConfigPath, "tlscacerts")
	if err := os.MkdirAll(tlscacertsPath, 0755); err != nil {
		return fmt.Errorf("failed to create tlscacerts directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tlscacertsPath, "cacert.pem"), []byte(*tlsCACert.Certificate), 0644); err != nil {
		return fmt.Errorf("failed to write TLS CA certificate: %w", err)
	}

	// Create and write to keystore directory
	keystorePath := filepath.Join(mspConfigPath, "keystore")
	if err := os.MkdirAll(keystorePath, 0755); err != nil {
		return fmt.Errorf("failed to create keystore directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(keystorePath, "key.pem"), []byte(signKey), 0600); err != nil {
		return fmt.Errorf("failed to write signing key: %w", err)
	}

	return nil
}

// setupExternalBuilders creates and configures the external builders for chaincode
func (p *LocalPeer) setupExternalBuilders(mspConfigPath string) error {
	// Create external builder directory structure
	rootExternalBuilderPath := filepath.Join(mspConfigPath, "ccaas")
	binExternalBuilderPath := filepath.Join(rootExternalBuilderPath, "bin")
	if err := os.MkdirAll(binExternalBuilderPath, 0755); err != nil {
		return fmt.Errorf("failed to create external builder directory: %w", err)
	}

	// Create build script
	buildScript := `#!/bin/bash

SOURCE=$1
OUTPUT=$3

#external chaincodes expect connection.json file in the chaincode package
if [ ! -f "$SOURCE/connection.json" ]; then
    >&2 echo "$SOURCE/connection.json not found"
    exit 1
fi

#simply copy the endpoint information to specified output location
cp $SOURCE/connection.json $OUTPUT/connection.json

if [ -d "$SOURCE/metadata" ]; then
    cp -a $SOURCE/metadata $OUTPUT/metadata
fi

exit 0`

	if err := os.WriteFile(filepath.Join(binExternalBuilderPath, "build"), []byte(buildScript), 0755); err != nil {
		return fmt.Errorf("failed to write build script: %w", err)
	}

	// Create detect script
	detectScript := `#!/bin/bash

METADIR=$2
# check if the "type" field is set to "external"
# crude way without jq which is not in the default fabric peer image
TYPE=$(tr -d '\n' < "$METADIR/metadata.json" | awk -F':' '{ for (i = 1; i < NF; i++){ if ($i~/type/) { print $(i+1); break }}}'| cut -d\" -f2)

if [ "$TYPE" = "ccaas" ]; then
    exit 0
fi

exit 1`

	if err := os.WriteFile(filepath.Join(binExternalBuilderPath, "detect"), []byte(detectScript), 0755); err != nil {
		return fmt.Errorf("failed to write detect script: %w", err)
	}

	// Create release script
	releaseScript := `#!/bin/bash

BLD="$1"
RELEASE="$2"

if [ -d "$BLD/metadata" ]; then
   cp -a "$BLD/metadata/"* "$RELEASE/"
fi

#external chaincodes expect artifacts to be placed under "$RELEASE"/chaincode/server
if [ -f $BLD/connection.json ]; then
   mkdir -p "$RELEASE"/chaincode/server
   cp $BLD/connection.json "$RELEASE"/chaincode/server

   #if tls_required is true, copy TLS files (using above example, the fully qualified path for these fils would be "$RELEASE"/chaincode/server/tls)

   exit 0
fi

exit 1`

	if err := os.WriteFile(filepath.Join(binExternalBuilderPath, "release"), []byte(releaseScript), 0755); err != nil {
		return fmt.Errorf("failed to write release script: %w", err)
	}

	return nil
}

const configYamlContent = `NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: orderer
`

// writeConfigFiles writes the config.yaml and core.yaml files
func (p *LocalPeer) writeConfigFiles(mspConfigPath, dataConfigPath string) error {
	// Write config.yaml
	if err := os.WriteFile(filepath.Join(mspConfigPath, "config.yaml"), []byte(configYamlContent), 0644); err != nil {
		return fmt.Errorf("failed to write config.yaml: %w", err)
	}
	convertedOverrides, err := p.convertAddressOverrides(mspConfigPath, p.opts.AddressOverrides)
	if err != nil {
		return fmt.Errorf("failed to convert address overrides: %w", err)
	}

	// Define template data
	data := struct {
		PeerID                  string
		ListenAddress           string
		ChaincodeAddress        string
		ExternalEndpoint        string
		DataPath                string
		MSPID                   string
		ExternalBuilderPath     string
		OperationsListenAddress string
		AddressOverrides        []AddressOverridePath
	}{
		PeerID:                  p.opts.ID,
		ListenAddress:           p.opts.ListenAddress,
		ChaincodeAddress:        p.opts.ChaincodeAddress,
		ExternalEndpoint:        p.opts.ExternalEndpoint,
		DataPath:                dataConfigPath,
		MSPID:                   p.mspID,
		ExternalBuilderPath:     filepath.Join(mspConfigPath, "ccaas"),
		OperationsListenAddress: p.opts.OperationsListenAddress,
		AddressOverrides:        convertedOverrides,
	}

	// Create template
	tmpl, err := template.New("core.yaml").Parse(coreYamlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse core.yaml template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute core.yaml template: %w", err)
	}

	// Write core.yaml
	if err := os.WriteFile(filepath.Join(mspConfigPath, "core.yaml"), buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write core.yaml: %w", err)
	}

	return nil
}

func (p *LocalPeer) getLogPath() string {
	return p.GetStdOutPath()
}

// TailLogs tails the logs of the peer service
func (p *LocalPeer) TailLogs(ctx context.Context, tail int, follow bool) (<-chan string, error) {
	logChan := make(chan string, 100)

	if p.mode == "docker" {
		containerName, err := p.getContainerName()
		if err != nil {
			close(logChan)
			return logChan, err
		}
		go func() {
			defer close(logChan)
			cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
			if err != nil {
				p.logger.Error("Failed to create docker client", "error", err)
				return
			}
			defer cli.Close()

			options := container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     follow,
				Details:    true,
				Tail:       fmt.Sprintf("%d", tail),
			}
			reader, err := cli.ContainerLogs(ctx, containerName, options)
			if err != nil {
				p.logger.Error("Failed to get docker logs", "error", err)
				return
			}
			defer reader.Close()

			header := make([]byte, 8)
			for {
				// Read the 8-byte header
				_, err := io.ReadFull(reader, header)
				if err != nil {
					if err != io.EOF {
						p.logger.Error("Failed to read docker log header", "error", err)
					}
					return
				}
				// Get the payload length
				length := int(uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7]))
				if length == 0 {
					continue
				}
				// Read the payload
				payload := make([]byte, length)
				_, err = io.ReadFull(reader, payload)
				if err != nil {
					if err != io.EOF {
						p.logger.Error("Failed to read docker log payload", "error", err)
					}
					return
				}

				// Strip trailing newlines to avoid double line breaks
				// cleanLine := strings.TrimRight(string(payload), "\r\n")
				select {
				case <-ctx.Done():
					return
				case logChan <- string(payload):
				}
			}
		}()
		return logChan, nil
	}

	// Service mode: use file tailing
	logPath := p.GetStdOutPath()
	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		close(logChan)
		return logChan, fmt.Errorf("log file does not exist: %s", logPath)
	}
	go func() {
		defer close(logChan)
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			// For Windows, use PowerShell Get-Content
			if follow {
				cmd = exec.Command("powershell", "Get-Content", "-Path", logPath, "-Tail", fmt.Sprintf("%d", tail), "-Wait")
			} else {
				cmd = exec.Command("powershell", "Get-Content", "-Path", logPath, "-Tail", fmt.Sprintf("%d", tail))
			}
		} else {
			// For Unix-like systems, use tail command
			if follow {
				cmd = exec.Command("tail", "-n", fmt.Sprintf("%d", tail), "-f", logPath)
			} else {
				cmd = exec.Command("tail", "-n", fmt.Sprintf("%d", tail), logPath)
			}
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			p.logger.Error("Failed to create stdout pipe", "error", err)
			return
		}
		if err := cmd.Start(); err != nil {
			p.logger.Error("Failed to start tail command", "error", err)
			return
		}
		scanner := bufio.NewScanner(stdout)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				cmd.Process.Kill()
				return
			case logChan <- scanner.Text() + "\n":
			}
		}
		if err := cmd.Wait(); err != nil {
			if ctx.Err() == nil {
				p.logger.Error("Tail command failed", "error", err)
			}
		}
	}()
	return logChan, nil
}

// Add this struct near the top of the file with other type definitions
type AdminCert struct {
	Cert   string
	CACert string
	PK     string
}

type CAConfig struct {
	TLSCACert string
}

// PrepareAdminCertMSP prepares the MSP directory structure with admin credentials
func (p *LocalPeer) PrepareAdminCertMSP(mspID string) (string, error) {
	// Create all required directories with proper permissions
	// Determine admin cert path based on mspID
	adminMspPath := filepath.Join(p.configService.GetDataPath(), "orgs", strings.ToLower(mspID), "users/admin")

	// Check if admin cert directory already exists
	if _, err := os.Stat(adminMspPath); err == nil {
		// Directory already exists, return path
		return adminMspPath, nil
	} else if !os.IsNotExist(err) {
		// Error other than not exists
		return "", fmt.Errorf("failed to check admin cert path: %w", err)
	}

	dirs := []string{
		filepath.Join(adminMspPath, "cacerts"),
		filepath.Join(adminMspPath, "keystore"),
		filepath.Join(adminMspPath, "signcerts"),
		filepath.Join(adminMspPath, "tlscacerts"),
	}
	ctx := context.Background()
	org, err := p.db.GetFabricOrganizationByID(ctx, p.organizationID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization: %w", err)
	}
	if !org.AdminSignKeyID.Valid {
		return "", fmt.Errorf("admin sign key is not set")
	}
	adminSignKeyDB, err := p.keyService.GetKey(ctx, int(org.AdminSignKeyID.Int64))
	if err != nil {
		return "", fmt.Errorf("failed to get admin sign key: %w", err)
	}
	if adminSignKeyDB.Certificate == nil {
		return "", fmt.Errorf("admin sign key is not set")
	}
	adminSignKey, err := p.keyService.GetDecryptedPrivateKey(int(org.AdminSignKeyID.Int64))
	if err != nil {
		return "", fmt.Errorf("failed to get decrypted admin sign key: %w", err)
	}
	adminCert := *adminSignKeyDB.Certificate
	signCAKeyDB, err := p.keyService.GetKey(ctx, int(org.SignKeyID.Int64))
	if err != nil {
		return "", fmt.Errorf("failed to get sign CA key: %w", err)
	}
	signCACert := *signCAKeyDB.Certificate

	tlsCAKeyDB, err := p.keyService.GetKey(ctx, int(org.TlsRootKeyID.Int64))
	if err != nil {
		return "", fmt.Errorf("failed to get TLS CA key: %w", err)
	}
	tlsCACert := *tlsCAKeyDB.Certificate

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write certificates and keys to their respective locations
	files := map[string]string{
		filepath.Join(adminMspPath, "cacerts", "cacert.pem"):    signCACert,
		filepath.Join(adminMspPath, "keystore", "priv_sk"):      adminSignKey,
		filepath.Join(adminMspPath, "signcerts", "admin.pem"):   adminCert,
		filepath.Join(adminMspPath, "tlscacerts", "cacert.pem"): tlsCACert,
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write file %s: %w", path, err)
		}
	}

	// Write config.yaml
	configYaml := `NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: orderer
`

	if err := os.WriteFile(filepath.Join(adminMspPath, "config.yaml"), []byte(configYaml), 0644); err != nil {
		return "", fmt.Errorf("failed to write config.yaml: %w", err)
	}

	return adminMspPath, nil
}

// LeaveChannel removes the peer from a channel
func (p *LocalPeer) LeaveChannel(channelID string) error {
	err := p.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop peer: %w", err)
	}

	p.logger.Info("Removing peer from channel", "peer", p.opts.ID, "channel", channelID)

	// Build peer channel remove command
	peerBinary, err := p.findPeerBinary()
	if err != nil {
		return fmt.Errorf("failed to find peer binary: %w", err)
	}
	peerPath := p.getPeerPath()
	peerConfigPath := filepath.Join(peerPath, "config")
	cmd := exec.Command(peerBinary, "node", "unjoin", "-c", channelID)
	listenAddress := strings.Replace(p.opts.ListenAddress, "0.0.0.0", "localhost", 1)

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CORE_PEER_MSPCONFIGPATH=%s", peerConfigPath),
		fmt.Sprintf("CORE_PEER_ADDRESS=%s", listenAddress),
		fmt.Sprintf("CORE_PEER_LOCALMSPID=%s", p.mspID),
		"CORE_PEER_TLS_ENABLED=true",
		fmt.Sprintf("FABRIC_CFG_PATH=%s", peerConfigPath),
	)

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove channel: %w, output: %s", err, string(output))
	}

	p.logger.Info("Successfully removed channel", "peer", p.opts.ID, "channel", channelID)
	_, err = p.Start()
	if err != nil {
		return fmt.Errorf("failed to start peer: %w", err)
	}
	return nil
}
func (p *LocalPeer) GetPeerURL() string {
	return fmt.Sprintf("grpcs://%s", p.opts.ExternalEndpoint)
}

func (p *LocalPeer) GetPeerAddress() string {
	return p.opts.ExternalEndpoint
}

func (p *LocalPeer) GetTLSRootCACert(ctx context.Context) (string, error) {
	tlsCAKeyDB, err := p.keyService.GetKey(ctx, int(p.org.TlsRootKeyID.Int64))
	if err != nil {
		return "", fmt.Errorf("failed to get TLS CA key: %w", err)
	}
	if tlsCAKeyDB.Certificate == nil {
		return "", fmt.Errorf("TLS CA key is not set")
	}
	return *tlsCAKeyDB.Certificate, nil
}

func (p *LocalPeer) GetSignRootCACert(ctx context.Context) (string, error) {
	signCAKeyDB, err := p.keyService.GetKey(ctx, int(p.org.SignKeyID.Int64))
	if err != nil {
		return "", fmt.Errorf("failed to get TLS CA key: %w", err)
	}
	if signCAKeyDB.Certificate == nil {
		return "", fmt.Errorf("TLS CA key is not set")
	}
	return *signCAKeyDB.Certificate, nil
}

type SaveChannelConfigResponse struct {
	TransactionID string
}

func (p *LocalPeer) SaveChannelConfig(ctx context.Context, channelID string, ordererUrl string, ordererTlsCACert string, channelData *cb.Envelope) (*SaveChannelConfigResponse, error) {
	adminIdentity, _, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin identity: %w", err)
	}
	envelope, err := SignConfigTx(channelID, channelData, adminIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to set anchor peers: %w", err)
	}

	ordererConn, err := p.CreateOrdererConnection(ctx, ordererUrl, ordererTlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create orderer connection: %w", err)
	}
	defer ordererConn.Close()
	ordererClient, err := orderer.NewAtomicBroadcastClient(ordererConn).Broadcast(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create orderer client: %w", err)
	}
	err = ordererClient.Send(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to send envelope: %w", err)
	}
	response, err := ordererClient.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}
	return &SaveChannelConfigResponse{
		TransactionID: response.String(),
	}, nil
}

// SaveChannelConfigResponse contains the transaction ID of the saved channel configuration

// CreateOrdererConnection establishes a gRPC connection to an orderer
func (p *LocalPeer) CreateOrdererConnection(ctx context.Context, ordererURL string, ordererTLSCACert string) (*grpc.ClientConn, error) {
	p.logger.Info("Creating orderer connection",
		"ordererURL", ordererURL)

	// Create a network node with the orderer details
	networkNode := network.Node{
		Addr:          ordererURL,
		TLSCACertByte: []byte(ordererTLSCACert),
	}

	// Establish connection to the orderer
	ordererConn, err := network.DialConnection(networkNode)
	if err != nil {
		return nil, fmt.Errorf("failed to dial orderer connection: %w", err)
	}

	return ordererConn, nil
}

const (
	msgVersion = int32(0)
	epoch      = 0
)

func SignConfigTx(channelID string, envConfigUpdate *cb.Envelope, signer identity.SigningIdentity) (*cb.Envelope, error) {
	payload, err := protoutil.UnmarshalPayload(envConfigUpdate.Payload)
	if err != nil {
		return nil, errors.New("bad payload")
	}

	if payload.Header == nil || payload.Header.ChannelHeader == nil {
		return nil, errors.New("bad header")
	}

	ch, err := protoutil.UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return nil, errors.New("could not unmarshall channel header")
	}

	if ch.Type != int32(cb.HeaderType_CONFIG_UPDATE) {
		return nil, errors.New("bad type")
	}

	if ch.ChannelId == "" {
		return nil, errors.New("empty channel id")
	}

	configUpdateEnv, err := protoutil.UnmarshalConfigUpdateEnvelope(payload.Data)
	if err != nil {
		return nil, errors.New("bad config update env")
	}

	sigHeader, err := protoutil.NewSignatureHeader(signer)
	if err != nil {
		return nil, err
	}

	configSig := &cb.ConfigSignature{
		SignatureHeader: protoutil.MarshalOrPanic(sigHeader),
	}

	configSig.Signature, err = signer.Sign(Concatenate(configSig.SignatureHeader, configUpdateEnv.ConfigUpdate))
	if err != nil {
		return nil, err
	}

	configUpdateEnv.Signatures = append(configUpdateEnv.Signatures, configSig)

	return protoutil.CreateSignedEnvelope(cb.HeaderType_CONFIG_UPDATE, channelID, signer, configUpdateEnv, msgVersion, epoch)
}
func Concatenate[T any](slices ...[]T) []T {
	size := 0
	for _, slice := range slices {
		size += len(slice)
	}

	result := make([]T, size)
	i := 0
	for _, slice := range slices {
		copy(result[i:], slice)
		i += len(slice)
	}

	return result
}

// CreatePeerConnection establishes a gRPC connection to a peer
func (p *LocalPeer) CreatePeerConnection(ctx context.Context, peerURL string, peerTLSCACert string) (*grpc.ClientConn, error) {
	// Create a temporary file for the TLS CA certificate

	networkNode := network.Node{
		Addr:          peerURL,
		TLSCACertByte: []byte(peerTLSCACert),
	}
	peerConn, err := network.DialConnection(networkNode)
	if err != nil {
		return nil, fmt.Errorf("failed to dial peer connection: %w", err)
	}
	return peerConn, nil
}

func (p *LocalPeer) GetMSPID() string {
	return p.mspID
}
func (p *LocalPeer) GetAdminIdentity(ctx context.Context) (identity.SigningIdentity, gwidentity.Sign, error) {
	adminSignKeyDB, err := p.keyService.GetKey(ctx, int(p.org.AdminSignKeyID.Int64))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get TLS CA key: %w", err)
	}
	if adminSignKeyDB.Certificate == nil {
		return nil, nil, fmt.Errorf("TLS CA key is not set")
	}
	certificate := *adminSignKeyDB.Certificate
	privateKey, err := p.keyService.GetDecryptedPrivateKey(int(p.org.AdminSignKeyID.Int64))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get decrypted private key: %w", err)
	}

	cert, err := gwidentity.CertificateFromPEM([]byte(certificate))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	priv, err := gwidentity.PrivateKeyFromPEM([]byte(privateKey))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signingIdentity, err := identity.NewPrivateKeySigningIdentity(p.mspID, cert, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create signing identity: %w", err)
	}

	signer, err := gwidentity.NewPrivateKeySign(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create signer: %w", err)
	}
	return signingIdentity, signer, nil
}

// Add this struct near the top with other type definitions
type GetChannelConfigResponse struct {
	ChannelGroup *cb.Config
}

// Add this new method to the LocalPeer struct
func (p *LocalPeer) GetChannelBlock(ctx context.Context, channelID string, ordererUrl string, ordererTlsCACert string) (*cb.Block, error) {
	p.logger.Info("Fetching channel config",
		"peer", p.opts.ID,
		"channel", channelID,
		"ordererUrl", ordererUrl)

	// Get admin identity
	adminIdentity, _, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin identity: %w", err)
	}
	peerUrl := p.GetPeerAddress()
	peerTLSCACert, err := p.GetTLSRootCACert(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA cert: %w", err)
	}
	peerConn, err := p.CreatePeerConnection(ctx, peerUrl, peerTLSCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConn.Close()
	// Fetch channel configuration
	configBlock, err := channel.GetConfigBlock(ctx, peerConn, adminIdentity, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to query channel config: %w", err)
	}
	return configBlock, nil

}

// getOrdererTLSKeyPair creates a TLS key pair for secure communication with the orderer
func (p *LocalPeer) getOrdererTLSKeyPair(ctx context.Context, ordererTLSCert string) (tls.Certificate, error) {
	// Get organization details
	org, err := p.orgService.GetOrganizationByMspID(ctx, p.mspID)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to get organization: %w", err)
	}

	if !org.AdminSignKeyID.Valid {
		return tls.Certificate{}, fmt.Errorf("organization has no admin sign key")
	}

	// Get private key from key management service
	privateKeyPEM, err := p.keyService.GetDecryptedPrivateKey(int(org.AdminSignKeyID.Int64))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to get private key: %w", err)
	}

	// Parse the orderer TLS certificate
	ordererTLSCertParsed, err := tls.X509KeyPair([]byte(ordererTLSCert), []byte(privateKeyPEM))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to parse orderer TLS certificate: %w", err)
	}

	return ordererTLSCertParsed, nil
}

// Add this new method to the LocalPeer struct
func (p *LocalPeer) GetChannelConfig(ctx context.Context, channelID string, ordererUrl string, ordererTlsCACert string) (*GetChannelConfigResponse, error) {

	// Fetch channel configuration
	configBlock, err := p.GetChannelBlock(ctx, channelID, ordererUrl, ordererTlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to query channel config: %w", err)
	}

	cmnConfig, err := ExtractConfigFromBlock(configBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to extract config from block: %w", err)
	}
	return &GetChannelConfigResponse{
		ChannelGroup: cmnConfig,
	}, nil
}

// SaveChannelConfigWithSignaturesResponse represents the response from saving a channel config with signatures
type SaveChannelConfigWithSignaturesResponse struct {
	TransactionID string
}

// SaveChannelConfigWithSignatures submits a config update envelope with signatures to the orderer
func (p *LocalPeer) SaveChannelConfigWithSignatures(
	ctx context.Context,
	channelID string,
	ordererUrl string,
	ordererTlsCACert string,
	envelopeBytes []byte,
	signatures [][]byte,
) (*SaveChannelConfigWithSignaturesResponse, error) {
	var cbEnvelope *cb.Envelope
	if err := proto.Unmarshal(envelopeBytes, cbEnvelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal envelope: %w", err)
	}

	signedEnvelope, err := protoutil.FormSignedEnvelope(cb.HeaderType_CONFIG_UPDATE, channelID, cbEnvelope, signatures, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to form signed envelope: %w", err)
	}

	ordererConn, err := p.CreateOrdererConnection(ctx, ordererUrl, ordererTlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create orderer connection: %w", err)
	}
	defer ordererConn.Close()
	ordererClient, err := orderer.NewAtomicBroadcastClient(ordererConn).Broadcast(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create orderer client: %w", err)
	}

	err = ordererClient.Send(signedEnvelope)
	if err != nil {
		return nil, fmt.Errorf("failed to save channel config with signatures: %w", err)
	}
	response, err := ordererClient.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}
	return &SaveChannelConfigWithSignaturesResponse{
		TransactionID: response.String(),
	}, nil
}

type PeerChannel struct {
	Name      string    `json:"name"`
	BlockNum  int64     `json:"blockNum"`
	CreatedAt time.Time `json:"createdAt"`
}

// GetChannels returns a list of channels the peer has joined
func (p *LocalPeer) GetChannels(ctx context.Context) ([]PeerChannel, error) {
	peerUrl := p.GetPeerAddress()
	tlsCACert, err := p.GetTLSRootCACert(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA cert: %w", err)
	}
	peerConn, err := p.CreatePeerConnection(ctx, peerUrl, tlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConn.Close()
	adminIdentity, _, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin identity: %w", err)
	}

	channelList, err := channel.ListChannelOnPeer(ctx, peerConn, adminIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to list channels on peer: %w", err)
	}

	channels := make([]PeerChannel, len(channelList))
	for i, channel := range channelList {
		blockInfo, err := p.GetChannelInfoOnPeer(ctx, channel.ChannelId)
		if err != nil {
			channels[i] = PeerChannel{
				Name:      channel.ChannelId,
				BlockNum:  0,
				CreatedAt: time.Now(),
			}
		} else {
			channels[i] = PeerChannel{
				Name:      channel.ChannelId,
				BlockNum:  int64(blockInfo.Height),
				CreatedAt: time.Now(),
			}
		}
	}
	return channels, nil
}

// getChannelBlockInfo gets the current block height for a channel
func (p *LocalPeer) getChannelBlockInfo(ctx context.Context, channelID string) (*cb.BlockchainInfo, error) {
	peerUrl := p.GetPeerAddress()
	tlsCACert, err := p.GetTLSRootCACert(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA cert: %w", err)
	}
	peerConn, err := p.CreatePeerConnection(ctx, peerUrl, tlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConn.Close()
	adminIdentity, _, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin identity: %w", err)
	}

	// Query info for the channel
	channelInfo, err := channel.GetBlockChainInfo(ctx, peerConn, adminIdentity, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to query channel info: %w", err)
	}

	// Get the block number from the channel info

	return channelInfo, nil
}

// ExtractConfigFromBlock extracts channel configuration from block
func ExtractConfigFromBlock(block *cb.Block) (*cb.Config, error) {
	if block == nil || block.Data == nil || len(block.Data.Data) == 0 {
		return nil, errors.New("invalid block")
	}
	blockPayload := block.Data.Data[0]

	envelope := &cb.Envelope{}
	if err := proto.Unmarshal(blockPayload, envelope); err != nil {
		return nil, err
	}
	payload := &cb.Payload{}
	if err := proto.Unmarshal(envelope.Payload, payload); err != nil {
		return nil, err
	}

	cfgEnv := &cb.ConfigEnvelope{}
	if err := proto.Unmarshal(payload.Data, cfgEnv); err != nil {
		return nil, err
	}
	return cfgEnv.Config, nil
}

// GetBlock retrieves a specific block by its number
func (p *LocalPeer) GetBlock(ctx context.Context, channelID string, blockNum uint64) (*cb.Block, error) {
	peerUrl := p.GetPeerAddress()
	tlsCACert, err := p.GetTLSRootCACert(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA cert: %w", err)
	}
	peerConn, err := p.CreatePeerConnection(ctx, peerUrl, tlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConn.Close()
	adminIdentity, signer, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin identity: %w", err)
	}
	gateway, err := client.Connect(adminIdentity, client.WithClientConnection(peerConn), client.WithSign(signer))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gateway: %w", err)
	}
	defer gateway.Close()
	network := gateway.GetNetwork(channelID)

	// Create a cancellable context derived from the parent context
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel() // Ensure we cancel the context when we're done

	blockEvents, err := network.BlockAndPrivateDataEvents(ctxWithCancel, client.WithStartBlock(blockNum))
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	for blockEvent := range blockEvents {
		// Cancel the context after we get the first block
		cancel()
		return blockEvent.Block, nil
	}
	return nil, fmt.Errorf("block not found")
}
func (p *LocalPeer) GetBlockTransactions(ctx context.Context, channelID string, blockNum uint64) ([]*cb.Envelope, error) {
	peerUrl := p.GetPeerAddress()
	tlsCACert, err := p.GetTLSRootCACert(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA cert: %w", err)
	}
	peerConn, err := p.CreatePeerConnection(ctx, peerUrl, tlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConn.Close()
	adminIdentity, signer, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin identity: %w", err)
	}
	gateway, err := client.Connect(adminIdentity, client.WithClientConnection(peerConn), client.WithSign(signer))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gateway: %w", err)
	}
	defer gateway.Close()
	network := gateway.GetNetwork(channelID)

	// Create a cancellable context derived from the parent context with a 10 second timeout
	ctxWithTimeout, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	ctxWithCancel, cancel := context.WithCancel(ctxWithTimeout)
	defer cancel()        // Ensure we cancel the context when we're done
	defer cancelTimeout() // Ensure we cancel the timeout context when we're done

	blockEvents, err := network.BlockAndPrivateDataEvents(ctxWithCancel, client.WithStartBlock(blockNum))
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	for blockEvent := range blockEvents {
		// Cancel the context after we get the first block
		cancel()

		var json bytes.Buffer
		if err := protolator.DeepMarshalJSON(&json, blockEvent.Block); err != nil {
			return nil, fmt.Errorf("failed to marshal block: %w", err)
		}
		var transactions []*cb.Envelope
		for _, data := range blockEvent.Block.Data.Data {
			envelope := &cb.Envelope{}
			if err := proto.Unmarshal(data, envelope); err != nil {
				return nil, fmt.Errorf("failed to unmarshal transaction envelope: %w", err)
			}
			transactions = append(transactions, envelope)
		}

		return transactions, nil
	}

	// Check if we timed out
	if ctxWithTimeout.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("timed out waiting for block after 10 seconds")
	}

	return nil, fmt.Errorf("block not found")
}

// GetBlocksInRange retrieves blocks from startBlock to endBlock (inclusive)
func (p *LocalPeer) GetBlocksInRange(ctx context.Context, channelID string, startBlock, endBlock uint64) ([]*cb.Block, error) {
	peerUrl := p.GetPeerAddress()
	tlsCACert, err := p.GetTLSRootCACert(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA cert: %w", err)
	}
	peerConn, err := p.CreatePeerConnection(ctx, peerUrl, tlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConn.Close()
	adminIdentity, signer, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin identity: %w", err)
	}
	gateway, err := client.Connect(adminIdentity, client.WithClientConnection(peerConn), client.WithSign(signer))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gateway: %w", err)
	}
	defer gateway.Close()

	network := gateway.GetNetwork(channelID)
	blockEvents, err := network.BlockEvents(ctx, client.WithStartBlock(startBlock))
	if err != nil {
		return nil, fmt.Errorf("failed to get blocks: %w", err)
	}

	var blocks []*cb.Block
	blockCount := uint64(0)
	maxBlocks := endBlock - startBlock + 1

	for blockEvent := range blockEvents {
		blocks = append(blocks, blockEvent)
		blockCount++

		if blockCount >= maxBlocks || blockEvent.Header.Number >= endBlock {
			break
		}
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("no blocks found in range %d to %d", startBlock, endBlock)
	}

	return blocks, nil
}

// GetChannelBlockInfo retrieves information about the blockchain for a specific channel
func (p *LocalPeer) GetChannelBlockInfo(ctx context.Context, channelID string) (*BlockInfo, error) {
	peerUrl := p.GetPeerAddress()
	tlsCACert, err := p.GetTLSRootCACert(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA cert: %w", err)
	}

	peerConn, err := p.CreatePeerConnection(ctx, peerUrl, tlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConn.Close()

	adminIdentity, _, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin identity: %w", err)
	}
	blockInfo, err := channel.GetBlockChainInfo(ctx, peerConn, adminIdentity, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get block chain info: %w", err)
	}

	return &BlockInfo{
		Height:            blockInfo.Height,
		CurrentBlockHash:  fmt.Sprintf("%x", blockInfo.CurrentBlockHash),
		PreviousBlockHash: fmt.Sprintf("%x", blockInfo.PreviousBlockHash),
	}, nil

}

const (
	qscc                = "qscc"
	qsccTransactionByID = "GetTransactionByID"
	qsccChannelInfo     = "GetChainInfo"
	qsccBlockByHash     = "GetBlockByHash"
	qsccBlockByNumber   = "GetBlockByNumber"
	qsccBlockByTxID     = "GetBlockByTxID"
)

// GetBlockByTxID retrieves a block containing the specified transaction ID
func (p *LocalPeer) GetBlockByTxID(ctx context.Context, channelID string, txID string) (*cb.Block, error) {
	peerUrl := p.GetPeerAddress()
	tlsCACert, err := p.GetTLSRootCACert(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA cert: %w", err)
	}

	peerConn, err := p.CreatePeerConnection(ctx, peerUrl, tlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConn.Close()

	adminIdentity, signer, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin identity: %w", err)
	}

	gateway, err := client.Connect(adminIdentity, client.WithClientConnection(peerConn), client.WithSign(signer))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gateway: %w", err)
	}
	defer gateway.Close()
	network := gateway.GetNetwork(channelID)
	contract := network.GetContract(qscc)
	response, err := contract.EvaluateTransaction(qsccBlockByTxID, channelID, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to query block by transaction ID: %w", err)
	}

	// Unmarshal block
	block := &cb.Block{}
	if err := proto.Unmarshal(response, block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return block, nil
}

// GetBlockByTxID retrieves a block containing the specified transaction ID
func (p *LocalPeer) GetChannelInfoOnPeer(ctx context.Context, channelID string) (*cb.BlockchainInfo, error) {
	peerUrl := p.GetPeerAddress()
	tlsCACert, err := p.GetTLSRootCACert(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA cert: %w", err)
	}

	peerConn, err := p.CreatePeerConnection(ctx, peerUrl, tlsCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConn.Close()

	adminIdentity, signer, err := p.GetAdminIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin identity: %w", err)
	}

	gateway, err := client.Connect(adminIdentity, client.WithClientConnection(peerConn), client.WithSign(signer))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gateway: %w", err)
	}
	defer gateway.Close()
	network := gateway.GetNetwork(channelID)
	contract := network.GetContract(qscc)
	response, err := contract.EvaluateTransaction(qsccChannelInfo, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to query block by transaction ID: %w", err)
	}
	p.logger.Info("Channel info", "response", response)
	bci := &cb.BlockchainInfo{}
	if err := proto.Unmarshal(response, bci); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block chain info: %w", err)
	}
	return bci, nil
}

// SynchronizeConfig synchronizes the peer's configuration files and service
func (p *LocalPeer) SynchronizeConfig(deployConfig *nodetypes.FabricPeerDeploymentConfig) error {
	slugifiedID := strings.ReplaceAll(strings.ToLower(p.opts.ID), " ", "-")
	dirPath := filepath.Join(p.configService.GetDataPath(), "peers", slugifiedID)
	mspConfigPath := filepath.Join(dirPath, "config")
	dataConfigPath := filepath.Join(dirPath, "data")
	// Write config.yaml
	if err := os.WriteFile(filepath.Join(mspConfigPath, "config.yaml"), []byte(configYamlContent), 0644); err != nil {
		return fmt.Errorf("failed to write config.yaml: %w", err)
	}
	convertedOverrides, err := p.convertAddressOverrides(mspConfigPath, deployConfig.AddressOverrides)
	if err != nil {
		return fmt.Errorf("failed to convert address overrides: %w", err)
	}

	// Define template data
	data := struct {
		PeerID                  string
		ListenAddress           string
		ChaincodeAddress        string
		ExternalEndpoint        string
		DataPath                string
		MSPID                   string
		ExternalBuilderPath     string
		OperationsListenAddress string
		AddressOverrides        []AddressOverridePath
	}{
		PeerID:                  p.opts.ID,
		ListenAddress:           deployConfig.ListenAddress,
		ChaincodeAddress:        deployConfig.ChaincodeAddress,
		ExternalEndpoint:        deployConfig.ExternalEndpoint,
		DataPath:                dataConfigPath,
		MSPID:                   deployConfig.MSPID,
		ExternalBuilderPath:     filepath.Join(mspConfigPath, "ccaas"),
		OperationsListenAddress: deployConfig.OperationsListenAddress,
		AddressOverrides:        convertedOverrides,
	}
	// Create template
	tmpl, err := template.New("core.yaml").Parse(coreYamlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse core.yaml template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute core.yaml template: %w", err)
	}

	// Write core.yaml
	if err := os.WriteFile(filepath.Join(mspConfigPath, "core.yaml"), buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write core.yaml: %w", err)
	}

	// Stop the peer if it's running
	if err := p.Stop(); err != nil {
		return fmt.Errorf("failed to stop peer before regenerating config: %w", err)
	}

	// Start the peer to regenerate config files
	if _, err := p.Start(); err != nil {
		return fmt.Errorf("failed to restart peer after regenerating config: %w", err)
	}
	return nil
}

// Add this new function
func (p *LocalPeer) convertAddressOverrides(mspConfigPath string, overrides []nodetypes.AddressOverride) ([]AddressOverridePath, error) {
	// Create temporary directory for override certificates
	tmpDir := filepath.Join(mspConfigPath, "orderer-overrides")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create orderer overrides directory: %w", err)
	}

	var convertedOverrides []AddressOverridePath
	for i, override := range overrides {
		// Write TLS CA certificate to file
		certPath := filepath.Join(tmpDir, fmt.Sprintf("tlsca-%d.pem", i))
		if err := os.WriteFile(certPath, []byte(override.TLSCACert), 0644); err != nil {
			return nil, fmt.Errorf("failed to write orderer TLS CA certificate: %w", err)
		}

		// Add converted override
		convertedOverrides = append(convertedOverrides, AddressOverridePath{
			From:      override.From,
			To:        override.To,
			TLSCAPath: certPath,
		})
	}

	return convertedOverrides, nil
}
