import * as React from "react";
import { LogSliceEvent, LogSliceType } from "../api/werft_pb";
import { Theme, createStyles, WithStyles, ExpansionPanel, ExpansionPanelDetails, ExpansionPanelSummary, Typography, Step, StepLabel, Stepper, CircularProgress, Button, Toolbar, Grid, FormControlLabel, Switch } from "@material-ui/core";
import { withStyles } from "@material-ui/styles";
import { StickyScroll } from "./StickyScroll";
import DoneIcon from "@material-ui/icons/Done";
import ErrorIcon from "@material-ui/icons/Error";

export const styles = (theme: Theme) =>
    createStyles({
        dividerFullWidth: {
            margin: `5px 0 0 ${theme.spacing(2)}px`,
        },
        stepper: {
            marginBottom: '1em',
            backgroundColor: 'inherit',
        },
        sectionHeader: {
            alignItems: 'center'
        },
        sectionTitle: {
            paddingLeft: '1em',
            width: '30%',
            overflow: 'hidden'
        },
        kubeUpdatesLabel: {
            fontSize: '1rem'
        }
    });

export interface LogViewProps extends WithStyles<typeof styles> {
    logs: LogSliceEvent[];
    failed?: boolean;
    raw?: boolean;
    finished: boolean;
    name?: string;
}

export interface LogViewState {
    chunks: Map<string, Chunk>;
    autoscroll: boolean;
    showKubeUpdates: boolean;
}

type Chunk = Content | Phase;

interface Content {
    type: "content"
    name: string
    lines: string[]
    status: "running" | "done" | "failed"
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
            autoscroll: true,
            showKubeUpdates: true,
        }

        this.updateChunks();
    }

    protected updateChunks() {
        let chunks = this.state.chunks;
        let icCount = new Map<string, number>();
        let phase = "default"

        this.props.logs.forEach(le => {
            const id = phase + ":" + le.getName();
            const type = le.getType();

            if (type === LogSliceType.SLICE_START) {
                icCount.set(le.getName(), 0);
            } else if (type === LogSliceType.SLICE_FAIL || type === LogSliceType.SLICE_DONE) {
                const content = (chunks.get(id) || { 
                    type: "content",
                    name: le.getName(),
                    lines: [],
                    status: "running"
                }) as Content;
                content.status = type === LogSliceType.SLICE_FAIL ? "failed" : "done";
                chunks.set(id, content);
            } else if (le.getType() === LogSliceType.SLICE_CONTENT) {
                const content = (chunks.get(id) || { 
                    type: "content",
                    name: le.getName(),
                    lines: [],
                    status: "running"
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
        let rawLog = this.props.logs.map(c => c.getPayload());
        if (!this.state.showKubeUpdates) {
            rawLog = this.props.logs.filter(c => !c.getPayload().trim().startsWith("[werft:kubernetes]")).map(c => c.getPayload());
        }
        const rawContent = rawLog.join("");

        return <React.Fragment>
            <Grid container>
                <Grid item>
                    <FormControlLabel label={<span className={this.props.classes.kubeUpdatesLabel}>Show Kubernetes Updates</span>} control={
                        <Switch checked={this.state.showKubeUpdates} onChange={e => {
                            this.setState({showKubeUpdates: e.target.checked});
                            console.log(this.state);
                        }} />
                    } />
                </Grid>
                <Grid item xs></Grid>
                <Grid item>
                    <Toolbar>
                        <Button onClick={() => this.downloadRawLogs()}>Download</Button>
                        <Button onClick={() => window.scrollTo({top: document.body.scrollHeight, behavior: 'smooth'})}>Scroll to Bottom</Button>
                    </Toolbar>
                </Grid>
            </Grid>
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
                if (isContent(chunk) && !chunk.name.startsWith("werft:")) { return (
                    <ExpansionPanel key={kv[0]} /*defaultExpanded={i===chunks.length - 1}*/>
                        <ExpansionPanelSummary className={classes.sectionHeader}>
                            { chunk.status === "done" && <DoneIcon /> }
                            { chunk.status === "failed" && <ErrorIcon /> }
                            { chunk.status === "running" && !this.props.finished && <CircularProgress style={{width:'24px', height:'24px'}} /> }
                            { chunk.status === "running" && this.props.finished && <DoneIcon style={{opacity:0.25}} /> }
                            <span className={classes.sectionTitle}>{ chunk.name }</span>
                            <span dangerouslySetInnerHTML={{__html: chunk.lines[chunk.lines.length - 1]}}></span>
                        </ExpansionPanelSummary>
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

    protected downloadRawLogs() {
        const rawLog = this.props.logs.map(c => c.getPayload());
        const rawContent = rawLog.join("")

        var element = document.createElement('a');
        element.setAttribute('href', 'data:text/plain;charset=utf-8,' + encodeURIComponent(rawContent));
        element.setAttribute('download', (this.props.name || "werft-logs") + ".txt");
        element.style.display = 'none';
        document.body.appendChild(element);
        element.click();
        document.body.removeChild(element);
    }
 
}

export const LogView = withStyles(styles)(LogViewImpl);
