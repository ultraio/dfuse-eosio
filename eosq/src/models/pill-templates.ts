import { ActionTrace, Action, DbOp, RAMOp, TableOp, KvOp } from "@dfuse/client"
import { PageContext } from "./core"

export interface GenericPillParams {
  console?: string
  dbops?: DbOp[]
  kvops?: KvOp[]
  ramops?: RAMOp[]
  tableops?: TableOp[]
  action: Action<any>
  traceInfo?: TraceInfo
  disabled?: boolean
  headerAndTitleOptions: { header: PillHeaderParams; title: string }
  pageContext?: PageContext
  highlighted?: boolean
  title?: string
  pill2Color?: string
}

export interface GenericPillState {
  fullContent: boolean
}

export interface PillHeaderParams {
  color: string
  text: string
  hoverTitle: string
}

export interface TraceInfo {
  receiver: string
  inline_traces: ActionTrace<any>[]
}
