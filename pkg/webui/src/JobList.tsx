import * as React from 'react';
import { WerftServiceClient, ResponseStream } from './api/werft_pb_service';
import { JobStatus, ListJobsResponse, ListJobsRequest, JobPhase, SubscribeRequest, FilterExpression, OrderExpression, SubscribeResponse } from './api/werft_pb';
import { Header, headerStyles } from './components/header';
import { createStyles, Theme, Button, Table, TableHead, TableRow, TableCell, TableSortLabel, TableBody, Link, Grid, TablePagination, Tabs, Tab } from '@material-ui/core';
import { WithStyles, withStyles } from '@material-ui/styles';
import ReactTimeago from 'react-timeago';
import WarningIcon from '@material-ui/icons/Warning';
import DoneIcon from '@material-ui/icons/Done';
import { ColorUnknown, ColorSuccess, ColorFailure } from './components/colors';
import { phaseToString } from './components/util';
import { SearchBox } from './components/SearchBox';

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

interface JobListProps extends WithStyles<typeof styles> {
    client: WerftServiceClient;
    readonly?: boolean;
}

interface JobListState {
    jobs: JobStatus.AsObject[]
    totalJobs: number
    sortCol?: string
    sortAscending: boolean
    rowsPerPage: number
    page: number
    search: FilterExpression[]
    initialSearchString: string | undefined;
}

class JobListImpl extends React.Component<JobListProps, JobListState> {

    protected eventStream: ResponseStream<SubscribeResponse> | undefined;

    constructor(props: JobListProps) {
        const initialSearch = decodeURIComponent(window.location.pathname.substring("/jobs/".length));

        super(props);
        this.state = {
            jobs: [],
            totalJobs: 0,
            sortCol: 'created',
            sortAscending: false,
            initialSearchString: initialSearch,
            search: [],
            rowsPerPage: 50,
            page: 0,
        };
    }

    async componentDidMount() {
        this.startListening();
    }

    protected startListening() {
        try {
            if (this.eventStream) {
                this.eventStream.cancel();
            }

            const req = new SubscribeRequest();
            req.setFilterList(this.state.search);
            this.eventStream = this.props.client.subscribe(req);
            this.eventStream.on('end', () => setTimeout(() => this.startListening(), 1000));
            this.eventStream.on('data', r => {
                const status = r.getResult();
                if (!status) {
                    return;
                }
                const incoming = status.toObject();

                const jobs = this.state.jobs;
                const idx = jobs.findIndex(o => o.name === incoming.name)
                if (idx > -1) {
                    jobs[idx] = incoming;
                } else {
                    jobs.unshift(incoming);
                }

                this.setState({ jobs });
            });
            this.eventStream.on('status', console.warn);
        } catch (err) {
            console.warn(err);
            setTimeout(() => this.startListening(), 1200);
        }
    }

    protected async update(newState: Partial<JobListState>) {
        const state = {
            ...this.state,
            ...newState
        };

        const req = new ListJobsRequest();
        req.setStart((state.page) * state.rowsPerPage);
        req.setLimit(state.rowsPerPage);
        req.setFilterList(state.search);

        if (!!state.sortCol) {
            const oexp = new OrderExpression();
            oexp.setField(state.sortCol);
            oexp.setAscending(state.sortAscending);

            // we display `created` as age which intutively sorts the other way 'round.
            if (state.sortCol === "age") {
                oexp.setAscending(!this.state.sortAscending);
            }

            req.setOrderList([oexp]);
        }

        const resp = await new Promise<ListJobsResponse>((resolve, reject) => this.props.client.listJobs(req, (err, resp) => !!err ? reject(err) : resolve(resp!)));
        state.jobs = resp.getResultList().map(r => r.toObject());
        state.totalJobs = resp.getTotal();
        const searchChanged = newState.search !== this.state.search;
        this.setState(state);
        
        if (searchChanged) {
            this.startListening();
        }
    }

