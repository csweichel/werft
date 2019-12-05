import React from 'react';
import {
    BrowserRouter as Router,
    Switch,
    Route,
} from "react-router-dom";
import { Grommet, Box, Button, Heading } from 'grommet';
import { Notification } from 'grommet-icons';
import { JobList } from './JobList';
import { JobView } from './JobView';
import { KeelServiceClient } from './api/keel_pb_service';

const theme = {
    "name": "my theme",
    "rounding": 4,
    "spacing": 24,
    "defaultMode": "light",
    "global": {
        "colors": {
            "brand": "#FDC953",
            "background": {
                "dark": "#080300",
                "light": "#FFFFFF"
            },
            "background-strong": {
                "dark": "#000000",
                "light": "#FFFFFF"
            },
            "background-weak": {
                "dark": "#6F6B68",
                "light": "#E7E7E7"
            },
            "background-xweak": {
                "dark": "#66666699",
                "light": "#CCCCCC90"
            },
            "text": {
                "dark": "#EEEEEE",
                "light": "#080300"
            },
            "text-strong": {
                "dark": "#FFFFFF",
                "light": "#000000"
            },
            "text-weak": {
                "dark": "#CCCCCC",
                "light": "#444444"
            },
            "text-xweak": {
                "dark": "#999999",
                "light": "#666666"
            },
            "border": "background-xweak",
            "control": "brand",
            "active-background": "background-weak",
            "active-text": "text-strong",
            "selected-background": "background-strong",
            "selected-text": "text-strong",
            "status-critical": "#F75E60",
            "status-warning": "#FDC854",
            "status-ok": "#2EC990",
            "status-unknown": "#CCCCCC",
            "status-disabled": "#CCCCCC"
        },
        "font": {
            "family": "Helvetica"
        },
        "graph": {
            "colors": {
                "dark": [
                    "brand"
                ],
                "light": [
                    "brand"
                ]
            }
        }
    }
};

const AppBar = (props: any) => (
    <Box
        tag='header'
        direction='row'
        align='center'
        justify='between'
        background='none'
        pad={{ left: 'medium', right: 'small', vertical: 'small' }}
        // elevation='medium'
        style={{ zIndex: '1' }}
        {...props}
    />
);

interface AppState {
    showSidebar?: boolean
}

export default class App extends React.Component<{}, AppState> {
    protected readonly client: KeelServiceClient;

    constructor(p: {}) {
        super(p)
        this.state = {};

        let url = window.location.href;
        url = url.substring(0, url.length-1);
        this.client = new KeelServiceClient(url);
    }

    render() {
        return <Router>
            <Grommet theme={theme} full>
                <AppBar>
                    <Heading level='4' margin='none'>keel</Heading>
                    <Button
                        icon={<Notification />}
                        onClick={() => this.setState(prevState => ({ showSidebar: !prevState.showSidebar }))}
                    />
                </AppBar>
                <Box direction='row' flex overflow={{ horizontal: 'hidden' }} pad={{ left: 'small', right: 'small', vertical: 'small' }}>
                    <Switch>
                        <Route path="/job">
                            <JobView />
                        </Route>
                        <Route path="/">
                            <JobList client={this.client} />
                        </Route>
                    </Switch>
                </Box>
            </Grommet>
        </Router >
    }
}

