import React from 'react'

import logo from '../assets/images/logo_path.svg';

const Header: React.SFC<{}> = (props) => (
    <header id="header" className="alt">
        <span className="logo"><img src={logo} alt="" height="200px" /></span>
        <h1>Just Kubernetes Native CI</h1>
        <p>Because your job is hard enough.</p>
        <a class="button primary" href="#getting-started">Getting started</a>
        <a class="button icon solid fa-github source" target="_blank" href="https://github.com/csweichel/werft">View on GitHub</a>
    </header>
)

export default Header
