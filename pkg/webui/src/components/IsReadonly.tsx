import React, { Children } from 'react';
import { WerftUIClient } from "../api/werft-ui_pb_service";
import { IsReadOnlyRequest } from '../api/werft-ui_pb';

export interface IsReadonlyProps {
    uiClient: WerftUIClient;
}

export interface IsReadonlyState {
    readonly: boolean;
}

export class IsReadonly extends React.Component<IsReadonlyProps, IsReadonlyState> {

    constructor(props: IsReadonlyProps) {
        super(props);
        this.state = { readonly: true };
    }

    componentDidMount() {
        try {
            this.props.uiClient.isReadOnly(new IsReadOnlyRequest(), (err, msg) => {
                if (err) {
                    console.warn("cannot determine if UI is readonly", err);
                    return;
                }

                this.setState({ readonly: msg!.getReadonly() });
            });
        } catch (err) {
            console.warn(err);
        }
    }

    render() {
        return Children.map(this.props.children, c => React.isValidElement(c) ? React.cloneElement(c, { readonly: this.state.readonly }) : undefined);
    }

}