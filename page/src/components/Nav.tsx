import React, { Ref } from 'react'
import Scrollspy from 'react-scrollspy'
import Scroll from './Scroll'

export interface NavProps {
    sticky: boolean;
}

const Nav: React.SFC<NavProps> = React.forwardRef((props, ref: Ref<HTMLElement>) => (
    <nav id="nav" className={props.sticky ? 'alt' : ''} ref={ref}>
        <Scrollspy items={ ['first', 'features', 'getting-started'] } currentClassName="is-active" offset={-300}>
            <li>
                <Scroll type="id" element="first">
                    <a href="#intro">Yet another CI system?</a>
                </Scroll>
            </li>
            <li>
                <Scroll type="id" element="features">
                    <a href="#features">Features</a>
                </Scroll>
            </li>
            <li>
                <Scroll type="id" element="getting-started">
                    <a href="#getting-started">Getting Started</a>
                </Scroll>
            </li>
        </Scrollspy>
    </nav>
))

export default Nav
