import * as React from "react";
import { LogSliceEvent, LogSliceType } from "../api/werft_pb";
import { Theme, createStyles, WithStyles, ExpansionPanel, ExpansionPanelDetails, ExpansionPanelSummary, Typography, Step, StepLabel, Stepper } from "@material-ui/core";
import { withStyles } from "@material-ui/styles";
import { StickyScroll } from "./StickyScroll";

export const styles = (theme: Theme) =>
    createStyles({
        dividerFullWidth: {
            margin: `5px 0 0 ${theme.spacing(2)}px`,
        },
        stepper: {
            marginBottom: '1em',
            backgroundColor: 'inherit',
        }
    });

export interface LogViewProps extends WithStyles<typeof styles> {
    logs: LogSliceEvent[];
    failed?: boolean;
    raw?: boolean;
    finished: boolean;
}

export interface LogViewState {
    chunks: Map<string, Chunk>;
    autoscroll: boolean;
}

type Chunk = Content | Phase;

interface Content {
    type: "content"
    name: string
    lines: string[]
}

function isContent(c: Chunk): c is Content {
    return "type" in c && c.type === "content";
}

interface Phase {
    type: "phase"
    name: string
    desc: string
}

function isPhase(c: Chunk): c is Phase {
    return "type" in c && c.type === "phase";
}

class LogViewImpl extends React.Component<LogViewProps, LogViewState> {
    
    constructor(props: LogViewProps) {
        super(props);
        this.state = {
            chunks: new Map<string, Chunk>(),
            autoscroll: true
        }

        this.updateChunks();
    }

    protected updateChunks() {
        let chunks = this.state.chunks;
        let icCount = new Map<string, number>();
        let phase = "default"

        this.props.logs.forEach(le => {
            const id = phase + ":" + le.getName();
            if (le.getType() === LogSliceType.SLICE_START) {
                icCount.set(le.getName(), 0);
            } else if (le.getType() === LogSliceType.SLICE_CONTENT) {
                const content = (chunks.get(id) || { 
                    type: "content",
                    name: le.getName(),
                    lines: []
                 }) as Content;

                const icc = (icCount.get(id) || 0) + 1;
                icCount.set(id, icc);
                if (icc <= content.lines.length) {
                    return
                }

                content.lines.push(le.getPayload());
                chunks.set(id, content);
            } else if (le.getType() === LogSliceType.SLICE_PHASE) {
                chunks.set("phase:"+le.getName(), {
                    type: "phase",
                    desc: le.getPayload(),
                    name: le.getName()
                })
                phase = le.getName();
            }
        });
    }

    renderRaw() {
        const rawLog = this.props.logs.map(c => c.getPayload());
        const rawContent = rawLog.join("")
        return <React.Fragment>
            <StickyScroll>
                <div className="term-container" style={{width:"100%"}} dangerouslySetInnerHTML={{__html: rawContent}} />
            </StickyScroll>
        </React.Fragment>
    }

    renderSliced() {
        this.updateChunks();
        const chunks = Array.from(this.state.chunks.entries());
        const classes = this.props.classes;
        
        const phases = chunks.map(c => c[1]).filter(c => isPhase(c));
        return <React.Fragment>
            <Stepper className={classes.stepper} alternativeLabel activeStep={phases.length-1}>{ phases.map((c, i) => 
                <Step key={i}>
                    <StepLabel error={this.props.failed && i === phases.length - 1}>{(c as Phase).desc}</StepLabel>
                </Step>
            )}</Stepper>

            <StickyScroll>
            { chunks.map((kv, i) => {
                const chunk = kv[1];
                if (isContent(chunk)) { return (
                    <ExpansionPanel key={kv[0]} defaultExpanded={i===chunks.length - 1}>
                        <ExpansionPanelSummary>{chunk.name}</ExpansionPanelSummary>
                        <ExpansionPanelDetails>
                            <div className="term-container" style={{width:"100%"}} dangerouslySetInnerHTML={{__html: chunk.lines.join("<br />")}} />
                        </ExpansionPanelDetails>
                    </ExpansionPanel>
                 )}
                 if (isPhase(chunk)) { return (
                    <Typography
                        className={this.props.classes.dividerFullWidth}
                        color="textSecondary"
                        display="block"
                        variant="caption"
                        key={kv[0]}
                        id={kv[0]}
                    >
                        {chunk.desc}
                    </Typography>
                 )}

                 return undefined;
            }) }
            </StickyScroll>
        </React.Fragment>
    }

    render() {
        return <React.Fragment>
            { this.props.raw && this.renderRaw() }
            { !this.props.raw && this.renderSliced() }
        </React.Fragment>
    }
 
}

export const LogView = withStyles(styles)(LogViewImpl);
