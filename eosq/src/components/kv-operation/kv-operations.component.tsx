import { KvOp } from "@dfuse/client"
import * as React from "react"
import { Cell } from "../../atoms/ui-grid/ui-grid.component"
import { FormattedText } from "../formatted-text/formatted-text"
import { t } from "i18next"
import { theme, styled } from "../../theme"
import { MonospaceTextWrap } from "../../atoms/text-elements/misc"
import { JsonWrapper } from "@dfuse/explorer"

interface Props {
  kvops: KvOp[]
}

const EditIndicator: React.ComponentType<any> = styled(Cell)`
  position: absolute;
  left: -25px;
  width: 10px;
  text-align: center;
  top: -2px;
`

export class KVOperations extends React.Component<Props> {
  renderContentNew(kvop: KvOp) {
    if (kvop.new!.json) {
      return <JsonWrapper>{JSON.stringify(kvop.new!.json, null, "   ")}</JsonWrapper>
    }

    return kvop.new!.hex
  }

  renderContentOld(kvop: KvOp) {
    if (kvop.old!.json) {
      return <JsonWrapper>{JSON.stringify(kvop.old!.json, null, "   ")}</JsonWrapper>
    }

    return kvop.old!.hex
  }

  renderKVOps() {
    return this.props.kvops.map((kvop: KvOp, index: number) => {
      const fields = [
        { name: "operation", type: "bold", value: t(`transaction.kvops.operations.${kvop.op}`) },
        {
          name: "contract",
          type: "bold",
          value: kvop.account,
          // type: "searchShortcut",
          // query: `db.table:${kvop.account}`
        },
        {
          name: "key",
          type: "bold",
          value: kvop.key
        }
      ]

      return (
        <Cell pb={[2]} pt={[1]} key={index}>
          <FormattedText i18nKey="transaction.kvops.label" fields={fields} fontSize="12px" />
          <Cell pt={[1]}>
            {kvop.old && kvop.old.hex ? (
              <MonospaceTextWrap color={theme.colors.editRemove} fontSize={[1]}>
                <EditIndicator>-</EditIndicator> {this.renderContentOld(kvop)}
              </MonospaceTextWrap>
            ) : null}
            {kvop.new && kvop.new.hex ? (
              <MonospaceTextWrap color={theme.colors.editAdd} fontSize={[1]}>
                <EditIndicator>+</EditIndicator> {this.renderContentNew(kvop)}
              </MonospaceTextWrap>
            ) : null}
          </Cell>
        </Cell>
      )
    })
  }

  render() {
    return (
      <Cell>
        {this.renderKVOps()}
      </Cell>
    )
  }
}
