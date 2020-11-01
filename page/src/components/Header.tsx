import React from 'react'

import logo from '../assets/images/logo_path.svg';

const Header: React.SFC<{}> = (props) => (
    <header id="header" className="alt">
        <span className="logo"><img src={logo} alt="" height="200px" /></span>
        <h1>Just Kubernetes Native CI</h1>
        <p>Because your job is hard enough.</p>
        <ul class="actions">
            <li><a class="button primary" href="#getting-started">Getting started</a></li>
            <li><a class="button icon solid fa-github" target="_blank" href="https://github.com/csweichel/werft">View on GitHub</a></li>
        </ul>
    </header>
)

export default Header
