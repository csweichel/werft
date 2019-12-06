import * as React from 'react';
import { DataTable, DataTableProps, Text, Box, Grommet } from 'grommet';
import { Validate, StatusCritical } from 'grommet-icons';
import { WerftServiceClient } from './api/werft_pb_service';
import { JobStatus, ListJobsResponse, ListJobsRequest, JobPhase, SubscribeRequest, FilterExpression, FilterTerm, FilterOp } from './api/werft_pb';
import ReactTimeago from 'react-timeago';
import { theme } from './theme';
import { AppBar } from './components/AppBar';

interface JobListProps {
    client: WerftServiceClient;
}

type JobIndex = { [key: string]: JobStatus.AsObject };

interface JobListState {
    jobs: Map<string, JobStatus.AsObject>
}

type DataTableColumn = Pick<DataTableProps, "columns">['columns']

type DataTableFields = "name" | "owner" | "age" | "repo" | "phase" | "success";

export class JobList extends React.Component<JobListProps, JobListState> {

    constructor(props: JobListProps) {
        super(props);
        this.state = {
            jobs: new Map<string, JobStatus.AsObject>()
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
        } catch(err) {
            alert(err);
        }

        try {
            const req = new SubscribeRequest();
            let evts = this.props.client.subscribe(req);
            evts.on('end', () => alert("updates ended"));
            evts.on('data', r => {
                const status = r.getResult();
                if (!status) {
                    return;
                }

                const jobs = this.state.jobs || {};
                jobs.set(status.getName(), status.toObject());
                this.setState({jobs});
            });
        } catch(err) {
            alert(err);
        }
    }

    protected async search(query: any) {
        const req = new ListJobsRequest();
        req.setLimit(50);

        const filter = Object.getOwnPropertyNames(query).map(f => {
            const tf = new FilterExpression();
            const tt = new FilterTerm();
            tt.setField(f);
            tt.setOperation(FilterOp.OP_CONTAINS);
            tt.setValue(query[f]);
            tf.setTermsList([tt]);
            return tf;
        })
        req.setFilterList(filter);

        const resp = await new Promise<ListJobsResponse>((resolve, reject) => this.props.client.listJobs(req, (err, resp) => !!err ? reject(err) : resolve(resp!)));
        const jobs = resp.getResultList().map(r => r.toObject());
        
        const idx = new Map<string, JobStatus.AsObject>();
        jobs.forEach(j => idx.set(j.name, j));

        this.setState({ jobs: idx });
    }

    render() {
        const columns: DataTableColumn = [
            {
                property: "name",
                header: <Text>Name</Text>,
                primary: true,
                search: true,
                sortable: true,
                render: (row: JobStatus.AsObject) => {
                    return <a href={`/job/${row.name}`}>{row.name}</a>;
                }
            },
            {
                property: "owner",
                header: "Owner",
                search: true,
                sortable: true,
                render: (row: JobStatus.AsObject) => {
                    return row.metadata!.owner;
                }
            },
            {
                property: "age",
                header: "Age",
                sortable: true,
                render: (row: JobStatus.AsObject) => {
                    return <ReactTimeago date={row.metadata!.created!.seconds * 1000} />;
                }
            },
            {
                property: "repo.repo",
                header: "Repository",
                search: true,
                sortable: true,
                render: (row: JobStatus.AsObject) => {
                    const md = row.metadata!.repository!;
                    return `${md.host}/${md.owner}/${md.repo}`;
                }
            },
            {
                property: "phase",
                header: "Phase",
                search: true,
                sortable: true,
                render: (row: JobStatus.AsObject) => {
                    const kvs = Object.getOwnPropertyNames(JobPhase).map(k => [k, (JobPhase as any)[k]]).find(kv => kv[1] === row.phase);
                    return kvs![0].split("_")[1].toLowerCase();
                }
            },
            {
                property: "success",
                header: "Success",
                render: (row: JobStatus.AsObject) => {
                    let statusColor = 'status-unknown';
                    let icon = (c: string) => <StatusCritical />;

                    if (row.conditions!.success) {
                        statusColor = 'status-ok';
                        icon = (c: string) => <Validate color={c} />;
                    } else {
                        statusColor = 'status-critical';
                        icon = (c: string) => <StatusCritical color={c} />;
                    }

                    let color = 'status-unknown';
                    if (row.phase === JobPhase.PHASE_DONE) {
                        color = statusColor;
                    }
                    return icon(color);
                }
            }
        ]
        const rows = Array.from(this.state.jobs.entries()).map(kv => kv[1]);

        return <Grommet theme={theme} full>
            <AppBar />
            <Box direction='row' flex overflow={{ horizontal: 'hidden' }} pad={{ left: 'small', right: 'small', vertical: 'small' }}>
                <Box align="center" justify="center" fill>
                    <DataTable columns={columns} data={rows} onSearch={q => this.search(q)} sortable resizeable />
                </Box> 
            </Box>
        </Grommet>
    }

}