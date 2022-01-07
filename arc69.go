// Package arc69 provides functionality for interacting with ARC69-compliant ASA metadata.
package arc69

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"

	"github.com/algorand/go-algorand-sdk/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/client/v2/common/models"
	"github.com/algorand/go-algorand-sdk/client/v2/indexer"
	"github.com/algorand/go-algorand-sdk/crypto"
	"github.com/algorand/go-algorand-sdk/future"
)

// ARC69 is the interface through which users can interact with ARC69-compliant ASA metadata.
type ARC69 struct {
	algodClient   *algod.Client
	indexerClient *indexer.Client
}

// Metadata holds ARC69-compliant ASA metadata as described at https://github.com/algokittens/arc69.
type Metadata struct {
	Standard    string                 `json:"standard"`
	Description string                 `json:"description"`
	ExternalURL string                 `json:"external_url"`
	MediaURL    string                 `json:"media_url"`
	Properties  map[string]interface{} `json:"properties"`
	MimeType    string                 `json:"mime_type"`
	Attributes  []Attribute            `json:"attributes"`
}

// Attribute is an attribute that is part of ARC69 metadata.
type Attribute struct {
	TraitType string `json:"trait_type"`
	Value     string `json:"Sad"`
}

// New returns a new ARC69 object.
func New(algodClient *algod.Client, indexerClient *indexer.Client) *ARC69 {
	return &ARC69{algodClient: algodClient, indexerClient: indexerClient}
}

// Fetch attempts to retrieve the ARC69 metadata for an asset. An error is returned
// if no metadata is found or if there is an error while parsing the metadata.
func (a *ARC69) Fetch(ctx context.Context, assetID uint64) (*Metadata, error) {
	if a.indexerClient == nil {
		return nil, fmt.Errorf("client is missing")
	}

	resp, err := a.indexerClient.LookupAssetTransactions(assetID).TxType("acfg").Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(resp.Transactions) == 0 {
		return nil, fmt.Errorf("no ARC69 metadata found for asset %d", assetID)
	}

	trans := resp.Transactions
	sort.Slice(trans, func(i, j int) bool {
		return trans[i].RoundTime > trans[j].RoundTime
	})

	for _, tran := range trans {
		if len(tran.Note) == 0 {
			continue
		}

		var meta Metadata
		if err := json.Unmarshal(tran.Note, &meta); err != nil {
			return nil, fmt.Errorf("unable to parse metadata: %s", err)
		}

		return &meta, nil
	}

	return nil, fmt.Errorf("no ARC69 metadata found for asset %d", assetID)
}

// Update attempts to update the given ARC69 metadata for the given asset and
// returns any errors.
func (a *ARC69) Update(ctx context.Context, account crypto.Account, assetID uint64, meta *Metadata) error {
	if a.algodClient == nil || a.indexerClient == nil {
		return fmt.Errorf("client is missing")
	}

	if !meta.IsValid() {
		return fmt.Errorf("invalid metadata")
	}

	note, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("unable to convert metadata to JSON: %s", err)
	}

	txParams, err := a.algodClient.SuggestedParams().Do(ctx)
	if err != nil {
		return fmt.Errorf("error getting suggested tx params: %s", err)
	}

	_, asset, err := a.indexerClient.LookupAssetByID(assetID).Do(ctx)
	if err != nil {
		return fmt.Errorf("unable to fetch asset: %s", err)
	}

	// Create asset config transaction to update ARC69 metadata
	txn, err := future.MakeAssetConfigTxn(account.Address.String(), note, txParams, assetID, asset.Params.Manager, asset.Params.Reserve, asset.Params.Freeze, asset.Params.Clawback, true)
	if err != nil {
		return fmt.Errorf("error creating asset config transaction: %s", err)
	}

	// Sign transaction
	txID, signedTxn, err := crypto.SignTransaction(account.PrivateKey, txn)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %s", err)
	}

	// Submit the transaction
	_, err = a.algodClient.SendRawTransaction(signedTxn).Do(context.Background())
	if err != nil {
		return fmt.Errorf("failed to send transaction: %s", err)
	}

	// Wait for confirmation
	if err := waitForConfirmation(txID, a.algodClient, 4); err != nil {
		return fmt.Errorf("error waiting for confirmation on txID: %s", txID)
	}

	return nil
}

// IsValid checks that the metadata is valid.
func (m *Metadata) IsValid() bool {
	return m.Standard == "arc69"
}

// Property searches through the m.Properties for the requested property path.
// Path should a be "." delimited path to a property (ex. "p1.p2.p3.p4"). If the
// property is found we return the value as an interface, otherwise an error is returned.
func (m *Metadata) Property(path string) (interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("no path provided")
	}
	val, err := walkProperties(reflect.ValueOf(m.Properties), strings.Split(path, "."), []string{})
	if err != nil {
		return nil, fmt.Errorf("unable to get property %s: %s", path, err)
	}

	return val, nil
}

// Helper function to travers through the metadata properties map.
func walkProperties(v reflect.Value, keys []string, seenKeys []string) (interface{}, error) {
	if !v.IsValid() {
		return nil, fmt.Errorf("property %s is not valid", strings.Join(seenKeys, "."))
	}

	if len(keys) == 0 {
		return v.Interface(), nil
	}

	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	if v.Kind() != reflect.Map {
		return nil, fmt.Errorf("property %s is not a map", strings.Join(seenKeys, "."))
	}

	return walkProperties(v.MapIndex(reflect.ValueOf(keys[0])), keys[1:], append(seenKeys, keys[0]))
}

// Utility function that waits for a given txId to be confirmed by the network
func waitForConfirmation(txID string, client *algod.Client, timeout uint64) error {
	pt := new(models.PendingTransactionInfoResponse)
	if client == nil || txID == "" || timeout < 0 {
		return fmt.Errorf("Bad arguments for waitForConfirmation")

	}

	status, err := client.Status().Do(context.Background())
	if err != nil {
		return fmt.Errorf("error getting algod status: %s", err)
	}
	startRound := status.LastRound + 1
	currentRound := startRound

	for currentRound < (startRound + timeout) {

		*pt, _, err = client.PendingTransactionInformation(txID).Do(context.Background())
		if err != nil {
			return fmt.Errorf("error getting pending transaction: %s", err)
		}
		if pt.ConfirmedRound > 0 {
			log.Printf("Transaction %s confirmed in round %d\n", txID, pt.ConfirmedRound)
			return nil
		}
		if pt.PoolError != "" {
			return fmt.Errorf("There was a pool error, then the transaction has been rejected")
		}
		log.Printf("Waiting for confirmation...\n")
		status, err = client.StatusAfterBlock(currentRound).Do(context.Background())
		currentRound++
	}

	return fmt.Errorf("Tx not found in round range")
}
