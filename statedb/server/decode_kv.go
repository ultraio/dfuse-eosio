// Copyright 2020 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/dfuse-io/dfuse-eosio/statedb"
	eos "github.com/eoscanada/eos-go"
	"github.com/francoispqt/gojay"
	"github.com/streamingfast/derr"
	"github.com/streamingfast/logging"
	"github.com/streamingfast/validator"
	"go.uber.org/zap"
)

func (srv *EOSServer) decodeKVHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	zlog := logging.Logger(ctx, zlog)

	request := &decodeKVRequest{}
	err := extractDecodeKVRequest(r, request)
	if err != nil {
		writeError(ctx, w, fmt.Errorf("extracting request: %w", err))
		return
	}

	zlog.Debug("extracted request", zap.Reflect("request", request))

	account := request.Account
	abiEntry, err := srv.fetchABI(ctx, string(account), request.BlockNum)
	if err != nil {
		writeError(ctx, w, fmt.Errorf("fetch ABI: %w", err))
		return
	}

	if abiEntry == nil {
		writeError(ctx, w, statedb.DataABINotFoundError(ctx, string(account), request.BlockNum))
		return
	}

	abi, _, err := abiEntry.ABI()
	if err != nil {
		writeError(ctx, w, fmt.Errorf("decode ABI: %w", err))
		return
	}

	response := &decodeKVResponse{
		BlockNum: abiEntry.Height(),
		Account:  request.Account,
	}

	for _, structDef := range abi.Structs {
		tableName := structDef.Name
		for _, hexRow := range request.HexRows {
			hexData, _ := hex.DecodeString(hexRow)
			decodedRow, err := abi.DecodeTableRowTyped(tableName, hexData)
			if err != nil {
				continue
			}

			response.Table = eos.TableName(tableName)
			response.DecodedRows = append(response.DecodedRows, string(decodedRow))
		}
	}

	if response.Table == "" {
		writeError(ctx, w, statedb.DataKVDecodingError(ctx, account.String()))
		return
	}

	zlog.Debug("streaming response", zap.Int("row_count", len(response.DecodedRows)), zap.Uint64("block_nun", response.BlockNum))
	streamResponse(ctx, w, response)
}

type decodeKVRequest struct {
	Account  eos.AccountName `json:"account"`
	HexRows  []string        `json:"hex_rows"`
	BlockNum uint64          `json:"block_num"`
}

type decodeKVResponse struct {
	BlockNum    uint64          `json:"block_num"`
	Account     eos.AccountName `json:"account"`
	Table       eos.TableName   `json:"table"`
	DecodedRows []string        `json:"rows"` // The slice elements are valid JSON string
}

func validateDecodeKVRequest(r *http.Request, request *decodeKVRequest) url.Values {
	return validator.ValidateJSONBody(r, request, validator.Rules{
		"account":  []string{"required", "fluxdb.eos.name"},
		"hex_rows": []string{"required", "fluxdb.eos.hexRows"},
	})
}

func extractDecodeKVRequest(r *http.Request, request *decodeKVRequest) error {
	ctx := r.Context()
	if r.Body == nil {
		return derr.MissingBodyError(ctx)
	}

	requestErrors := validateDecodeKVRequest(r, request)
	if len(requestErrors) > 0 {
		if _, ok := requestErrors["_error"]; ok {
			return derr.InvalidJSONError(ctx, errors.New(requestErrors["_error"][0]))
		}

		return derr.RequestValidationError(ctx, requestErrors)
	}

	return nil
}

func (r *decodeKVResponse) MarshalJSONObject(enc *gojay.Encoder) {
	enc.AddIntKey("block_num", int(r.BlockNum))
	enc.AddStringKey("account", string(r.Account))
	enc.AddStringKey("table", string(r.Table))

	enc.AddArrayKey("rows", gojay.EncodeArrayFunc(func(enc *gojay.Encoder) {
		lastIdx := len(r.DecodedRows) - 1
		for idx, decodedRow := range r.DecodedRows {
			enc.AppendBytes([]byte(decodedRow))
			if idx != lastIdx {
				enc.AppendByte(',')
			}
		}
	}))
}

func (r *decodeKVResponse) IsNil() bool { return r == nil }
