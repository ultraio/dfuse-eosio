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

package eosws

import (
	"fmt"
	"testing"
	"time"

	pbstatedb "github.com/dfuse-io/dfuse-eosio/pb/dfuse/eosio/statedb/v1"
)

func Test_onGetAccount(t *testing.T) {
	subscriptionHub := newTestSubscriptionHub(t, 0, nil)
	stateClient := pbstatedb.NewMockStateClient()

	cases := []struct {
		name           string
		msg            string
		account        string
		expectedOutput []string
		expectedError  string
	}{
		{
			name:           "sunny path",
			account:        `{"account_name":"eoscanadacom","privileged":false,"last_code_update":"1970-01-01T00:00:00","created":"2018-06-10T13:04:15","core_liquid_balance":"71603.4182 EOS","ram_quota":308040,"ram_usage":13324,"net_weight":85000,"cpu_weight":175000,"net_limit":{"used":105,"available":6595725,"max":6595830},"cpu_limit":{"used":1319,"available":15926,"max":17245},"permissions":[{"perm_name":"active","parent":"owner","required_auth":{"threshold":4,"accounts":[{"permission":{"actor":"eoscanadaaaa","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaab","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaac","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaad","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaae","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaaf","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaag","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaah","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaai","permission":"active"},"weight":1}],"waits":[{"wait_sec":10800,"weight":1}]}},{"perm_name":"blacklistops","parent":"active","required_auth":{"threshold":1,"keys":[{"key":"EOS7idX86zQ6M3mrzkGQ9MGHf4btSECmcTj4i8Le59ga7CpSpZYy5","weight":1}]}},{"perm_name":"claimer","parent":"active","required_auth":{"threshold":1,"keys":[{"key":"EOS7NFuBesBKK5XHHLtzFxm7S57Eq11gUtndrsvq3Mt3XZNMTHfqc","weight":1}]}},{"perm_name":"day2day","parent":"active","required_auth":{"threshold":1,"accounts":[{"permission":{"actor":"eoscanadaaaa","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaac","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaaf","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaag","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaah","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaai","permission":"active"},"weight":1}]}},{"perm_name":"eosforumdapp","parent":"active","required_auth":{"threshold":1,"keys":[{"key":"EOS7YNS1swh6QWANkzGgFrjiX8E3u8WK5CK9GMAb6EzKVNZMYhCH3","weight":1}]}},{"perm_name":"owner","parent":"","required_auth":{"threshold":5,"accounts":[{"permission":{"actor":"eoscanadaaaa","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaab","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaac","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaad","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaae","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaaf","permission":"active"},"weight":1}],"waits":[{"wait_sec":86400,"weight":1},{"wait_sec":604800,"weight":2}]}}],"total_resources":{"owner":"eoscanadacom","net_weight":"8.5000 EOS","cpu_weight":"17.5000 EOS", "power_weight":"0 EOS","ram_bytes":306640},"self_delegated_bandwidth":{"from":"eoscanadacom","to":"eoscanadacom","net_weight":"7.0000 EOS","cpu_weight":"17.0000 EOS"},"refund_request":null,"voter_info":{"owner":"eoscanadacom","proxy":"","producers":[],"staked":1530000,"last_vote_weight":665716568638.4147,"proxied_vote_weight":0,"is_proxy":0}}`,
			msg:            `{"type":"get_account","req_id":"abc", "fetch":true, "data": { "name": "eoscanadacom" }}`,
			expectedOutput: []string{`{"data":{"account":{"account_name":"eoscanadacom","account_verifications":null,"core_liquid_balance":"71603.4182 EOS","cpu_limit":{"available":15926,"max":17245,"used":1319},"cpu_weight":175000,"created":"2018-06-10T13:04:15","creator":{"block_id":"","block_num":0,"block_time":"1970-01-01T00:00:00Z","created":"eoscanadacom","creator":"bozo","trx_id":""},"eosio_any_linked_actions":null,"has_contract":false,"head_block_num":0,"head_block_time":"0001-01-01T00:00:00","last_code_update":"1970-01-01T00:00:00","linked_permissions":null,"net_limit":{"available":6595725,"max":6595830,"used":105},"net_weight":85000,"permissions":[{"linked_actions":null,"parent":"owner","perm_name":"active","required_auth":{"accounts":[{"permission":{"actor":"eoscanadaaaa","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaab","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaac","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaad","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaae","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaaf","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaag","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaah","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaai","permission":"active"},"weight":1}],"threshold":4,"waits":[{"wait_sec":10800,"weight":1}]}},{"linked_actions":null,"parent":"active","perm_name":"blacklistops","required_auth":{"keys":[{"key":"EOS7idX86zQ6M3mrzkGQ9MGHf4btSECmcTj4i8Le59ga7CpSpZYy5","weight":1}],"threshold":1}},{"linked_actions":null,"parent":"active","perm_name":"claimer","required_auth":{"keys":[{"key":"EOS7NFuBesBKK5XHHLtzFxm7S57Eq11gUtndrsvq3Mt3XZNMTHfqc","weight":1}],"threshold":1}},{"linked_actions":null,"parent":"active","perm_name":"day2day","required_auth":{"accounts":[{"permission":{"actor":"eoscanadaaaa","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaac","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaaf","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaag","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaah","permission":"active"},"weight":1},{"permission":{"actor":"eoscanadaaai","permission":"active"},"weight":1}],"threshold":1}},{"linked_actions":null,"parent":"active","perm_name":"eosforumdapp","required_auth":{"keys":[{"key":"EOS7YNS1swh6QWANkzGgFrjiX8E3u8WK5CK9GMAb6EzKVNZMYhCH3","weight":1}],"threshold":1}},{"linked_actions":null,"parent":"","perm_name":"owner","required_auth":{"accounts":[{"permission":{"actor":"eoscanadaaaa","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaab","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaac","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaad","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaae","permission":"active"},"weight":2},{"permission":{"actor":"eoscanadaaaf","permission":"active"},"weight":1}],"threshold":5,"waits":[{"wait_sec":86400,"weight":1},{"wait_sec":604800,"weight":2}]}}],"privileged":false,"ram_quota":308040,"ram_usage":13324,"refund_request":null,"self_delegated_bandwidth":{"cpu_weight":"17.0000 EOS","from":"eoscanadacom","net_weight":"7.0000 EOS","power_weight":"0 ","to":"eoscanadacom"},"subjective_cpu_bill_limit":{"available":0,"max":0,"used":0},"total_resources":{"cpu_weight":"17.5000 EOS","net_weight":"8.5000 EOS","owner":"eoscanadacom","power_weight":"0 EOS","ram_bytes":306640},"voter_info":{"flags1":0,"is_proxy":0,"last_vote_weight":665716568638.4147,"owner":"eoscanadacom","producers":[],"proxied_vote_weight":0,"proxy":"","reserved2":0,"reserved3":"0 ","staked":1530000}}},"req_id":"abc","type":"account"}`},
		},
		{
			name:           "invalid_account path",
			msg:            `{"type":"get_account","req_id":"abc", "fetch":true, "data": { "name": "eoscanadacomcomcom" }}`,
			expectedOutput: []string{fmt.Sprintf(`{"data": {"code":"ws_message_data_validation_error", "details":{"reason":"The data.name field must be a valid EOS name"}, "message":"The received message data is not valid.", "trace_id":"%s"}, "req_id":"abc", "type":"error"}`, defaultTraceID)},
		},
		{
			name:           "invalid_account path",
			msg:            `{"type":"get_account","req_id":"abc", "fetch":true, "data": { "name": "eosc@n@d@c0m" }}`,
			expectedOutput: []string{fmt.Sprintf(`{"data": {"code":"ws_message_data_validation_error", "details":{"reason":"The data.name field must be a valid EOS name"}, "message":"The received message data is not valid.", "trace_id":"%s"}, "req_id":"abc", "type":"error"}`, defaultTraceID)},
		},
	}

	for _, c := range cases {

		t.Run(c.name, func(t *testing.T) {

			testAccountGetter := NewTestAccountGetter()
			testAccountGetter.SetAccount(c.account)

			handler := NewWebsocketHandler(
				nil,
				testAccountGetter,
				NewMockDB(""), // FIXME: this THING is NOT needed for those tests.. they are too all-encompassing.. better tests are needed.
				subscriptionHub,
				stateClient,
				nil,
				nil,
				nil,
				NewTestIrreversibleFinder("00000002a", nil),
				0,
				12,
			)

			conn, closer := newTestConnection(t, handler)
			defer closer()

			conn.WriteMessage(1, []byte(c.msg))

			validateOutput(t, "", c.expectedOutput, conn, 5*time.Second)
		})
	}
}
