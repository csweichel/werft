import * as React from 'react';
import { WerftServiceClient } from './api/werft_pb_service';
import { JobStatus, GetJobRequest, GetJobResponse, LogSliceEvent, ListenRequest, ListenRequestLogs, JobPhase, StopJobRequest, StartFromPreviousJobRequest } from './api/werft_pb';
import ReactTimeago from 'react-timeago';
import './components/terminal.css';
import { LogView } from './components/LogView';
import { ResultView } from './components/ResultView';
import { Header, headerStyles } from './components/header';
import { createStyles, Theme, Toolbar, Grid, Tooltip, IconButton, Tabs, Tab, Typography, Button } from '@material-ui/core';
import { WithStyles, withStyles } from '@material-ui/styles';
import CloseIcon from '@material-ui/icons/Close';
import StopIcon from '@material-ui/icons/Stop';
import ReplayIcon from '@material-ui/icons/Replay';
import { ColorUnknown, ColorFailure, ColorSuccess } from './components/colors';
import { debounce, phaseToString } from './components/util';
import * as moment from 'moment';

const styles = (theme: Theme) => createStyles({
    main: {
        flex: 1,
        padding: theme.spacing(6, 4),
        background: '#eaeff1',
    },
    mainError: {
        flex: 1,
        padding: theme.spacing(6, 4),
        background: '#39355B'
    },
    errorMessage: {
        color: '#eaeff1',
        fontWeight: 800,
    },
    button: headerStyles(theme).button,
    metadataItemLabel: {
        fontWeight: "bold",
        paddingRight: "0.5em"
    },
    infobar: {
        paddingBottom: "1em"
    }
});

export interface JobViewProps extends WithStyles<typeof styles> {
    client: WerftServiceClient;
    jobName: string;
    view: "logs" | "raw-logs" | "results";
}

interface JobViewState {
    status?: JobStatus.AsObject
    showDetails: boolean;
    log: LogSliceEvent[];
    error?: any;
}

class JobViewImpl extends React.Component<JobViewProps, JobViewState> {
    protected logCache: LogSliceEvent[] = [];

    constructor(props: JobViewProps) {
        super(props);
        this.state = {
            showDetails: true,
            log: []
        };
    }

    async componentDidMount() {
        window.addEventListener("keydown", returnToJobList);

        const req = new GetJobRequest();
        req.setName(this.props.jobName);
        try {
            const resp = await new Promise<GetJobResponse>((resolve, reject) => this.props.client.getJob(req, (err, resp) => !!err ? reject(err) : resolve(resp!)));
            this.setState({ status: resp.getResult()!.toObject() });
        } catch (err) {
            this.setState({error: err});
            return;
        }

        const lreq = new ListenRequest();
        lreq.setLogs(ListenRequestLogs.LOGS_HTML);
        if (this.props.view === "raw-logs") {
            lreq.setLogs(ListenRequestLogs.LOGS_UNSLICED);
        }
        lreq.setUpdates(true);
        lreq.setName(this.props.jobName);
        const evts = this.props.client.listen(lreq);
        
        let updateLogState = debounce((l: LogSliceEvent[]) => this.setState({log: l}), 200);
        evts.on('data', h => {
            if (h.hasUpdate()) {
                this.setState({ status: h.getUpdate()!.toObject() });
            } else if (h.hasSlice()) {
                const log = this.logCache;
                log.push(h.getSlice()!);
                updateLogState(log);
            }
        });
        evts.on('end', console.log);
    }

    componentWillUnmount() {
        window.removeEventListener("keydown", returnToJobList)
    }

