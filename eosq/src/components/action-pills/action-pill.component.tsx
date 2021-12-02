import * as React from "react"
import { getHeaderAndTitle } from "../../helpers/action.helpers"
import { ActionTrace, Action, DbOp, RAMOp, TableOp, KvOp } from "@dfuse/client"
import { PageContext } from "../../models/core"
import { templateStore } from "../../stores"
import { TraceInfo } from "../../models/pill-templates"

interface Props {
  action: Action<any>
  disabled?: boolean
  ramops?: RAMOp[]
  dbops?: DbOp[]
  kvops?: KvOp[]
  pageContext?: PageContext
}

export const ActionPill: React.SFC<Props> = ({ action, disabled, pageContext, ramops, dbops, kvops }) => {
  const headerParams = getHeaderAndTitle(action, action.account)
  const ConcreteComponent = templateStore.getComponent(action)

  return (
    <ConcreteComponent
      dbops={dbops}
      kvops={kvops}
      ramops={ramops}
      pageContext={pageContext}
      disabled={disabled}
      action={action}
      headerAndTitleOptions={headerParams}
    />
  )
}

interface TraceProps {
  actionTrace: ActionTrace<any>
  disabled?: boolean
  ramops?: RAMOp[]
  dbops?: DbOp[]
  kvops?: KvOp[]
  tableops?: TableOp[]
  pageContext?: PageContext
  highlighted?: boolean
}

export const ActionTracePill: React.SFC<TraceProps> = ({
  ramops,
  dbops,
  kvops,
  tableops,
  actionTrace,
  disabled,
  pageContext,
  highlighted
}) => {
  const action = actionTrace.act
  const headerParams = getHeaderAndTitle(action, actionTrace.receipt.receiver)
  const ConcreteComponent = templateStore.getComponent(action)

  const traceInfo: TraceInfo = {
    inline_traces: actionTrace.inline_traces || [],
    receiver: actionTrace.receipt.receiver
  }

  return (
    <ConcreteComponent
      console={actionTrace.console}
      highlighted={highlighted}
      ramops={ramops}
      pageContext={pageContext}
      dbops={dbops}
      kvops={kvops}
      tableops={tableops}
      disabled={disabled}
      action={action}
      traceInfo={traceInfo}
      headerAndTitleOptions={headerParams}
    />
  )
}
