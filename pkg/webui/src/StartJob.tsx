import * as React from 'react';
import { WerftServiceClient } from './api/werft_pb_service';
import { Header, headerStyles } from './components/header';
import { createStyles, Theme, Tooltip, IconButton, Grid, Typography, Button, List, ListItem, ListItemText, ListItemIcon, TextField, Switch, CircularProgress } from '@material-ui/core';
import { WithStyles, withStyles } from '@material-ui/styles';
import CloseIcon from '@material-ui/icons/Close';
import { WerftUIClient } from './api/werft-ui_pb_service';
import { ListJobSpecsResponse, ListJobSpecsRequest } from './api/werft-ui_pb';
import { debounce } from './components/util';
import CheckIcon from '@material-ui/icons/Check';
import { StartGitHubJobRequest, JobMetadata, JobTrigger, Annotation } from './api/werft_pb';
import { green } from '@material-ui/core/colors';

const styles = (theme: Theme) => createStyles({
    main: {
        flex: 1,
        padding: theme.spacing(6, 4),
        background: '#eaeff1',
    },
    button: headerStyles(theme).button,
    actions: {
        textAlign: "right",
    },
    jobList: {
        overflow: "scroll-y",
    },
    arg: {
        width: "100%"
    },
    buttonProgress: {
        color: green[500],
        position: 'absolute',
        top: '50%',
        left: '50%',
        marginTop: -12,
        marginLeft: -12,
    },
});

interface StartJobProps extends WithStyles<typeof styles> {
    client: WerftServiceClient;
    uiClient: WerftUIClient;
}

interface StartJobState {
    specs: ListJobSpecsResponse[]
    args: Map<string, string>
    active?: ListJobSpecsResponse
    submitted?: boolean

    useRef?: boolean
    targetRefRev?: string
}

class StartJobImpl extends React.Component<StartJobProps, StartJobState> {

    constructor(props: StartJobProps) {
        super(props);
        this.state = {
            args: new Map<string, string>(),
            specs: [],
        };
    }

    async componentDidMount() {
        const updateState = debounce((a: Partial<StartJobState>) => this.setState(a as any), 100);
        try {
            let specs = this.props.uiClient.listJobSpecs(new ListJobSpecsRequest());
            specs.on("data", resp => {
                this.state.specs.push(resp);
                updateState({ specs: this.state.specs });
            })
        } catch (err) {
            alert(err);
        }
    }

    render() {
        const classes = this.props.classes;
        
        const actions = <React.Fragment>
            <Grid item xs></Grid>
            <Grid item>
                <Tooltip title="Back">
                    <IconButton color="inherit" onClick={() => window.location.href = "/"}>
                        <CloseIcon />
                    </IconButton>
                </Tooltip>
            </Grid>
        </React.Fragment>

        return <React.Fragment>
            <Header title="Start Job" actions={actions} />
            <main className={classes.main}>
                <form onSubmit={e => { e.preventDefault(); if(this.valid()) this.startJob(); }}>
                <Grid container alignItems="stretch">
                    <Grid item xs={this.state.active ? 6 : 12} className={classes.jobList}>
                        <Typography variant="h5">Job</Typography>
                        <List component="nav">{ this.state.specs.map((s, i) => {
                            const repo = s.toObject().repo!;
                            return (

                            <ListItem key={i} button component="a" onClick={() => this.setState({
                                active: s, 
                                useRef: !!s.getRepo()!.getRef(), 
                                targetRefRev: s.getRepo()!.getRef() || s.getRepo()!.getRevision()
                            })}>
                                { this.state.active === s && <ListItemIcon><CheckIcon /></ListItemIcon> }
                                <ListItemText primary={s.getName()} secondary={`${repo.owner}/${repo.repo}`} />
                            </ListItem>
                        )})}</List>
                    </Grid>
                    { this.state.active &&
                        <Grid item xs={6}>
                            <Typography variant="h5">Arguments</Typography>
                            <List>
                                <ListItem>
                                    <Switch defaultChecked={this.state.useRef} onChange={e => this.setState({useRef: e.target.checked})} />
                                    <TextField 
                                        className={classes.arg} 
                                        label={this.state.useRef ? "Ref" : "Revision"}
                                        value={this.state.targetRefRev}
                                        onChange={e => this.setState({targetRefRev: e.target.value})} />
                                </ListItem>

                                { this.state.active.getArgumentsList().map(a => a.toObject()).map((a, i) => (
                                    <ListItem key={i}>
                                        <ListItemText>
                                            <TextField 
                                                className={classes.arg}
                                                label={capitalize(a.name)}
                                                helperText={a.description}
                                                required={a.required}
                                                error={a.required && !this.state.args.get(a.name)} 
                                                onChange={e => {
                                                    this.state.args.set(a.name, e.target.value);
                                                    this.setState({ args: this.state.args });
                                                }}
                                            />
                                        </ListItemText>
                                    </ListItem>
                                ))}
                            </List>
                        </Grid>
                    }
                    <Grid item xs={12} className={classes.actions}>
                        <Button type="submit" variant="outlined" color="primary" onClick={() => this.startJob()} disabled={!this.state.submitted && !this.valid()}>
                            Go
                            {this.state.submitted && <CircularProgress size={24} className={classes.buttonProgress} />}
                        </Button>
                    </Grid>
                </Grid>
                </form>
            </main>
        </React.Fragment>
    }

    protected valid(): boolean {
        const j = this.state.active;
        if (!j) {
            return false;
        }

        const isReqMissing = j.getArgumentsList().find(a => a.getRequired() && !this.state.args.get(a.getName()));
        if (isReqMissing) {
            return false;
        }

        return true;
    }

    protected startJob() {
        const j = this.state.active;
        if (!j) {
            return
        }
        if (!!this.state.submitted) {
            return
        }

        this.setState({submitted: true});

        const repo = j.getRepo()!;
        if (!!this.state.useRef) {
            repo.setRef(this.state.targetRefRev!);
        } else {
            repo.setRevision(this.state.targetRefRev!);
        }

        const annotations = Array.from(this.state.args.entries()).map(kv => {
            const a = new Annotation();
            a.setKey(kv[0]);
            a.setValue(kv[1]);
            return a;
        });

        const md = new JobMetadata();
        md.setRepository(repo);
        md.setOwner("webui");
        md.setTrigger(JobTrigger.TRIGGER_MANUAL);
        md.setAnnotationsList(annotations);

        const req = new StartGitHubJobRequest();
        req.setJobName(j.getName());
        req.setMetadata(md);
        
        this.props.client.startGitHubJob(req, (err, ok) => {
            this.setState({submitted: false});
            if (err) {
                alert(err);
                return;
            }

            window.location.href = "/job/" + ok!.getStatus()!.getName();
        });
    }

    protected handleSearchKeyPress(evt: KeyboardEvent) {
        if (evt.charCode !== 13) {
            return
        }

        window.location.href = "/jobs/" + (evt.target as HTMLInputElement).value;
    }

}

function capitalize(v: string): string {
    return v[0].toUpperCase() + v.substring(1);
}

export const StartJob = withStyles(styles)(StartJobImpl);
