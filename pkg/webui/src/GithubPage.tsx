import * as React from 'react';
import { headerStyles } from './components/header';
import { createStyles, Theme, Typography, Button } from '@material-ui/core';
import { WithStyles, withStyles } from '@material-ui/styles';


const styles = (theme: Theme) => createStyles({
    main: {
        flex: 1,
        padding: theme.spacing(6, 4),
        background: '#eaeff1',
    },
    button: headerStyles(theme).button,
    metadataItemLabel: {
        fontWeight: "bold",
        paddingRight: "0.5em"
    },
    infobar: {
        paddingBottom: "1em"
    }
});

export interface GithubPageProps extends WithStyles<typeof styles> {
}

interface GithubPageState {
}

class GithubPageImpl extends React.Component<GithubPageProps, GithubPageState> {
    
    render() {
        const params = new URLSearchParams(window.location.search);
        const isInstallation = params.get('setup_action') === "install"

        return <React.Fragment>
            <main className={this.props.classes.main}>
                <Typography variant="h2">Welcome.</Typography>
                { isInstallation && <React.Fragment>
                    <Typography>It would appear you have just installed a GitHub app pointing to this werft installation. To make this installation work you'll need to change the configuration of this installation to include the following:</Typography>
                    <pre>
                    {`
{
    "webhookSecret": "<your-webhook-secret>",
    "privateKeyPath": "<path-to-your-keyfile>",
    "appID": <your-app-id>,
    "installationID": ${params.get('installation_id')}
}
`}
                    </pre>
                </React.Fragment> }
                <Button href="/" variant="outlined">Back to dashboard</Button>
            </main>
        </React.Fragment>
    }

}

export const GithubPage = withStyles(styles)(GithubPageImpl);
