package eosio

import pbcodec "github.com/dfuse-io/dfuse-eosio/pb/dfuse/eosio/codec/v1"

type Hydrator interface {
	// HydrateBlock decodes the received Deep Mind AcceptedBlock data structure against the
	// correct struct for this version of EOSIO supported by this hydrator.
	HydrateBlock(block *pbcodec.Block, input []byte) error

	// HydrateBlock decodes the received Deep Mind AcceptedBlock_V2 data structure against the
	// correct struct for this version of EOSIO supported by this hydrator.
	HydrateBlockV2(block *pbcodec.Block, input []byte, blockId string, blockNumber uint32, libNum uint32, finalityDataInput []byte, proposerPolicyInput []byte, finalizerPolicyInput []byte) error

	// DecodeTransactionTrace decodes the received Deep Mind AppliedTransaction data structure against the
	// correct struct for this version of EOSIO supported by this hydrator.
	DecodeTransactionTrace(input []byte, opts ...ConversionOption) (*pbcodec.TransactionTrace, error)
}
