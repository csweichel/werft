import React from 'react';
import AppBar from '@material-ui/core/AppBar';
import Grid from '@material-ui/core/Grid';
import Toolbar from '@material-ui/core/Toolbar';
import Typography from '@material-ui/core/Typography';
import { createStyles, Theme, withStyles, WithStyles } from '@material-ui/core/styles';

const lightColor = 'rgba(255, 255, 255, 0.7)';

export const headerStyles = (theme: Theme) =>
    createStyles({
        mainBar: {
            zIndex: 999,
        },
        secondaryBar: {
            paddingTop: '0.5em',
            zIndex: 2,
        },
        thirdBar: {
            zIndex: 1,
        },
        menuButton: {
            marginLeft: -theme.spacing(1),
        },
        iconButtonAvatar: {
            padding: 4,
        },
        link: {
            textDecoration: 'none',
            color: lightColor,
            '&:hover': {
                color: theme.palette.common.white,
            },
        },
        button: {
            // borderColor: lightColor,
        },
    });

export interface HeaderProps extends WithStyles<typeof headerStyles> {
    title: string
    color?: string
    actions?: React.ReactFragment
    secondary?: React.ReactFragment
    thirdrow?: React.ReactFragment
}

interface HeaderState {}

class HeaderImpl extends React.Component<HeaderProps, HeaderState> {

    render() {
        const { classes } = this.props;
        let appbarStyle = {};
        if (this.props.color) {
            appbarStyle = { backgroundColor: this.props.color};
        }

        return (
            <React.Fragment>
                <AppBar
                    component="div"
                    className={classes.mainBar}
                    color="primary"
                    position="sticky"
                    elevation={0}
                    style={appbarStyle}
                >
                    <Toolbar>
                        <Grid container alignItems="center" spacing={1}>
                            <Grid item>
                                <Typography color="inherit" variant="h5" component="h2">
                                    {this.props.title}
                                </Typography>
                            </Grid>
                            {this.props.actions}
                        </Grid>
                    </Toolbar>
                </AppBar>
                { this.props.secondary && 
                    <AppBar
                        component="div"
                        className={classes.secondaryBar}
                        color="primary"
                        position="static"
                        elevation={0}
                        style={appbarStyle}
                >{this.props.secondary}</AppBar>
                }
                { this.props.thirdrow && 
                    <AppBar
                        component="div"
                        className={classes.thirdBar}
                        color="primary"
                        position="static"
                        elevation={0}
                        style={appbarStyle}
                >{this.props.thirdrow}</AppBar>
                }
            </React.Fragment>
        )
    }
}

export const Header = withStyles(headerStyles)(HeaderImpl);