    render() {
        let color = ColorUnknown;
        let failed = false;
        let finished = true;
        if (this.state.status && this.state.status.conditions) {
            if (this.state.status.phase !== JobPhase.PHASE_DONE) {
                color = '#A3B2BD';
                finished = false;
            } else if (this.state.status.conditions.success) {
                color = ColorSuccess;
            } else {
                color = ColorFailure;
                failed = true;
            }
        }

        const job = this.state.status;
        const actions = <React.Fragment>
            <Grid item xs></Grid>
            <Grid item>
                <Tabs onChange={() => {}} value={this.props.view}>
                    <Tab label="Logs" value="logs" href={`/job/${this.props.jobName}`} />
                    <Tab label="Raw Logs" value="raw-logs" href={`/job/${this.props.jobName}/raw`} />
                    { job && job.resultsList.length > 0 && <Tab label="Results" value="results" href={`/job/${this.props.jobName}/results`} /> }
                </Tabs>
            </Grid>
            <Grid item>
                { !!job && !![JobPhase.PHASE_PREPARING, JobPhase.PHASE_STARTING, JobPhase.PHASE_RUNNING].find(i => job.phase === i) && 
                    <Tooltip title="Cancel Job">
                        <IconButton color="inherit" onClick={() => this.stopJob()}>
                            <StopIcon />
                        </IconButton>
                    </Tooltip>
                }
                { !!job && job.phase === JobPhase.PHASE_DONE && job.conditions!.canReplay &&
                    <Tooltip title="Replay">
                        <IconButton color="inherit" onClick={() => this.replay()}>
                            <ReplayIcon />
                        </IconButton>
                    </Tooltip>
                }
                <Tooltip title="Back">
                    <IconButton color="inherit" onClick={() => window.location.href = "/jobs"}>
                        <CloseIcon />
                    </IconButton>
                </Tooltip>
            </Grid>
        </React.Fragment>

        const classes = this.props.classes;
        let secondary: React.ReactFragment | undefined;
        if (job) {
            secondary = <Toolbar>
                <Grid container spacing={1} alignItems="center" className={classes.infobar}>
                    <JobMetadataItemProps label="Owner">{job!.metadata!.owner}</JobMetadataItemProps>
                    <JobMetadataItemProps label="Repository">{`${job.metadata!.repository!.host}/${job.metadata!.repository!.owner}/${job.metadata!.repository!.repo}`}</JobMetadataItemProps>
                    <JobMetadataItemProps label="Ref">{job.metadata!.repository!.ref}</JobMetadataItemProps>
                    <JobMetadataItemProps label="Started"><ReactTimeago date={job.metadata!.created.seconds * 1000} /></JobMetadataItemProps>
                    <JobMetadataItemProps label="Revision" xs={6}>{job.metadata!.repository!.revision}</JobMetadataItemProps>
                    <JobMetadataItemProps label="Phase">{phaseToString(job.phase)}</JobMetadataItemProps>
                    { job.metadata!.finished && <JobMetadataItemProps label="Finished">
                        <Tooltip title={((job.metadata!.finished.seconds-job.metadata!.created.seconds)/60)+" minutes"}>
                            <span>
                                {moment.unix(job.metadata!.finished.seconds).from(moment.unix(job.metadata!.created.seconds))}
                            </span>
                        </Tooltip>
                    </JobMetadataItemProps> }
                </Grid>
            </Toolbar>;
        }
        
        if (!!this.state.error) {
            return <main className={classes.mainError}>
                <Grid container alignContent="center" justify="center" direction="column">
                    <Grid item xs>
                        <Typography variant="h1" className={classes.errorMessage}>{this.state.error.metadata.headersMap["grpc-message"][0]}</Typography>
                        <Button variant="contained" href="/jobs">Back</Button>
                    </Grid>
                </Grid>
            </main>
        }

        return <React.Fragment>
            <Header color={color} title={this.props.jobName} actions={actions} secondary={secondary} />
            <main className={classes.main}>
                { this.state.status && this.state.status.details }
                { (this.props.view === "logs" || this.props.view === "raw-logs") &&
                    <LogView name={this.state.status && this.state.status.name} logs={this.state.log} failed={failed} raw={this.props.view === "raw-logs"} finished={finished} />
                }
                { this.props.view === "results" &&
                    <ResultView status={this.state.status} />  
                }
            </main> 
        </React.Fragment>
    }

    protected stopJob() {
        const req = new StopJobRequest();
        req.setName(this.props.jobName);
        this.props.client.stopJob(req, (err) => {
            if (!err) {
                return
            }

            alert(err);
        });
    }

    protected replay() {
        const job = this.state.status;
        if (!job) {
            return;
        }

        const req = new StartFromPreviousJobRequest();
        req.setPreviousJob(job.name);
        this.props.client.startFromPreviousJob(req, (err, ok) => {
            if (err) {
                alert(err);
                return;
            }

            window.location.href = "/job/" + ok!.getStatus()!.getName();
        });
    }

}

function returnToJobList(this: Window, evt: KeyboardEvent): any {
    if (evt.keyCode !== 27) {
        return
    }

    evt.preventDefault();
    window.location.replace("/jobs");
}

export const JobView = withStyles(styles)(JobViewImpl);

interface JobMetadataItemProps extends WithStyles<typeof styles> {
    label: string
    xs?: 1|2|3|4|5|6|7|8|9
}

class JobMetadataItemPropsImpl extends React.Component<JobMetadataItemProps, {}> {
    render() {
        return <Grid item xs={this.props.xs || 3}>
            <span className={this.props.classes.metadataItemLabel}>{this.props.label}</span>
            {this.props.children}
        </Grid>
    }
}

const JobMetadataItemProps = withStyles(styles)(JobMetadataItemPropsImpl)
