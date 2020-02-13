import * as React from 'react';
import { WerftServiceClient, ResponseStream } from './api/werft_pb_service';
import { JobStatus, ListJobsResponse, ListJobsRequest, JobPhase, SubscribeRequest, SubscribeResponse, OrderExpression } from './api/werft_pb';
import { Header, headerStyles } from './components/header';
import { createStyles, Theme, Button, Table, TableHead, TableRow, TableCell, TableBody, Link, Grid, Tabs, Tab, Tooltip } from '@material-ui/core';
import { WithStyles, withStyles } from '@material-ui/styles';
import WarningIcon from '@material-ui/icons/Warning';
import DoneIcon from '@material-ui/icons/Done';
import { ColorUnknown, ColorSuccess, ColorFailure } from './components/colors';


const styles = (theme: Theme) => createStyles({
    main: {
        flex: 1,
        padding: theme.spacing(6, 4),
        background: '#eaeff1',
    },
    button: headerStyles(theme).button,
    hiddenLink: {
        color: 'inherit',
        textDecoration: 'none'
    }
});

type JobIdx = { [key: string]: JobStatus.AsObject };

interface BranchListProps extends WithStyles<typeof styles> {
    client: WerftServiceClient;
}

interface BranchListState {
    branches: Map<string, JobStatus.AsObject[]>
}

class BranchListImpl extends React.Component<BranchListProps, BranchListState> {

    protected eventStream: ResponseStream<SubscribeResponse> | undefined;

    constructor(props: BranchListProps) {
        super(props);
        this.state = {
            branches: new Map<string, JobStatus.AsObject[]>()
        };
    }

    async componentDidMount() {
        const order = new OrderExpression();
        order.setAscending(false);
        order.setField("created");
        const req = new ListJobsRequest();
        req.addOrder(order);
        req.setLimit(200);
        const resp = await new Promise<ListJobsResponse>((resolve, reject) => this.props.client.listJobs(req, (err, resp) => !!err ? reject(err) : resolve(resp!)));
        this.addToTree(resp.getResultList().map(r => r.toObject()));

        this.startListening();
    }

    protected startListening() {
        try {
            if (this.eventStream) {
                this.eventStream.cancel();
            }

            const req = new SubscribeRequest();
            this.eventStream = this.props.client.subscribe(req);
            this.eventStream.on('end', () => setTimeout(() => this.startListening(), 1000));
            this.eventStream.on('data', r => {
                const status = r.getResult();
                if (!status) {
                    return;
                }
                const incoming = status.toObject();
                this.addToTree([incoming]);
            });
            this.eventStream.on('status', console.warn);
        } catch (err) {
            console.warn(err);
            setTimeout(() => this.startListening(), 1200);
        }
    }

    protected addToTree(objs: JobStatus.AsObject[]) {
        const tree = this.state.branches;
        objs.filter(e => e.metadata && e.metadata.repository && e.metadata.repository.ref && e.metadata.repository.owner && e.metadata.repository.repo).forEach(e => {
            const key = e.metadata!.repository!.ref;
            let jobs = (tree.get(key) || []);
            jobs.push(e);
            jobs.sort((a, b) => {
                const as = a.name.split(".");
                const bs = b.name.split(".");
                const ap = as[as.length - 1];
                const bp = bs[bs.length - 1];
                const len = ap.length > bp.length ? ap.length : bp.length;
                const pap = ap.padStart(len, "0");
                const pbp = bp.padStart(len, "0");
                if (pap < pbp) {
                    return 1;
                } else {
                    return -1;
                }
            });

            const maxJobCount = 20;
            if (jobs.length > maxJobCount) {
                jobs = jobs.slice(0, maxJobCount);
            }

            tree.set(key, jobs);
        });
        this.setState({branches: tree});
    }

    render() {
        const classes = this.props.classes;

        const rows = Array.from(this.state.branches.entries()).map(e => {
            return {
                name: e[0],
                jobs: e[1],
            }
        }).sort((a, b) => {
            if (a.name < b.name) {
                return -1;
            } else {
                return 1;
            }
        });
        type Row = typeof rows[number];
        const columns = [
            {
                property: "branch",
                header: "Branch",
                render: (row: Row) => {
                    return <Link href={`/jobs/${row.name}`}>{row.name}</Link>;
                }
            },
            {
                property: "repo",
                header: "Repo",
                render: (row: Row) => {
                    const md = row.jobs[0]!.metadata!.repository!;
                    const repo = `${md.host}/${md.owner}/${md.repo}`;
                    return <a className={classes.hiddenLink} href={`https://${repo}`}>{repo}</a>;
                }
            },
            {
                property: "jobs",
                header: "Jobs",
                render: (row: Row) => {
                    return row.jobs.map((j, i) => {
                        let statusColor = ColorUnknown;
                        let icon = (c: string) => <WarningIcon />;

                        if (j.conditions!.success) {
                            statusColor = ColorSuccess;
                            icon = (c: string) => <DoneIcon style={{ color: c }} />;
                        } else {
                            statusColor = ColorFailure;
                            icon = (c: string) => <WarningIcon style={{ color: c }} />;
                        }

                        let color = ColorUnknown;
                        if (j.phase === JobPhase.PHASE_DONE) {
                            color = statusColor;
                        }
                        return <Tooltip title={j.name}><Link href={`/job/${j.name}`} key={i}>{icon(color)}</Link></Tooltip>;
                    });
                }
            },
        ]

        const actions = <React.Fragment>
                <Grid item xs></Grid>
                <Grid item>
                    <Tabs onChange={() => {}} value="branches">
                        <Tab label="Jobs" value="jobs" href={`/jobs`} />
                        <Tab label="Branches" value="branches" href={`/branches`} />
                    </Tabs>
                </Grid>
                <Grid item>
                    <Button href="/start" className={classes.button} variant="outlined" color="inherit" size="small">
                        Start Job
                    </Button>
                </Grid>
            </React.Fragment>

        return <React.Fragment>
            <Header title="Branches" actions={actions} />
            <main className={classes.main}>
                <Table>
                    <TableHead>
                        <TableRow>{columns.map(col =>
                            <TableCell key={col.property}>
                                { col.header }
                            </TableCell>
                        )}</TableRow>
                    </TableHead>
                    <TableBody>{rows.map((row, i) =>
                        <TableRow key={i}>{columns.map(col =>
                            <TableCell key={col.property}>
                                {col.render(row as any)}
                            </TableCell>
                        )}</TableRow>
                    )}</TableBody>
                </Table>
            </main>
        </React.Fragment>
    }

}

export const BranchList = withStyles(styles)(BranchListImpl);
