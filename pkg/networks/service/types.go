package service

import (
	"time"
)

type ImportNetworkParams struct {
	NetworkType string
	GenesisFile []byte
	Name        string
	Description string
	ChainID     *int64
}

type ImportNetworkResult struct {
	NetworkID int64
	Message   string
}

// JoinPeerRequest represents a request to join a peer to a network
type JoinPeerRequest struct {
	PeerID  int64  `json:"peer_id" validate:"required"`
	OrgName string `json:"org_name" validate:"required"`
}

// JoinPeerResponse represents a response from joining a peer to a network
type JoinPeerResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	NetworkID int64  `json:"network_id"`
	PeerID    int64  `json:"peer_id"`
}

// JoinOrdererRequest represents a request to join an orderer to a network
type JoinOrdererRequest struct {
	OrdererID int64  `json:"orderer_id" validate:"required"`
	OrgName   string `json:"org_name" validate:"required"`
}

// JoinOrdererResponse represents a response from joining an orderer to a network
type JoinOrdererResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	NetworkID int64  `json:"network_id"`
	OrdererID int64  `json:"orderer_id"`
}

// RemovePeerRequest represents a request to remove a peer from a network
type RemovePeerRequest struct {
	PeerID int64 `json:"peer_id" validate:"required"`
}

// RemovePeerResponse represents a response from removing a peer from a network
type RemovePeerResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	NetworkID int64  `json:"network_id"`
	PeerID    int64  `json:"peer_id"`
}

// RemoveOrdererRequest represents a request to remove an orderer from a network
type RemoveOrdererRequest struct {
	OrdererID int64 `json:"orderer_id" validate:"required"`
}

// RemoveOrdererResponse represents a response from removing an orderer from a network
type RemoveOrdererResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	NetworkID int64  `json:"network_id"`
	OrdererID int64  `json:"orderer_id"`
}

type TxType string

const (
	MESSAGE              TxType = "MESSAGE"
	CONFIG               TxType = "CONFIG"
	CONFIG_UPDATE        TxType = "CONFIG_UPDATE"
	ENDORSER_TRANSACTION TxType = "ENDORSER_TRANSACTION"
	ORDERER_TRANSACTION  TxType = "ORDERER_TRANSACTION"
	DELIVER_SEEK_INFO    TxType = "DELIVER_SEEK_INFO"
	CHAINCODE_PACKAGE    TxType = "CHAINCODE_PACKAGE"
)

type Transaction struct {
	ID          string
	Type        TxType
	ChannelID   string
	CreatedAt   time.Time
	ChaincodeID string
	Version     string
	Path        string
	Response    []byte
	Request     []byte
	Event       TransactionEvent
	Writes      []*TransactionWrite
	Reads       []*TransactionRead
}
type TransactionEvent struct {
	Name  string
	Value string
}
type TransactionWrite struct {
	ChaincodeID string
	Deleted     bool
	Key         string
	Value       string
}
type TransactionRead struct {
	ChaincodeID     string
	Key             string
	BlockNumVersion int
	TxNumVersion    int
}
type Block struct {
	Number       int
	DataHash     string
	Transactions []*Transaction
	CreatedAt    *time.Time
}

type ChainInfo struct {
	Height            uint64
	CurrentBlockHash  string
	PreviousBlockHash string
}
