
import * as React from 'react';

interface StickyScrollState {
}

export interface StickyScrollProps {
    enabled: boolean
};

export class StickyScroll extends React.Component<StickyScrollProps, StickyScrollState> {
    protected endOfLine: HTMLDivElement | null = null;
    protected container: HTMLDivElement | null = null;

    componentDidMount() {
        this.scrollToBottom();
    }

    componentDidUpdate() {
        this.scrollToBottom();
    }

    protected scrollToBottom() {
        if (!this.props.enabled) {
            return;
        }
        if (!this.endOfLine) {
            return;
        }

        this.endOfLine.scrollIntoView({ behavior: "smooth" });
    }

    render() {
        return <React.Fragment>
            <div ref={el => this.container = el}>
                {this.props.children}<div ref={el => this.endOfLine = el} />
            </div>
        </React.Fragment>
    }
}