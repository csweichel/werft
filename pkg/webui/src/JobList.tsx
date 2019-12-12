import * as React from 'react';
import { WerftServiceClient } from './api/werft_pb_service';
import { JobStatus, ListJobsResponse, ListJobsRequest, JobPhase, SubscribeRequest, FilterExpression, FilterTerm, FilterOp, OrderExpression } from './api/werft_pb';
import { Header, headerStyles } from './components/header';
import { createStyles, Theme, Button, Table, TableHead, TableRow, TableCell, TableSortLabel, TableBody, Link, Grid, fade, InputBase, TablePagination } from '@material-ui/core';
import { WithStyles, withStyles } from '@material-ui/styles';
import ReactTimeago from 'react-timeago';
import WarningIcon from '@material-ui/icons/Warning';
import DoneIcon from '@material-ui/icons/Done';
import SearchIcon from '@material-ui/icons/Search';
import { ColorUnknown, ColorSuccess, ColorFailure } from './components/colors';
import { debounce, phaseToString } from './components/util';


const styles = (theme: Theme) => createStyles({
    main: {
        flex: 1,
        padding: theme.spacing(6, 4),
        background: '#eaeff1',
    },
    button: headerStyles(theme).button,
    search: {
        position: 'relative',
        borderRadius: theme.shape.borderRadius,
        backgroundColor: fade(theme.palette.common.white, 0.15),
        '&:hover': {
            backgroundColor: fade(theme.palette.common.white, 0.25),
        },
        marginLeft: 0,
        width: '100%',
    },
    searchIcon: {
        width: theme.spacing(7),
        height: '100%',
        position: 'absolute',
        pointerEvents: 'none',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
    },
    inputRoot: {
        color: 'inherit',
        width: '100%'
    },
    inputInput: {
        padding: theme.spacing(1, 1, 1, 7),
        width: '100%',
    },
});

type JobIdx = { [key: string]: JobStatus.AsObject };

interface JobListProps extends WithStyles<typeof styles> {
    client: WerftServiceClient;
}

interface JobListState {
    jobs: JobStatus.AsObject[]
    totalJobs: number
    sortCol?: string
    sortAscending: boolean
    search?: string
    rowsPerPage: number
    page: number
}

class JobListImpl extends React.Component<JobListProps, JobListState> {

    constructor(props: JobListProps) {
        let search: string | undefined = window.location.pathname.substring("/job/".length+1);
        if (search.length === 0) {
            search = undefined;
        }

        super(props);
        this.state = {
            jobs: [],
            totalJobs: 0,
            sortCol: 'created',
            sortAscending: false,
            search,
            rowsPerPage: 50,
            page: 0
        };
    }

    async componentDidMount() {
        try {
            this.update({});
            this.startListening();
        } catch (err) {
            alert(err);
        }
    }

    protected startListening() {
        try {
            const req = new SubscribeRequest();
            let evts = this.props.client.subscribe(req);
            evts.on('end', () => setTimeout(() => this.startListening(), 1000));
            evts.on('data', r => {
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
            evts.on('status', console.warn);
        } catch (err) {
            alert(err);
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

        let query: any = {};
        if (state.search) {
            query._all = state.search;
        }

        let allFilter: FilterExpression[] = [];
        if (query._all) {
            const terms = ['name', 'owner', 'repo.repo', 'phase'].map(f => {
                const tt = new FilterTerm();
                tt.setField(f);
                tt.setOperation(FilterOp.OP_CONTAINS);
                tt.setValue(query._all);
                return tt;
            });
            const tf = new FilterExpression();
            tf.setTermsList(terms);
            allFilter.push(tf);

            delete query["_all"];
        }

        allFilter = allFilter.concat(Object.getOwnPropertyNames(query).filter(f => !f.startsWith("_")).map(f => {
            const tf = new FilterExpression();
            const tt = new FilterTerm();
            tt.setField(f);
            tt.setOperation(FilterOp.OP_CONTAINS);
            tt.setValue(query[f]);
            tf.setTermsList([tt]);
            return tf;
        }));
        req.setFilterList(allFilter);

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
        this.setState(state);
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
                    return `${md.host}/${md.owner}/${md.repo}`;
                }
            },
            {
                property: "repo.ref",
                header: "Ref",
                search: true,
                sort: true,
                render: (row: JobStatus.AsObject) => {
                    return row.metadata!.repository!.ref!;
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

        const debounceSearch = debounce((search?: string) => this.update({}), 500);
        const actions = <React.Fragment>
                <Grid item xs={2}></Grid>
                <Grid item xs={7}>
                    <div className={classes.search}>
                        <div className={classes.searchIcon}>
                            <SearchIcon />
                        </div>
                        <InputBase
                            placeholder="Search…"
                            classes={{
                                root: classes.inputRoot,
                                input: classes.inputInput,
                            }}
                            inputProps={{ 'aria-label': 'search' }}
                            onChange={e => {
                                const search = e.target.value;
                                this.update({ search });
                                debounceSearch(undefined);
                            }}
                            onKeyPress={e => this.handleSearchKeyPress(e as any)}
                            value={this.state.search}
                        />
                    </div>
                </Grid>
                <Grid item xs></Grid>
                <Grid item>
                    <Button href="/start" className={classes.button} variant="outlined" color="inherit" size="small">
                        Start Job
                    </Button>
                </Grid>
            </React.Fragment>

        return <React.Fragment>
            <Header title="Jobs" actions={actions} />
            <main className={classes.main}>
                <TablePagination
                    rowsPerPageOptions={[10, 50, 100]}
                    component="div"
                    count={this.state.totalJobs}
                    page={this.state.page}
                    rowsPerPage={this.state.rowsPerPage}
                    onChangePage={(_, page) => {
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
        if (evt.charCode !== 13) {
            return
        }

        window.location.href = "/jobs/" + (evt.target as HTMLInputElement).value;
    }

}

export const JobList = withStyles(styles)(JobListImpl);
