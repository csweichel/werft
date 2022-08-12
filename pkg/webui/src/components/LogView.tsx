import * as React from "react";
import { LogSliceEvent, LogSliceType } from "../api/werft_pb";
import { Theme, createStyles, WithStyles, ExpansionPanel, ExpansionPanelDetails, ExpansionPanelSummary, Typography, Step, StepLabel, Stepper, CircularProgress, Button, Toolbar, Grid, FormControlLabel, Switch } from "@material-ui/core";
import { withStyles } from "@material-ui/styles";
import { StickyScroll } from "./StickyScroll";
import DoneIcon from "@material-ui/icons/Done";
import ErrorIcon from "@material-ui/icons/Error";
import LinkIcon from '@material-ui/icons/Link';
import { ColorFailure } from './colors';

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
            alignItems: 'center',
            "&.Mui-expanded a": {
                opacity: 0.3,
            },
            "&.Mui-expanded a:hover": {
                opacity: 0.7,
            }
        },
        sectionTitle: {
            paddingLeft: '1em',
            width: '30%',
            overflow: 'hidden'
        },
        sectionDesc: {
            flex: 1,
            overflowWrap: 'anywhere'
        },
        sectionLink: {
            alignSelf: 'end',
            justifySelf: 'end',
            color: 'black',
            opacity: 0
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
    autoscroll: boolean;
    showKubeUpdates: boolean;
}

type Chunk = Content | Phase;

interface Content {
    type: "content"
    name: string
    lines: string[]
    status: "running" | "done" | "failed" | "unknown"
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
    protected readonly chunks: Map<string, Chunk>;

    constructor(props: LogViewProps) {
        super(props);

        this.chunks = new Map<string, Chunk>();
        this.state = {
            autoscroll: true,
            showKubeUpdates: false,
        }

        this.updateChunks();
    }

    protected updateChunks() {
        let chunks = this.chunks;
        chunks.clear();

        let phase = "default"
        this.props.logs.forEach((le, idx) => {
            const id = phase + ":" + le.getName();
            const type = le.getType();

            switch (type) {
                case LogSliceType.SLICE_PHASE: {
                    const id = "phase:"+le.getName();
                    if (chunks.has(id)) {
                        return;
                    }

                    chunks.set(id, {
                        type: "phase",
                        desc: le.getPayload(),
                        name: le.getName()
                    })
                    phase = le.getName();

                    Array.from(chunks.entries())
                        .filter(([id, chunk]) => !id.startsWith(phase) && isContent(chunk) && chunk.status === "running")
                        .forEach(([id, chunk]) => { 
                            (chunk as Content).status = "unknown"; 
                            chunks.set(id, chunk); 
                        });
                    break;
                }

                case LogSliceType.SLICE_START: {
                    if (phase === "default") {
                        debugger;
                    }
                    chunks.set(id, {
                        type: "content",
                        name: le.getName(),
                        lines: [],
                        status: "running",
                    });
                    break;
                }

                case LogSliceType.SLICE_CONTENT: {
                    const chunk = chunks.get(id) as Content;
                    if (!!chunk) {
                        chunk.lines.push(le.getPayload());
                    }
                    break;
                }

                case LogSliceType.SLICE_DONE: {
                    const chunk = chunks.get(id) as Content;
                    if (!!chunk) {
                        chunk.status = 'done';
                    }
                    break;
                }
                case LogSliceType.SLICE_FAIL: {
                    const chunk = chunks.get(id) as Content;
                    if (!!chunk) {
                        chunk.status = 'failed';
                    }
                    break;
                }
            }
        });
    }

    renderRaw() {
        return <React.Fragment>
            <Grid container>
                <Grid item>
                    <FormControlLabel label={<span className={this.props.classes.kubeUpdatesLabel}>Show Kubernetes Updates</span>} control={
                        <Switch checked={this.state.showKubeUpdates} onChange={e => {
                            this.setState({showKubeUpdates: e.target.checked});
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
                <div className="term-container" style={{width:"100%"}}>{this.getRawLogs()}</div>
            </StickyScroll>
        </React.Fragment>
    }

    renderSliced() {
        this.updateChunks();
        const chunks = Array.from(this.chunks.entries());
        const classes = this.props.classes;

        const activeChunk = window.location.hash ? window.location.hash.substring(1) : undefined;
        
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
                const isActiveChunk = kv[0] === activeChunk;
                let activeChunkEl: HTMLElement | undefined;
                if (isActiveChunk) {
                    setTimeout(() => {
                        if (!activeChunkEl) {
                            return;
                        }

                        window.scrollTo({top: activeChunkEl.scrollHeight, behavior: 'smooth'});
                    }, 100);
                }

                if (isContent(chunk) && !chunk.name.startsWith("werft:")) { return (
                    <ExpansionPanel ref={(el: HTMLElement) => activeChunkEl = el} key={kv[0]} defaultExpanded={chunk.status === "failed" || isActiveChunk}>
                        <ExpansionPanelSummary className={classes.sectionHeader} style={chunk.status === "failed" ? { color: ColorFailure} : {}}>
                            { chunk.status === "done" && <DoneIcon /> }
                            { chunk.status === "failed" && <ErrorIcon /> }
                            { chunk.status === "running" && !this.props.finished && <CircularProgress style={{width:'24px', height:'24px'}} /> }
                            { ((chunk.status === "running" && this.props.finished) || chunk.status === "unknown") && <DoneIcon style={{opacity:0.25}} /> }
                            <span className={classes.sectionTitle}>{ chunk.name }</span>
                            <span className={classes.sectionDesc} dangerouslySetInnerHTML={{__html: chunk.lines[chunk.lines.length - 1]}}></span>
                            <a className={classes.sectionLink} href={`#${kv[0]}`}><LinkIcon /></a>
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

    protected getRawLogs() {
        let rawLog = this.props.logs.map(c => c.getPayload());
        if (!this.state.showKubeUpdates) {
            rawLog = this.props.logs.filter(c => 
                !c.getPayload().trim().startsWith("[werft:kubernetes]")
                && !c.getPayload().trim().startsWith("[werft:status]")
            ).map(c => c.getPayload());
        }
        return rawLog.join("");
    }

    protected downloadRawLogs() {
        var element = document.createElement('a');
        element.setAttribute('href', 'data:text/plain;charset=utf-8,' + encodeURIComponent(this.getRawLogs()));
        element.setAttribute('download', (this.props.name || "werft-logs") + ".txt");
        element.style.display = 'none';
        document.body.appendChild(element);
        element.click();
        document.body.removeChild(element);
    }
 
}

export const LogView = withStyles(styles)(LogViewImpl);
