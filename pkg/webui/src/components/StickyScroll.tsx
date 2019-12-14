
import * as React from 'react';

interface StickyScrollState {
    enabled: boolean
}

export interface StickyScrollProps {
};

export class StickyScroll extends React.Component<StickyScrollProps, StickyScrollState> {
    protected endOfLine: HTMLDivElement | null = null;
    protected container: HTMLDivElement | null = null;

    constructor(p: StickyScrollProps) {
        super(p);
        this.state = {
            enabled: false
        };
        this.onScroll = this.onScroll.bind(this);
    }

    componentDidMount() {
        this.scrollToBottom();
        window.addEventListener('scroll', this.onScroll);
    }

    componentWillUnmount() {
        window.removeEventListener('scroll', this.onScroll);
    }

    componentDidUpdate() {
        this.scrollToBottom();
    }

    protected scrollToBottom() {
        if (!this.state.enabled) {
            return;
        }
        if (!this.endOfLine) {
            return;
        }

        this.endOfLine.scrollIntoView({ behavior: "smooth" });
    }

    protected onScroll() {
        let stick: boolean;
        if (this.state.enabled) {
            stick = (window.innerHeight + window.pageYOffset) >= document.body.offsetHeight - 500;
        } else {
            stick = (window.innerHeight + window.pageYOffset) >= document.body.offsetHeight - 100;
        }

        if (this.state.enabled !== stick) {
            this.setState({enabled: stick});
        }
    }

    render() {
        return <React.Fragment>
            <div ref={el => this.container = el}>
                {this.props.children}<div ref={el => this.endOfLine = el} />
            </div>
        </React.Fragment>
    }
}