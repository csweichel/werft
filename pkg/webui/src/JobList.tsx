import * as React from 'react';
import { WerftServiceClient } from './api/werft_pb_service';
import { JobStatus, ListJobsResponse, ListJobsRequest, JobPhase, SubscribeRequest, FilterExpression, FilterTerm, FilterOp, OrderExpression } from './api/werft_pb';
import { Header, headerStyles } from './components/header';
import { createStyles, Theme, Button, Table, TableHead, TableRow, TableCell, TableSortLabel, TableBody, Link, Toolbar, Grid, fade, InputBase } from '@material-ui/core';
import { WithStyles, withStyles } from '@material-ui/styles';
import ReactTimeago from 'react-timeago';
import WarningIcon from '@material-ui/icons/Warning';
import DoneIcon from '@material-ui/icons/Done';
import SearchIcon from '@material-ui/icons/Search';
import { ColorUnknown, ColorSuccess, ColorFailure } from './components/colors';
import { debounce } from './components/util';


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
        [theme.breakpoints.up('sm')]: {
            marginLeft: theme.spacing(1),
            width: 'auto',
        },
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
    },
    inputInput: {
        padding: theme.spacing(1, 1, 1, 7),
        transition: theme.transitions.create('width'),
        width: '100%',
        [theme.breakpoints.up('sm')]: {
            width: 120,
            '&:focus': {
                width: 200,
            },
        },
    },
});

interface JobListProps extends WithStyles<typeof styles> {
    client: WerftServiceClient;
}

interface JobListState {
    jobs: Map<string, JobStatus.AsObject>
    sortCol?: string
    sortAscending: boolean
}


class JobListImpl extends React.Component<JobListProps, JobListState> {

    constructor(props: JobListProps) {
        super(props);
        this.state = {
            jobs: new Map<string, JobStatus.AsObject>(),
            sortAscending: true
        };
    }

    async componentDidMount() {
        try {
            const req = new ListJobsRequest();
            req.setLimit(50);
            const resp = await new Promise<ListJobsResponse>((resolve, reject) => this.props.client.listJobs(req, (err, resp) => !!err ? reject(err) : resolve(resp!)));
            const jobs = resp.getResultList().map(r => r.toObject());

            const idx = new Map<string, JobStatus.AsObject>();
            jobs.forEach(j => idx.set(j.name, j));

            this.setState({ jobs: idx });
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

                const jobs = this.state.jobs || {};
                jobs.set(status.getName(), status.toObject());
                this.setState({ jobs });
            });
        } catch (err) {
            alert(err);
        }
    }

    protected async search(query: any) {
        const req = new ListJobsRequest();
        req.setLimit(50);

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

        if (!!this.state.sortCol) {
            const oexp = new OrderExpression();
            oexp.setField(this.state.sortCol);
            oexp.setAscending(this.state.sortAscending);
            req.setOrderList([oexp]);
        }

        const resp = await new Promise<ListJobsResponse>((resolve, reject) => this.props.client.listJobs(req, (err, resp) => !!err ? reject(err) : resolve(resp!)));
        const jobs = resp.getResultList().map(r => r.toObject());

        const idx = new Map<string, JobStatus.AsObject>();
        jobs.forEach(j => idx.set(j.name, j));

        this.setState({ jobs: idx });
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
                render: (row: JobStatus.AsObject) => {
                    return <ReactTimeago date={row.metadata!.created!.seconds * 1000} />;
                }
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
                property: "phase",
                header: "Phase",
                search: true,
                sort: true,
                render: (row: JobStatus.AsObject) => {
                    const kvs = Object.getOwnPropertyNames(JobPhase).map(k => [k, (JobPhase as any)[k]]).find(kv => kv[1] === row.phase);
                    return kvs![0].split("_")[1].toLowerCase();
                }
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
        const rows = Array.from(this.state.jobs.entries()).map(kv => kv[1]);

        const debounceSearch = debounce((s: any) => this.search(s), 500);
        const actions = <React.Fragment>
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
                    onChange={e => debounceSearch({_all: e.target.value})}
                />
            </div>
            
        </React.Fragment>

        const secondary = <Toolbar>
            <Grid container>
                <Grid item xs></Grid>
                <Grid item>
                    <Button className={classes.button} variant="outlined" color="inherit" size="small">
                        Start Job
                    </Button>
                </Grid>
            </Grid>
        </Toolbar>

        return <React.Fragment>
            <Header title="Jobs" actions={actions} secondary={secondary} />
            <main className={classes.main}>
                <Table>
                    <TableHead>
                        <TableRow>{columns.map(col =>
                            <TableCell key={col.property}>
                                {col.sort &&
                                    <TableSortLabel
                                        active={this.state.sortCol === col.property}
                                        onClick={() => this.sortColumn(col.property)}
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

        this.setState({ sortCol: col, sortAscending: sortAsc });
        this.search({});
    }

}

export const JobList = withStyles(styles)(JobListImpl);
