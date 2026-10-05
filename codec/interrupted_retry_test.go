package codec

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	pbcodec "github.com/dfuse-io/dfuse-eosio/pb/dfuse/eosio/codec/v1"
	"github.com/stretchr/testify/require"
)

func TestConsoleReader_InterruptedBlockRetry(t *testing.T) {
	for _, input := range []string{
		"DMLOG START_BLOCK 10\nDMLOG START_BLOCK 10\n",
		"DMLOG START_BLOCK 9\nDMLOG SWITCH_FORK\nDMLOG START_BLOCK 10\nDMLOG START_BLOCK 10\n",
	} {
		reader := testReaderConsoleReader(t, strings.NewReader(input), func() {})
		_, err := reader.Read()
		require.Equal(t, io.EOF, err)
	}
}

func TestConsoleReader_InterruptedRetryPreservesAcceptedOutput(t *testing.T) {
	input, err := os.ReadFile("testdata/deep-mind.dmlog")
	require.NoError(t, err)
	firstStart := bytes.Index(input, []byte("DMLOG START_BLOCK "))
	// Retry a normal block after genesis bootstrap operations were accepted.
	start := firstStart + 1 + bytes.Index(input[firstStart+1:], []byte("DMLOG START_BLOCK "))
	accepted := start + bytes.Index(input[start:], []byte("DMLOG ACCEPTED_BLOCK "))
	require.Greater(t, accepted, start)
	// Replay a complete attempted block's operations before accepting its retry.
	retry := append(append([]byte{}, input[:accepted]...), input[start:]...)
	read := func(data []byte) []*pbcodec.Block {
		reader := testReaderConsoleReader(t, bytes.NewReader(data), func() {})
		var blocks []*pbcodec.Block
		for {
			value, err := reader.Read()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			if value != nil {
				blocks = append(blocks, value.(*pbcodec.Block))
			}
		}
		return blocks
	}
	baseline, recovered := read(input), read(retry)
	require.NotEmpty(t, baseline)
	require.Len(t, recovered, len(baseline))
	for i := range baseline {
		require.Equal(t, protoJSONMarshalIndent(t, baseline[i]), protoJSONMarshalIndent(t, recovered[i]), "accepted block %d differs", i)
	}
}

func TestConsoleReader_UnexpectedActiveHeightStillFails(t *testing.T) {
	reader := testReaderConsoleReader(t, strings.NewReader("DMLOG START_BLOCK 10\nDMLOG START_BLOCK 11\n"), func() {})
	_, err := reader.Read()
	require.Error(t, err)
	require.Contains(t, err.Error(), "already processing block #10")
}

func TestABIDecoder_InterruptedRetryDiscardsUnacceptedABI(t *testing.T) {
	decoder := newABIDecoder()
	original, unaccepted := readABI(t, "test.1.abi.json"), readABI(t, "test.2.abi.json")
	previous := testBlock(t, "00000009aa", "00000008aa")
	require.NoError(t, decoder.cache.addABI("test", 0, original))
	require.NoError(t, decoder.startBlock(previous.Num()))
	require.NoError(t, decoder.endBlock(previous))
	require.NoError(t, decoder.startBlock(10))
	require.NoError(t, decoder.processTransaction(trxTrace(t, actionTraceSetABI(t, "test", 0, 100, unaccepted))))
	require.NoError(t, decoder.abortBlock())
	replacement := testBlock(t, "0000000aaa", "00000009aa",
		trxTrace(t, actionTrace(t, "test:test:act1", 0, 100, original, `{"from":"retry"}`)))
	require.NoError(t, decoder.startBlock(replacement.Num()))
	require.NoError(t, decoder.processTransaction(replacement.UnfilteredTransactionTraces[0]))
	require.NoError(t, decoder.endBlock(replacement))
	require.Same(t, original, decoder.cache.findABI("test", 100))
	require.JSONEq(t, `{"from":"retry"}`, replacement.UnfilteredTransactionTraces[0].ActionTraces[0].Action.JsonData)
}

func TestConsoleReader_ParseFailureReleasesLogPipe(t *testing.T) {
	input, output := io.Pipe()
	defer input.Close()
	defer output.Close()
	reader, err := NewConsoleReader(input)
	require.NoError(t, err)
	writeDone := make(chan error, 1)
	go func() {
		_, err := io.WriteString(output, "DMLOG START_BLOCK 10\nDMLOG START_BLOCK 11\n"+strings.Repeat("DMLOG START_BLOCK 11\n", 5000))
		writeDone <- err
	}()
	_, err = reader.Read()
	require.Error(t, err)
	select {
	case <-reader.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("parser failure left the scanner running")
	}
	select {
	case <-writeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("parser failure blocked the supervised log writer")
	}
	_, err = io.WriteString(output, "DMLOG START_BLOCK 12\n")
	require.Error(t, err)
	reader.Close()
	reader.Close()
}

func TestConsoleReader_ParseFailureBlockedScannerEndsWithEOF(t *testing.T) {
	input, output := io.Pipe()
	defer input.Close()
	defer output.Close()
	reader, err := NewConsoleReader(input)
	require.NoError(t, err)
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		_, _ = io.WriteString(output, "DMLOG START_BLOCK 10\nDMLOG START_BLOCK 11\n")
	}()
	<-writeDone
	_, err = reader.Read()
	require.Error(t, err)
	select {
	case <-reader.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("blocked scanner did not terminate")
	}
	// The mindreader closes its downstream block channel only on EOF. A closed
	// source error repeated forever would keep its shutdown consumer blocked.
	_, err = reader.Read()
	require.Equal(t, io.EOF, err)
}
