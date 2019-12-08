import * as React from "react";
import { LogSliceEvent, LogSlicePhase } from "../api/keel_pb";
import { Theme, createStyles, WithStyles, ExpansionPanel, ExpansionPanelDetails, ExpansionPanelSummary } from "@material-ui/core";
import { withStyles } from "@material-ui/styles";

export const styles = (theme: Theme) =>
    createStyles({
        
    });

export interface LogViewProps extends WithStyles<typeof styles> {
    logs: LogSliceEvent[];
}

export interface LogViewState {
    chunks: Map<string, string[]>;
}

class LogViewImpl extends React.Component<LogViewProps, LogViewState> {
    
    constructor(props: LogViewProps) {
        super(props);
        this.state = {
            chunks: new Map<string, string[]>()
        }

        this.updateChunks();
    }

    protected updateChunks() {
        let chunks = this.state.chunks;
        let icCount = new Map<string, number>();

        this.props.logs.forEach(le => {
            if (le.getPhase() === LogSlicePhase.SLICE_START) {
                icCount.set(le.getName(), 0);
            } else if (le.getPhase() === LogSlicePhase.SLICE_CONTENT) {
                const content = (chunks.get(le.getName()) || []);
                const icc = (icCount.get(le.getName()) || 0) + 1;
                icCount.set(le.getName(), icc);
                if (icc <= content.length) {
                    return
                }

                content.push(le.getPayload());
                chunks.set(le.getName(), content);
            }
        });
    }

    render() {
        this.updateChunks();
        const chunks = Array.from(this.state.chunks.entries());
        
        return <React.Fragment>
            { chunks.map((kv, i) => (
                <ExpansionPanel key={kv[0]} defaultExpanded={i===chunks.length - 1}>
                    <ExpansionPanelSummary>{kv[0]}</ExpansionPanelSummary>
                    <ExpansionPanelDetails>
                        <div className="term-container" style={{width:"100%"}} dangerouslySetInnerHTML={{__html: kv[1].join("<br />")}} />
                    </ExpansionPanelDetails>
                </ExpansionPanel>
            )) }
        </React.Fragment>
    }
 
}

export const LogView = withStyles(styles)(LogViewImpl);
