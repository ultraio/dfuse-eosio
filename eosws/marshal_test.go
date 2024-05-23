package eosws

import (
	"fmt"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/dfuse-io/dfuse-eosio/eosws/wsmsg"
)

// TestMarshalOutgoingMessager is a code snippet extracted from eosws.(*WSConn).Emit() function.
// Here, SetReqID() is called to set an invalid string, because another function for OutgoingMessager interface, SetType() is called 
// and the result is validated in Emit().
func TestMarshalOutgoingMessager(t *testing.T) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = fmt.Errorf("unexpected error marshalling message: %w", v)
			case string, fmt.Stringer:
				err = fmt.Errorf("unexpected error marshalling message: %s", v)
			default:
				err = fmt.Errorf("unexpected error marshalling message: %v", v)
			}
			fmt.Printf("%s\n", err)
			assert.Error(t, err)
		}
	}()
	var msg wsmsg.OutgoingMessager
	msg.SetReqID("")
	_, err = json.Marshal(msg)
}
