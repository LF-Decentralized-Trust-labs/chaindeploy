package block

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hyperledger/fabric-config/protolator"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"

	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

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
	ID          string              `json:"id"`
	Type        TxType              `json:"type"`
	ChannelID   string              `json:"channelId"`
	CreatedAt   time.Time           `json:"createdAt"`
	ChaincodeID string              `json:"chaincodeId"`
	Version     string              `json:"version"`
	Path        string              `json:"path"`
	Response    []byte              `json:"response"`
	Request     []byte              `json:"request"`
	Event       TransactionEvent    `json:"event"`
	Writes      []*TransactionWrite `json:"writes"`
	Reads       []*TransactionRead  `json:"reads"`
}
type TransactionEvent struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type TransactionWrite struct {
	ChaincodeID string `json:"chaincodeId"`
	Deleted     bool   `json:"deleted"`
	Key         string `json:"key"`
	Value       string `json:"value"`
}
type TransactionRead struct {
	ChaincodeID     string `json:"chaincodeId"`
	Key             string `json:"key"`
	BlockNumVersion int    `json:"blockNumVersion"`
	TxNumVersion    int    `json:"txNumVersion"`
}
type Block struct {
	Number       int            `json:"number"`
	DataHash     string         `json:"dataHash"`
	Transactions []*Transaction `json:"transactions"`
	CreatedAt    *time.Time     `json:"createdAt"`
	Base64Data   string         `json:"base64Data"`
}

func UnmarshalTransaction(txBytes []byte) (*peer.Transaction, error) {
	tx := &peer.Transaction{}
	err := proto.Unmarshal(txBytes, tx)
	return tx, errors.Wrap(err, "error unmarshaling Transaction")
}
func UnmarshalChannelHeader(bytes []byte) (*common.ChannelHeader, error) {
	chdr := &common.ChannelHeader{}
	err := proto.Unmarshal(bytes, chdr)
	return chdr, errors.Wrap(err, "error unmarshaling ChannelHeader")
}

func MapBlock(block *common.Block) (*Block, error) {
	var buf bytes.Buffer
	err := protolator.DeepMarshalJSON(&buf, block)
	if err != nil {
		return nil, err
	}
	blk := &Block{
		Number:     int(block.Header.Number),
		DataHash:   hex.EncodeToString(block.Header.DataHash),
		Base64Data: base64.StdEncoding.EncodeToString(buf.Bytes()),
	}

	blk.Transactions = []*Transaction{}
	for _, txData := range block.Data.Data {
		transaction := &Transaction{}
		tx, err := UnmarshalTransaction(txData)
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Tx ID %d ", len(tx.Actions))
		env := &common.Envelope{}
		err = proto.Unmarshal(txData, env)
		if err != nil {
			return nil, err
		}

		payload := &common.Payload{}
		err = proto.Unmarshal(env.Payload, payload)
		if err != nil {
			return nil, err
		}
		chdr, err := UnmarshalChannelHeader(payload.Header.ChannelHeader)
		if err != nil {
			return nil, err
		}
		channelHeader := &common.ChannelHeader{}
		if err := proto.Unmarshal(payload.Header.ChannelHeader, channelHeader); err != nil {
			return nil, errors.Wrap(err, "unmarshal payload from envelope failed")
		}
		transaction.ID = channelHeader.TxId
		transaction.ChannelID = chdr.ChannelId
		txDate, err := ptypes.Timestamp(chdr.Timestamp)
		if err != nil {
			return nil, err
		}
		transaction.CreatedAt = txDate
		if blk.CreatedAt == nil {
			blk.CreatedAt = &transaction.CreatedAt
		}
		var txType TxType
		switch common.HeaderType(chdr.Type) {
		case common.HeaderType_MESSAGE:
			txType = MESSAGE
			logrus.Debugf("HeaderType_MESSAGE")
		case common.HeaderType_CONFIG:
			txType = CONFIG
			logrus.Debugf("HeaderType_CONFIG")
		case common.HeaderType_CONFIG_UPDATE:
			txType = CONFIG_UPDATE
			logrus.Debugf("HeaderType_CONFIG_UPDATE")
		case common.HeaderType_ENDORSER_TRANSACTION:
			txType = ENDORSER_TRANSACTION
			logrus.Debugf("HeaderType_ENDORSER_TRANSACTION")
			logrus.Debugf("Header type %d", chdr.Type)
			action, err := GetActionFromEnvelopeMsg(env)
			if err != nil {
				logrus.Debugf("Failed to get action %v", err)
			} else {
				events, err := UnmarshalChaincodeEvents(action.GetEvents())
				if err != nil {
					return nil, err
				}
				transaction.Event = TransactionEvent{
					Name:  events.EventName,
					Value: string(events.Payload),
				}
				transaction.ChaincodeID = action.ChaincodeId.Name
				transaction.Version = action.ChaincodeId.Version
				transaction.Path = action.ChaincodeId.Path
				transaction.Response = action.Response.Payload
				transaction.Request = action.Results
				txRWSet := &TxRwSet{}
				err = txRWSet.FromProtoBytes(action.Results)
				if err != nil {
					return nil, err
				}
				for _, set := range txRWSet.NsRwSets {
					logrus.Debugf("Set %v", set)
				}
				var writes []*TransactionWrite
				var reads []*TransactionRead
				for _, set := range txRWSet.NsRwSets {
					chaincodeID := set.NameSpace

					for _, rw := range set.KvRwSet.Writes {
						write := &TransactionWrite{
							ChaincodeID: chaincodeID,
							Deleted:     rw.IsDelete,
							Key:         rw.Key,
							Value:       string(rw.Value),
						}
						writes = append(writes, write)
					}

					for _, rw := range set.KvRwSet.Reads {

						read := &TransactionRead{
							ChaincodeID: chaincodeID,
							Key:         rw.Key,
						}
						if rw.Version != nil {
							read.BlockNumVersion = int(rw.Version.BlockNum)
							read.TxNumVersion = int(rw.Version.TxNum)
						}
						reads = append(reads, read)
					}

				}
				transaction.Writes = writes
				transaction.Reads = reads
			}
		case common.HeaderType_ORDERER_TRANSACTION:
			txType = ORDERER_TRANSACTION
			logrus.Debugf("HeaderType_ORDERER_TRANSACTION")
		case common.HeaderType_DELIVER_SEEK_INFO:
			txType = DELIVER_SEEK_INFO
			logrus.Debugf("HeaderType_DELIVER_SEEK_INFO")
		case common.HeaderType_CHAINCODE_PACKAGE:
			txType = CHAINCODE_PACKAGE
			logrus.Debugf("HeaderType_CHAINCODE_PACKAGE")
		}
		transaction.Type = txType
		channelID, err := GetChannelIDFromBlock(block)
		if err != nil {
			return nil, err
		}
		//block.Data.Data
		logrus.Debugf("Channel ID %s Data hash %s", channelID, hex.EncodeToString(block.Header.DataHash))
		blk.Transactions = append(blk.Transactions, transaction)
	}
	return blk, nil
}
func GetEnvelopeFromBlock(data []byte) (*common.Envelope, error) {
	// Block always begins with an envelope
	var err error
	env := &common.Envelope{}
	if err = proto.Unmarshal(data, env); err != nil {
		return nil, errors.Wrap(err, "error unmarshaling Envelope")
	}

	return env, nil
}

