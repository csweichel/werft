import * as React from 'react';
import { DataTable, DataTableProps, Text, Box } from 'grommet';
import { Validate, StatusCritical } from 'grommet-icons';
import { KeelServiceClient } from './api/keel_pb_service';
import { JobStatus, ListJobsResponse, ListJobsRequest, JobPhase } from './api/keel_pb';

interface JobListProps {
    client: KeelServiceClient;
}

interface JobListState {
    jobs?: JobStatus.AsObject[]
}

type DataTableColumn = Pick<DataTableProps, "columns">['columns']

export class JobList extends React.Component<JobListProps, JobListState> {

    constructor(props: JobListProps) {
        super(props);
        this.state = {};
    }

    async componentDidMount() {
        const req = new ListJobsRequest();
        req.setLimit(50);

        try {
            const resp = await new Promise<ListJobsResponse>((resolve, reject) => this.props.client.listJobs(req, (err, resp) => !!err ? reject(err) : resolve(resp!)));
            const jobs = resp.getResultList().map(r => r.toObject());
            this.setState({ jobs });
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
                property: "phase",
                header: "Phase",
                render: (row: JobStatus.AsObject) => {
                    const kvs = Object.getOwnPropertyNames(JobPhase).map(k => [k, (JobPhase as any)[k]]).find(kv => kv[1] == row.phase);
                    return kvs![0].split("_")[1].toLowerCase();
                }
            },
            {
                property: "success",
                header: "Success",
                render: row => {
                    let statusColor = 'status-unknown';
                    let icon = (c: string) => <StatusCritical />;

                    if (row.success) {
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
        // const rows = [
        //     {name: "gpl/helm-stuff.1", phase: "done", success: true},
        //     {name: "cw/debug.2", phase: "running", success: false},
        //     {name: "cw/debug.1", phase: "done", success: false},
        // ]

        return <Box align="center" justify="center" fill>
            <DataTable columns={columns} data={this.state.jobs} onSearch={() => {}} sortable={false} />
        </Box>
    }

}