package decoder

import (
	"encoding/hex"
	"fmt"
	"time"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

type DecodedBlock struct {
	Number       uint64           `json:"number"`
	DataHash     string           `json:"dataHash"`
	PreviousHash string           `json:"previousHash"`
	TxCount      int              `json:"txCount"`
	Transactions []DecodedBlockTx `json:"transactions"`
}

type DecodedBlockTx struct {
	Index     int       `json:"index"`
	TxID      string    `json:"txId"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	ChannelID string    `json:"channelId"`
}

func DecodeBlock(block *cb.Block) (*DecodedBlock, error) {
	if block == nil {
		return nil, fmt.Errorf("nil block")
	}
	header := block.GetHeader()
	db := &DecodedBlock{
		Number:       header.GetNumber(),
		DataHash:     hex.EncodeToString(header.GetDataHash()),
		PreviousHash: hex.EncodeToString(header.GetPreviousHash()),
		TxCount:      len(block.GetData().GetData()),
	}

	for i, envBytes := range block.GetData().GetData() {
		env, err := protoutil.UnmarshalEnvelope(envBytes)
		if err != nil {
			db.Transactions = append(db.Transactions, DecodedBlockTx{
				Index: i,
				Type:  "error: " + err.Error(),
			})
			continue
		}
		payload, err := protoutil.UnmarshalPayload(env.GetPayload())
		if err != nil {
			db.Transactions = append(db.Transactions, DecodedBlockTx{
				Index: i,
				Type:  "error: " + err.Error(),
			})
			continue
		}
		chdr, err := protoutil.UnmarshalChannelHeader(payload.GetHeader().GetChannelHeader())
		if err != nil {
			db.Transactions = append(db.Transactions, DecodedBlockTx{
				Index: i,
				Type:  "error: " + err.Error(),
			})
			continue
		}

		txType := "application"
		if chdr.GetTxId() == "" {
			txType = "config"
		}

		db.Transactions = append(db.Transactions, DecodedBlockTx{
			Index:     i,
			TxID:      chdr.GetTxId(),
			Type:      txType,
			Timestamp: chdr.GetTimestamp().AsTime(),
			ChannelID: chdr.GetChannelId(),
		})
	}

	return db, nil
}
