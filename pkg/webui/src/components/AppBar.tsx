import * as React from "react";
import { Box, Heading } from "grommet";
import { Previous } from 'grommet-icons';

export interface AppBarProps {
    backLink?: string;
    backgroundColor?: string;
}

export class AppBar extends React.Component<AppBarProps> {

    render() {
        return <Box
            tag='header'
            direction='row'
            align='center'
            justify='between'
            background={this.props.backgroundColor || 'none'}
            pad={{ left: 'medium', right: 'small', vertical: 'small' }}
            style={{ zIndex: 1 }}>

            { this.props.backLink && <a href={this.props.backLink}><Previous /></a> }
            {this.props.children}
            <Heading level='4' margin='none'>keel</Heading>
        </Box>
    }

}