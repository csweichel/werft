import * as React from 'react';
import { DataTable, DataTableProps, Text, Box } from 'grommet';
import { Validate, StatusCritical } from 'grommet-icons';
import { KeelServiceClient } from './api/keel_pb_service';
import { JobStatus, ListJobsResponse, ListJobsRequest, JobPhase, SubscribeRequest } from './api/keel_pb';
import ReactTimeago from 'react-timeago';

interface JobListProps {
    client: KeelServiceClient;
}

type JobIndex = { [key: string]: JobStatus.AsObject };

interface JobListState {
    jobs: Map<string, JobStatus.AsObject>
}

type DataTableColumn = Pick<DataTableProps, "columns">['columns']

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

    render() {
        const columns: DataTableColumn = [
            {
                property: "name",
                header: <Text>Name</Text>,
                primary: true,
                search: true
            },
            {
                property: "owner",
                header: "Owner",
                render: (row: JobStatus.AsObject) => {
                    return row.metadata!.owner;
                }
            },
            {
                property: "age",
                header: "Age",
                render: (row: JobStatus.AsObject) => {
                    return <ReactTimeago date={row.metadata!.created!.seconds * 1000} />;
                }
            },
            {
                property: "repo",
                header: "Repository",
                render: (row: JobStatus.AsObject) => {
                    const md = row.metadata!.repository!;
                    return `${md.host}/${md.owner}/${md.repo}`;
                }
            },
            {
                property: "phase",
                header: "Phase",
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

        return <Box align="center" justify="center" fill>
            <DataTable columns={columns} data={rows} onSearch={() => {}} sortable={false} />
        </Box>
    }

}