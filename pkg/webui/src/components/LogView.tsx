import * as React from "react";
import { LogSliceEvent, LogSlicePhase } from "../api/keel_pb";
import { Box, Heading } from "grommet";

export interface LogViewProps {
    logs: LogSliceEvent[];
}

export interface LogViewState {
    chunks: Map<string, string[]>;
}

export class LogView extends React.Component<LogViewProps, LogViewState> {
    
    constructor(props: LogViewProps) {
        super(props);
        this.state = {
            chunks: new Map<string, string[]>()
        }

        this.updateChunks();
    }

    componentDidUpdate() {
        this.updateChunks();
    }

    protected updateChunks() {
        let currentChunk = "default";
        let chunks = this.state.chunks;
        let icCount = 0;

        this.props.logs.forEach(le => {
            if (le.getPhase() === LogSlicePhase.SLICE_START) {
                currentChunk = le.getName();
                icCount = 0;
            } else if (le.getPhase() === LogSlicePhase.SLICE_CONTENT) {
                const content = (chunks.get(currentChunk) || []);
                if (icCount++ <= content.length) {
                    return
                }

                content.push(le.getPayload());
                chunks.set(currentChunk, content);
            }
        });
    }

    render() {
        return <React.Fragment>
            { Array.from(this.state.chunks.entries()).map(kv => (
                <Box key={kv[0]}>
                    <Heading level="6">{kv[0]}</Heading>
                    <div className="term-container" dangerouslySetInnerHTML={{__html: kv[1].join("<br />")}} />
                </Box>
            )) }
        </React.Fragment>
    }
 
}