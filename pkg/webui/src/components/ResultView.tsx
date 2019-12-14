import { Theme, createStyles, WithStyles, List, ListItem, ListItemText, Link, ListItemAvatar } from "@material-ui/core";
import { JobStatus } from '../api/werft_pb';
import * as React from 'react';
import { withStyles } from "@material-ui/styles";
import ReceiptIcon from '@material-ui/icons/ReceiptOutlined';
import LinkIcon from '@material-ui/icons/Link';
import { InlineIcon } from "@iconify/react";
import docckerIcon from "@iconify/icons-mdi/docker";

export const styles = (theme: Theme) =>
    createStyles({
        
    });

export interface ResultViewProps extends WithStyles<typeof styles> {
    status?: JobStatus.AsObject
}

const ResultViewImpl: React.SFC<ResultViewProps> = (props) => {
    if (!props.status) {
        return <React.Fragment />;
    }

    return <List>
        { props.status.resultsList.map((r, i) => (
            <ListItem key={i}>
                {renderIcon(r.type)}
                <ListItemText primary={renderPayload(r.type, r.payload)} secondary={r.description} />
            </ListItem>
        )) }
    </List>;
};

function renderIcon(type: string) {
    let icon: JSX.Element;
    switch (type) {
        case "url":    icon = <LinkIcon />; break;
        case "docker": icon = <InlineIcon icon={docckerIcon} height="32px" />; break;
        default:       icon = <ReceiptIcon />; break;
    }
    return <ListItemAvatar>
        {icon}
    </ListItemAvatar>
}

function renderPayload(type: string, payload: string) {
    switch (type) {
        case "url": return <Link href={payload}>{payload}</Link>
        case "docker": return <code>docker pull <b>{payload}</b></code>
    }
}

export const ResultView = withStyles(styles)(ResultViewImpl);
