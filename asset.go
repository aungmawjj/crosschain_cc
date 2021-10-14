package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

const (
	KeyAssets = "assets"
)

type Asset struct {
	ID             []byte
	Owner          []byte
	PendingAuction []byte
}

func main() {
	err := shim.Start(new(AssetChaincode))
	if err != nil {
		log.Fatalf("failed to start chaincode %+v", err)
	}
}

type AssetChaincode struct {
	stub shim.ChaincodeStubInterface
	args []string
}

func (cc *AssetChaincode) Init(
	stub shim.ChaincodeStubInterface, method string, args []string,
) ([]byte, error) {
	return nil, nil
}

func (cc *AssetChaincode) Invoke(
	stub shim.ChaincodeStubInterface, method string, args []string,
) ([]byte, error) {
	cc.stub = stub
	cc.args = args

	switch method {

	case "addAsset":
		return nil, cc.addAsset()

	case "setAuction":
		return nil, cc.setAuction()

	case "endAuction":
		return nil, cc.endAuction()

	default:
		return nil, fmt.Errorf("method not found")
	}
}

func (cc *AssetChaincode) Query(
	stub shim.ChaincodeStubInterface, method string, args []string,
) ([]byte, error) {
	cc.stub = stub
	cc.args = args

	switch method {

	case "getAsset":
		return cc.getAssetRaw()

	default:
		return nil, fmt.Errorf("method not found")
	}
}

func (cc *AssetChaincode) addAsset() error {
	var asset Asset
	err := json.Unmarshal([]byte(cc.args[0]), &asset)
	if err != nil {
		return err
	}
	return cc.setAsset(asset)
}

type SetAuctionArgs struct {
	AssetID   []byte
	AuctionID []byte
}

func (cc *AssetChaincode) setAuction() error {
	var args SetAuctionArgs
	err := json.Unmarshal([]byte(cc.args[0]), &args)
	if err != nil {
		return err
	}
	asset, err := cc.getAsset(args.AssetID)
	if err != nil {
		return err
	}
	asset.PendingAuction = args.AuctionID

	return cc.setAsset(asset)
}

func (cc *AssetChaincode) endAuction() error {
	assetID, err := base64.StdEncoding.DecodeString(cc.args[0])
	if err != nil {
		return err
	}
	asset, err := cc.getAsset(assetID)
	if err != nil {
		return err
	}
	if asset.PendingAuction == nil {
		return fmt.Errorf("no pending auction")
	}

	result, err := cc.getAuctionResult(asset.PendingAuction)
	if err != nil {
		return err
	}
	if !result.Ended {
		return fmt.Errorf("base auction not ended yet")
	}
	// transfer asset to winner
	asset.Owner = result.HighestBidder
	asset.PendingAuction = nil
	return cc.setAsset(asset)
}

type AuctionResultRequest struct {
	Address []byte
}

type AuctionResult struct {
	Ended         bool
	HighestBid    int64
	HighestBidder []byte
}

func (cc *AssetChaincode) getAuctionResult(auctionID []byte) (*AuctionResult, error) {
	buf := bytes.NewBuffer(nil)
	json.NewEncoder(buf).Encode(AuctionResultRequest{Address: auctionID})
	resp, err := http.Post("http://localhost:9000/auction_result", "application/json", buf)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get auction result, status: %d", resp.StatusCode)
	}
	result := new(AuctionResult)
	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (cc *AssetChaincode) getAssetRaw() ([]byte, error) {
	assetID, err := base64.StdEncoding.DecodeString(cc.args[0])
	if err != nil {
		return nil, err
	}
	return cc.stub.GetState(cc.makeAssetKey(assetID))
}

func (cc *AssetChaincode) getAsset(assetID []byte) (Asset, error) {
	var asset Asset
	b, err := cc.stub.GetState(cc.makeAssetKey(assetID))
	if err != nil {
		return asset, err
	}
	if b == nil {
		return asset, fmt.Errorf("asset not found")
	}
	err = json.Unmarshal(b, &asset)
	return asset, err
}

func (cc *AssetChaincode) setAsset(asset Asset) error {
	b, err := json.Marshal(asset)
	if err != nil {
		return err
	}
	return cc.stub.PutState(cc.makeAssetKey(asset.ID), b)
}

func (cc *AssetChaincode) makeAssetKey(assetID []byte) string {
	return fmt.Sprintf("%s_%s", KeyAssets, assetID)
}
