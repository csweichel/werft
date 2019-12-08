import * as React from 'react';
import { WerftServiceClient } from './api/werft_pb_service';
import { JobStatus, GetJobRequest, GetJobResponse, LogSliceEvent, ListenRequest, ListenRequestLogs, JobPhase } from './api/werft_pb';
import ReactTimeago from 'react-timeago';
import './components/terminal.css';
import { LogView } from './components/LogView';
import { Header, headerStyles } from './components/header';
import { createStyles, Theme, Toolbar, Grid, Tooltip, IconButton } from '@material-ui/core';
import { WithStyles, withStyles } from '@material-ui/styles';
import CloseIcon from '@material-ui/icons/Close';
import { ColorUnknown, ColorFailure, ColorSuccess, ColorWarning } from './components/colors';
import { debounce, phaseToString } from './components/util';

const styles = (theme: Theme) => createStyles({
    main: {
        flex: 1,
        padding: theme.spacing(6, 4),
        background: '#eaeff1',
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
}

interface JobViewState {
    status?: JobStatus.AsObject
    showDetails: boolean;
    log: LogSliceEvent[];
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
        const req = new GetJobRequest();
        req.setName(this.props.jobName);
        const resp = await new Promise<GetJobResponse>((resolve, reject) => this.props.client.getJob(req, (err, resp) => !!err ? reject(err) : resolve(resp!)));
        this.setState({
            status: resp.getResult()!.toObject()
        });

        const lreq = new ListenRequest();
        lreq.setLogs(ListenRequestLogs.LOGS_HTML);
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

    render() {
        let color = ColorUnknown;
        if (this.state.status && this.state.status.conditions) {
            if (this.state.status.phase !== JobPhase.PHASE_DONE) {
                color = '#A3B2BD';
            } else if (this.state.status.conditions.success) {
                color = ColorSuccess;
            } else {
                color = ColorFailure;
            }
        }

        const actions = <React.Fragment>
            <Tooltip title="Back">
                <IconButton color="inherit" onClick={() => window.location.href = "/jobs"}>
                    <CloseIcon />
                </IconButton>
            </Tooltip>
        </React.Fragment>

        const job = this.state.status;
        const classes = this.props.classes;
        let secondary: React.ReactFragment | undefined;
        if (job) {
            secondary = <Toolbar>
                <Grid container spacing={1} alignItems="center" className={classes.infobar}>
                    <JobMetadataItemProps label="Owner">{job!.metadata!.owner}</JobMetadataItemProps>
                    <JobMetadataItemProps label="Repository">{`${job.metadata!.repository!.host}/${job.metadata!.repository!.owner}/${job.metadata!.repository!.repo}`}</JobMetadataItemProps>
                    <JobMetadataItemProps label="Revision" xs={6}>{job.metadata!.repository!.ref}</JobMetadataItemProps>
                    <JobMetadataItemProps label="Started"><ReactTimeago date={job.metadata!.created.seconds * 1000} /></JobMetadataItemProps>
                    <JobMetadataItemProps label="Finished">{!!job.metadata!.finished ? <ReactTimeago date={job.metadata!.finished.seconds * 1000} /> : "-"}</JobMetadataItemProps>
                    <JobMetadataItemProps label="Phase">{phaseToString(job.phase)}</JobMetadataItemProps>
                </Grid>
            </Toolbar>
        }

        return <React.Fragment>
            <Header color={color} title={this.props.jobName} actions={actions} secondary={secondary} />
            <main className={classes.main}>
                <LogView logs={this.state.log} />
            </main>
        </React.Fragment>
    }

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
