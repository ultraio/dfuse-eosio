import { decodeABIResponseToKVOps, groupKVOpHex } from "../helpers/kvop.helpers"
import { KvOp } from "@dfuse/client"
import { legacyHandleDfuseApiError } from "../clients/rest/api"

import { getDfuseClient } from "@dfuse/explorer"

export function decodeKVOps(kvops: KvOp[], blockNum: number, callback: (kvops: KvOp[]) => any) {
  const groupedKVOps = groupKVOpHex(kvops)
  console.log(groupedKVOps)
  const promises = Object.keys(groupedKVOps).map((groupKey: string) => {
    return getDfuseClient()
      .stateAbiKvToJson<any>(
        groupKey,
        groupedKVOps[groupKey],
        { blockNum }
      )
      .catch(legacyHandleDfuseApiError)
  })

  Promise.all(promises).then((responses) => {
    callback(decodeABIResponseToKVOps(responses, kvops))
  })
}
