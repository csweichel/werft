import * as React from 'react';
import { WerftServiceClient } from './api/werft_pb_service';
import { JobStatus, GetJobRequest, GetJobResponse, LogSliceEvent, ListenRequest, ListenRequestLogs } from './api/werft_pb';
import { Grommet, Box, Text, Table, TableBody, TableRow, TableCell, Collapsible, Heading } from 'grommet';
import { theme } from './theme';
import { AppBar } from './components/AppBar';
import ReactTimeago from 'react-timeago';
import './components/terminal.css';
import { LogView } from './components/LogView';

export interface JobViewProps {
    client: WerftServiceClient;
    jobName: string;
}

interface JobViewState {
    status?: JobStatus.AsObject
    showDetails: boolean;
    log: LogSliceEvent[];
}

export class JobView extends React.Component<JobViewProps, JobViewState> {

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
        evts.on('data', h => {
            if (!h.hasSlice()) {
                return;
            }

            const log = this.state.log;
            console.log(log);
            log.push(h.getSlice()!);
            this.setState({ log });
        });
        evts.on('end', console.log);
    }

    render() {
        let color = 'status-unknown';
        if (this.state.status && this.state.status.conditions) {
            if (this.state.status.conditions.success) {
                color = 'status-ok';
            } else {
                color = 'status-critical';
            }
        }

        const job = this.state.status;
        let metadata: ({
            label: string
            value: string | React.ReactFragment
        })[][] | undefined;
        if (!!job) {
            metadata = [
                [
                    { label: "Owner", value: job.metadata!.owner },
                    { label: "Started", value: <React.Fragment><ReactTimeago date={job.metadata!.created.seconds * 1000} /></React.Fragment> },
                ],
                [
                    { label: "Repository", value: `${job.metadata!.repository!.host}/${job.metadata!.repository!.owner}/${job.metadata!.repository!.repo}` },
                    !!job.metadata!.finished ? { label: "Finished", value: <React.Fragment><ReactTimeago date={job.metadata!.finished.seconds * 1000} /></React.Fragment> } : { label: "", value: "" }
                ],
                [
                    { label: "Revision", value: job.metadata!.repository!.ref }
                ]
            ];
        }

        return <Grommet theme={theme} full>
            <AppBar backLink="/" backgroundColor={color}>
                <Text>{this.props.jobName}</Text>
            </AppBar>
            <Box direction='row' flex overflow={{ horizontal: 'hidden' }} pad={{ left: 'small', right: 'small', vertical: 'small' }}>
                <Box fill>
                    {job &&
                        <Box>
                            <Heading level="4" onClick={() => this.setState({ showDetails: !this.state.showDetails })} style={{ cursor: "pointer" }}>Details</Heading>
                            <Collapsible open={this.state.showDetails}>
                                <Table>
                                    <TableBody>
                                        {metadata && metadata.map((rs, i) => (
                                            <TableRow key={i}>{
                                                rs.map((p, j) => (
                                                    <React.Fragment key={j}>
                                                        <TableCell><Text style={{ fontWeight: "bold" }}>{p!.label}</Text></TableCell>
                                                        <TableCell><Text>{p!.value}</Text></TableCell>
                                                    </React.Fragment>
                                                ))
                                            }</TableRow>
                                        ))}
                                    </TableBody>
                                </Table>
                            </Collapsible>
                        </Box>
                    }

                    <Box>
                        <Heading level="4">Logs</Heading>
                        <LogView logs={this.state.log} />
                    </Box>
                </Box>
            </Box>
        </Grommet>
    }

}