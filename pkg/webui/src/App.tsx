import React from 'react';
import {
    BrowserRouter as Router,
    Switch,
    Route,
    Link
} from "react-router-dom";
import { Grommet, Box, Button, Heading, Collapsible } from 'grommet';
import { Notification } from 'grommet-icons';
import { JobList } from './JobList';
import { JobView } from './JobView';

const theme = {
    "name": "my theme",
    "rounding": 4,
    "spacing": 24,
    "defaultMode": "light",
    "global": {
        "colors": {
            "brand": "#FFAA15",
            "background": {
                "dark": "#222222",
                "light": "#FFFFFF"
            },
            "background-strong": {
                "dark": "#000000",
                "light": "#FFFFFF"
            },
            "background-weak": {
                "dark": "#444444a0",
                "light": "#E8E8E880"
            },
            "background-xweak": {
                "dark": "#66666699",
                "light": "#CCCCCC90"
            },
            "text": {
                "dark": "#EEEEEE",
                "light": "#333333"
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
            "status-critical": "#FF4040",
            "status-warning": "#FFAA15",
            "status-ok": "#00C781",
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

    constructor() {
        super({})
        this.state = {};
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
                <Box direction='row' flex overflow={{ horizontal: 'hidden' }}>
                    <Switch>
                        <Route path="/job">
                            <JobView />
                        </Route>
                        <Route path="/">
                            <JobList />
                        </Route>
                    </Switch>
                </Box>
            </Grommet>
        </Router >
    }
}