// GetChannelIDFromBlock returns channel ID in the block
func GetChannelIDFromBlock(block *common.Block) (string, error) {
	if block == nil || block.Data == nil || block.Data.Data == nil || len(block.Data.Data) == 0 {
		return "", errors.New("failed to retrieve channel id - block is empty")
	}
	var err error
	envelope, err := GetEnvelopeFromBlock(block.Data.Data[0])
	if err != nil {
		return "", err
	}
	payload, err := UnmarshalPayload(envelope.Payload)
	if err != nil {
		return "", err
	}

	if payload.Header == nil {
		return "", errors.New("failed to retrieve channel id - payload header is empty")
	}
	chdr, err := UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return "", err
	}

	return chdr.ChannelId, nil
}
func UnmarshalPayload(encoded []byte) (*common.Payload, error) {
	payload := &common.Payload{}
	err := proto.Unmarshal(encoded, payload)
	return payload, errors.Wrap(err, "error unmarshaling Payload")
}
func GetActionFromEnvelopeMsg(env *common.Envelope) (*peer.ChaincodeAction, error) {
	payl, err := UnmarshalPayload(env.Payload)
	if err != nil {
		return nil, err
	}

	tx, err := UnmarshalTransaction(payl.Data)
	if err != nil {
		return nil, err
	}

	if len(tx.Actions) == 0 {
		return nil, errors.New("at least one TransactionAction required")
	}

	_, respPayload, err := GetPayloads(tx.Actions[0])
	return respPayload, err
}

// GetPayloads gets the underlying payload objects in a TransactionAction
func GetPayloads(txActions *peer.TransactionAction) (*peer.ChaincodeActionPayload, *peer.ChaincodeAction, error) {
	// TODO: pass in the tx type (in what follows we're assuming the
	// type is ENDORSER_TRANSACTION)
	ccPayload, err := UnmarshalChaincodeActionPayload(txActions.Payload)
	if err != nil {
		return nil, nil, err
	}

	if ccPayload.Action == nil || ccPayload.Action.ProposalResponsePayload == nil {
		return nil, nil, errors.New("no payload in ChaincodeActionPayload")
	}
	pRespPayload, err := UnmarshalProposalResponsePayload(ccPayload.Action.ProposalResponsePayload)
	if err != nil {
		return nil, nil, err
	}

	if pRespPayload.Extension == nil {
		return nil, nil, errors.New("response payload is missing extension")
	}

	respPayload, err := UnmarshalChaincodeAction(pRespPayload.Extension)
	if err != nil {
		return ccPayload, nil, err
	}
	return ccPayload, respPayload, nil
}
func UnmarshalProposalResponsePayload(prpBytes []byte) (*peer.ProposalResponsePayload, error) {
	prp := &peer.ProposalResponsePayload{}
	err := proto.Unmarshal(prpBytes, prp)
	return prp, errors.Wrap(err, "error unmarshaling ProposalResponsePayload")
}
func UnmarshalChaincodeActionPayload(capBytes []byte) (*peer.ChaincodeActionPayload, error) {
	cap := &peer.ChaincodeActionPayload{}
	err := proto.Unmarshal(capBytes, cap)
	return cap, errors.Wrap(err, "error unmarshaling ChaincodeActionPayload")
}
func UnmarshalChaincodeAction(caBytes []byte) (*peer.ChaincodeAction, error) {
	chaincodeAction := &peer.ChaincodeAction{}
	err := proto.Unmarshal(caBytes, chaincodeAction)
	return chaincodeAction, errors.Wrap(err, "error unmarshaling ChaincodeAction")
}

func UnmarshalChaincodeEvents(eBytes []byte) (*peer.ChaincodeEvent, error) {
	chaincodeEvent := &peer.ChaincodeEvent{}
	err := proto.Unmarshal(eBytes, chaincodeEvent)
	return chaincodeEvent, errors.Wrap(err, "error unmarshaling ChaicnodeEvent")
}