    render() {
        const classes = this.props.classes;
        const columns = [
            {
                property: "name",
                header: "Name",
                search: true,
                sort: true,
                render: (row: JobStatus.AsObject) => {
                    return <Link href={`/job/${row.name}`}>{row.name}</Link>;
                }
            },
            {
                property: "owner",
                header: "Owner",
                search: true,
                sort: true,
                render: (row: JobStatus.AsObject) => {
                    return row.metadata!.owner;
                }
            },
            {
                property: "created",
                header: "Age",
                sort: true,
                render: (row: JobStatus.AsObject) => {
                    return <ReactTimeago date={row.metadata!.created!.seconds * 1000} />;
                },
            },
            {
                property: "repo.repo",
                header: "Repository",
                search: true,
                sort: true,
                render: (row: JobStatus.AsObject) => {
                    const md = row.metadata!.repository!;
                    const repo = `${md.host}/${md.owner}/${md.repo}`;
                    return <a className={classes.hiddenLink} href={`https://${repo}`}>{repo}</a>;
                }
            },
            {
                property: "repo.ref",
                header: "Ref",
                search: true,
                sort: true,
                render: (row: JobStatus.AsObject) => {
                    const md = row.metadata!.repository!;
                    if (md.host === "github.com") {
                        const repo = `${md.host}/${md.owner}/${md.repo}`;
                        return <a href={`https://${repo}/tree/${md.ref}`} className={classes.hiddenLink}>{md.ref}</a>
                    }
                    return md.ref;
                }
            },
            {
                property: "phase",
                header: "Phase",
                search: true,
                sort: true,
                render: (row: JobStatus.AsObject) => phaseToString(row.phase)
            },
            {
                property: "success",
                header: "Success",
                sort: true,
                render: (row: JobStatus.AsObject) => {
                    let statusColor = ColorUnknown;
                    let icon = (c: string) => <WarningIcon />;

                    if (row.conditions!.success) {
                        statusColor = ColorSuccess;
                        icon = (c: string) => <DoneIcon style={{ color: c }} />;
                    } else {
                        statusColor = ColorFailure;
                        icon = (c: string) => <WarningIcon style={{ color: c }} />;
                    }

                    let color = ColorUnknown;
                    if (row.phase === JobPhase.PHASE_DONE) {
                        color = statusColor;
                    }
                    return icon(color);
                }
            }
        ]
        const rows = this.state.jobs;

        const actions = <React.Fragment>
                <Grid item xs={2}></Grid>
                <Grid item xs={7}>
                    <SearchBox 
                        onUpdate={e => this.update({ search: e })} 
                        defaultValue={[this.state.initialSearchString].filter(e => !!e).map(e => e!)} />
                </Grid>
                <Grid item xs></Grid>
                <Grid item>
                    <Tabs onChange={() => {}} value="jobs">
                        <Tab label="Jobs" value="jobs" href={`/jobs`} />
                        <Tab label="Branches" value="branches" href={`/branches`} />
                    </Tabs>
                </Grid>
                <Grid item>
                    { !this.props.readonly && 
                        <Button href="/start" className={classes.button} variant="outlined" color="inherit" size="small">
                            Start Job
                        </Button>
                    }
                </Grid>
            </React.Fragment>

        return <React.Fragment>
            <Header title="Jobs" actions={actions} />
            <main className={classes.main}>
                <TablePagination
                    component="div"
                    count={this.state.totalJobs}
                    page={this.state.page}
                    rowsPerPageOptions={[10, 50, 100]}
                    rowsPerPage={this.state.rowsPerPage}
                    onPageChange={(_, page) => {
                        this.update({ page });
                    }}
                    onChangeRowsPerPage={(src) => {
                        this.update({ page: 0, rowsPerPage: parseInt(src.target.value) });
                    }}
                />
                <Table>
                    <TableHead>
                        <TableRow>{columns.map(col =>
                            <TableCell key={col.property}>
                                {col.sort &&
                                    <TableSortLabel
                                        active={this.state.sortCol === col.property}
                                        onClick={() => this.sortColumn(col.property)}
                                        direction={this.state.sortAscending ? 'asc' : 'desc'}
                                    >
                                        {col.header}
                                    </TableSortLabel>
                                }
                                {!col.sort && col.header }
                            </TableCell>
                        )}</TableRow>
                    </TableHead>
                    <TableBody>{rows.map((row, i) =>
                        <TableRow key={i}>{columns.map(col =>
                            <TableCell key={col.property}>
                                {col.render(row)}
                            </TableCell>
                        )}</TableRow>
                    )}</TableBody>
                </Table>
            </main>
        </React.Fragment>
    }

    protected sortColumn(col: string) {
        let sortAsc = this.state.sortAscending;
        if (this.state.sortCol === col) {
            sortAsc = !sortAsc;
        }

        this.update({ sortCol: col, sortAscending: sortAsc });
    }

    protected handleSearchKeyPress(evt: KeyboardEvent) {
        if (evt.key !== "Enter") {
            return;
        }

        window.location.href = "/jobs/" + (evt.target as HTMLInputElement).value;
    }

}

export const JobList = withStyles(styles)(JobListImpl);
