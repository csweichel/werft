import React from 'react';
import clsx from 'clsx';
import { createStyles, Theme, withStyles, WithStyles } from '@material-ui/core/styles';
import Divider from '@material-ui/core/Divider';
import Drawer, { DrawerProps } from '@material-ui/core/Drawer';
import List from '@material-ui/core/List';
import ListItem from '@material-ui/core/ListItem';
import ListItemIcon from '@material-ui/core/ListItemIcon';
import ListItemText from '@material-ui/core/ListItemText';
import HomeIcon from '@material-ui/icons/Home';
import PlayArrowIcon from '@material-ui/icons/PlayArrow';
import { Omit } from '@material-ui/types';

const categories = [
    {
        id: 'Control',
        children: [
            { id: 'Launch', icon: <PlayArrowIcon />, active: false },
        ],
    },
];

const styles = (theme: Theme) =>
    createStyles({
        categoryHeader: {
            paddingTop: theme.spacing(2),
            paddingBottom: theme.spacing(2),
        },
        categoryHeaderPrimary: {
            color: theme.palette.common.white,
        },
        item: {
            paddingTop: 1,
            paddingBottom: 1,
            color: 'rgba(255, 255, 255, 0.7)',
            '&:hover,&:focus': {
                backgroundColor: 'rgba(255, 255, 255, 0.08)',
            },
        },
        itemCategory: {
            backgroundColor: '#232f3e',
            boxShadow: '0 -1px 0 #404854 inset',
            paddingTop: theme.spacing(2),
            paddingBottom: theme.spacing(2),
        },
        firebase: {
            fontSize: 24,
            color: theme.palette.common.white,
        },
        itemActiveItem: {
            color: '#4fc3f7',
        },
        itemPrimary: {
            fontSize: 'inherit',
        },
        itemIcon: {
            minWidth: 'auto',
            marginRight: theme.spacing(2),
        },
        divider: {
            marginTop: theme.spacing(2),
        },
    });

export interface NavigatorProps extends Omit<DrawerProps, 'classes'>, WithStyles<typeof styles> { }


class NavigatorImpl extends React.Component<NavigatorProps, {}> {

    render() {
        const { classes, ...other } = this.props;
        return (
            <Drawer variant="permanent" {...other}>
                <List disablePadding>
                    <ListItem className={clsx(classes.firebase, classes.item, classes.itemCategory)}>werft</ListItem>
                    <ListItem className={clsx(classes.item, classes.itemCategory)}>
                        <ListItemIcon className={classes.itemIcon}><HomeIcon /></ListItemIcon>
                        <ListItemText classes={{ primary: classes.itemPrimary }}>Jobs</ListItemText>
                    </ListItem>
                    {categories.map(({ id, children }) => (
                        <React.Fragment key={id}>
                            <ListItem className={classes.categoryHeader}>
                                <ListItemText classes={{primary: classes.categoryHeaderPrimary}}>{id}</ListItemText>
                            </ListItem>
                            {children.map(({ id: childId, icon, active }) => (
                                <ListItem
                                    key={childId}
                                    button
                                    className={clsx(classes.item, active && classes.itemActiveItem)}
                                >
                                    <ListItemIcon className={classes.itemIcon}>{icon}</ListItemIcon>
                                    <ListItemText
                                        classes={{
                                            primary: classes.itemPrimary,
                                        }}
                                    >
                                        {childId}
                                    </ListItemText>
                                </ListItem>
                            ))}
                            <Divider className={classes.divider} />
                        </React.Fragment>
                    ))}
                </List>
            </Drawer>
        );
    }
}

export const Navigator = withStyles(styles)(NavigatorImpl);
