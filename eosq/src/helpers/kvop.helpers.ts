import { KvOp, StateKvToJsonResponse } from "@dfuse/client"

interface KeyValues {
  [key: string]: string[]
}

export interface ABIDecoderParams {
  table: string
  account: string
  hex_rows: string[]
  block_num: number
}

export function groupKVOpHex(kvops: KvOp[]): KeyValues {
  const groupedKVOp: KeyValues = {}
  kvops.forEach((kvop: KvOp) => {
    const key = `${kvop.account}`
    if (groupedKVOp[key]) {
      groupedKVOp[key] = addKVOpHex(groupedKVOp[key], kvop)
    } else {
      groupedKVOp[key] = addKVOpHex([], kvop)
    }
  })

  return groupedKVOp
}

export function addKVOpHex(groupedKVOp: string[], kvop: KvOp) {
  if (kvop.old && kvop.old.hex) {
    groupedKVOp.push(kvop.old.hex)
  }

  if (kvop.new && kvop.new.hex) {
    groupedKVOp.push(kvop.new.hex)
  }

  return groupedKVOp
}

export function decodeABIResponseToKVOps(
  responses: (StateKvToJsonResponse | undefined)[],
  kvops: KvOp[]
) : KvOp[] {
  let decodedKVOps: KvOp[] = []
  responses.forEach((response) => {
    if (!response) {
      return
    }

    let index = 0
    const tmpKVOps = kvops
      .filter((kvop: KvOp) => kvop.account === response.account)
      .map((kvop: KvOp) => {
        if (kvop.old && kvop.old.hex) {
          kvop.old.json = response.rows[index]
          index += 1
        }

        if (kvop.new && kvop.new.hex) {
          kvop.new.json = response.rows[index]
          index += 1
        }

        return kvop
      })

    decodedKVOps = decodedKVOps.concat(tmpKVOps)
  })

  return decodedKVOps
}
