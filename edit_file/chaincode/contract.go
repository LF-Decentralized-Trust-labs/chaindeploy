// InitLedger adds a base set of assets to the ledger for testing and demonstration purposes
func (t *SimpleChaincode) InitLedger(ctx contractapi.TransactionContextInterface) error {
	log.Info().Msg("InitLedger called")

	assets := []Asset{
		{ID: "asset1", Color: "blue", Size: 5, Owner: "Tomoko", AppraisedValue: 300},
		{ID: "asset2", Color: "red", Size: 10, Owner: "Brad", AppraisedValue: 400},
		{ID: "asset3", Color: "green", Size: 15, Owner: "Jin Soo", AppraisedValue: 500},
		{ID: "asset4", Color: "yellow", Size: 7, Owner: "Max", AppraisedValue: 600},
		{ID: "asset5", Color: "black", Size: 12, Owner: "Adriana", AppraisedValue: 700},
	}

	for _, asset := range assets {
		asset.DocType = "asset"
		assetBytes, err := json.Marshal(asset)
		if err != nil {
			log.Error().Err(err).Str("assetID", asset.ID).Msg("failed to marshal asset")
			return err
		}

		err = ctx.GetStub().PutState(asset.ID, assetBytes)
		if err != nil {
			log.Error().Err(err).Str("assetID", asset.ID).Msg("failed to put state")
			return err
		}

		// Create composite key for color~name index
		colorNameIndexKey, err := ctx.GetStub().CreateCompositeKey(index, []string{asset.Color, asset.ID})
		if err != nil {
			log.Error().Err(err).Msg("failed to create composite key")
			return err
		}

		err = ctx.GetStub().PutState(colorNameIndexKey, []byte{0x00})
		if err != nil {
			log.Error().Err(err).Msg("failed to put index for asset")
			return err
		}

		log.Info().Str("assetID", asset.ID).Msg("Asset initialized")
	}

	log.Info().Msg("InitLedger completed successfully")
	return nil
}
