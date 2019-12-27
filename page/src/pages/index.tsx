import { Link } from 'gatsby';
import React from 'react';
import Helmet from 'react-helmet';
import { Waypoint } from 'react-waypoint';
import pic01 from '../assets/images/pic01.jpg';
import screenshot from '../assets/images/screenshot.png';
import logCutting from '../assets/images/log-cutting.png';
import results from '../assets/images/results.png';
import githubIntegration from '../assets/images/github-integration.png';
import cli from '../assets/images/cli.png';
import Header from '../components/Header';
import Layout from '../components/layout';
import Nav from '../components/Nav';
import SyntaxHighlighter from 'react-syntax-highlighter';
import { solarizedLight } from 'react-syntax-highlighter/dist/esm/styles/hljs';

interface IndexState {
    stickyNav: boolean;
}

class Index extends React.Component<{}, IndexState> {
    protected main?: HTMLElement;

    constructor(props) {
        super(props)
        this.state = {
            stickyNav: false,
        }
    }

    handleWaypointPositionChange = (args: Waypoint.CallbackArgs) => {
        this.setState({ stickyNav: args.currentPosition === 'above' });
    }

    render() {
        return <Layout>
            <Helmet title="werft - just Kubernetes-native CI" />
            <Header />

            <section id="intro" className="main">
                <img src={screenshot} alt="" width="100%" />
            </section>

            <Waypoint onPositionChange={this.handleWaypointPositionChange} />
            <Nav sticky={this.state.stickyNav} />
            <div id="main">
                <section id="first" className="main special">
                    <header className="major">
                        <h2>Really? Yet another CI system?</h2>
                    </header>
                    <p className="content">
                        Thing is, most existing CI systems do too much. You already have your build system, it's called yarn/go/bazel/maven/make/you-name-it.
                        There is little need for putting another layer on top of it, certainly not one that's reasonably inscruitable outside of that particular CI system.
                        That includes any kind of declarative business like YAML or DSLs, but also imperative ones like using Groovy or Lua.
                        <br /><br />
                        As of late CI systems want to control your infrastructure, introducing yet another means of deploying your services, databases and containers.
                        All this means that you end up duplicating your work, having to learn yet another custom resource definition, the kinks of yet another language.
                        <br /><br />
                        Werft is built in the spirit of Unix and microservices where services do one thing, and they do it well.
                        Werft executes jobs that run your build or deployment. It does not prescribe how that build ought to be structured, or make assumptions about your deployment.
                        At its core it starts, monitors and controls Kubernetes pods, effectively turning Kubernetes into your CI system.
                        Werft integrates well with GitHub, has a great CLI and UI, re-uses pods and Go templates (like Helm does).
                        If you are coming from a cloud-native world, you will feel right at home.
                        <br /><br />
                        We do not attempt to solve all the issues around building and deploying software. Instead, we let others do the great job they do - and enable you to use the software you have already in place.
                    </p>
                </section>

                <section id="features" className="main special">
                    <header className="major">
                        <h2>Features</h2>
                    </header>
                    <ul className="features">
                        <li>
                            <div className="highlight">
                                <SyntaxHighlighter language="yaml" style={solarizedLight} showLineNumbers={false}>{`pod:
  containers:
  - name: build
    image: golang:1.13-alpine
    workingDir: /workspace
    command:
    - sh 
    - -c
    - |
      echo "[webui|PHASE] build webui"
      make webui-static
      echo "[build|PHASE] build release"
      make release
      `}
                                </SyntaxHighlighter>
                            </div>
                            <div className="detail">
                                <h3>Jobs are Pods</h3>
                                <p>
                                    A werft job is nothing but a Kubernetes pod. In that pod you can run any container, any command, any script.
                                    This means you don't have to learn yet another pipeline language and figure out how to do get your stuff done.
                                    <br />
                                    Werft initializes and automatically adds a <code>/workspace</code> volume which contains your job context, e.g. your repo cloned from GitHub.
                                    <br />
                                    <br />
                                    One key benefit of the traditional pipeline apparoach of other CI systems is the structure it gives log output.
                                    In werft we use <a href="#structured-logging">structured logging</a> to recover process structure keep your job understandable.
                                </p>
                                <ul className="actions">
                                    <li>
                                        <Link to="/generic" className="button">Learn More</Link>
                                    </li>
                                </ul>
                            </div>
                        </li>
                        <li id="structured-logging">
                            <div className="highlight">
                                <img src={logCutting} alt="" />
                            </div>
                            <div className="detail">
                                <h3>Structured Logging</h3>
                                <p>
                                    Build systems produce a lot of log output which contains a lot of structure.
                                    In most CI system that structure is lost and your log degenerates to a flat text file.
                                </p>
                                <p>
                                    In Werft we parse the log output (log cutting) and maintain that structure on the UI.
                                    This way you can find errors quickly and get a good understanding of what's going on.
                                </p>
                                <ul className="actions">
                                    <li>
                                        <Link to="/generic" className="button">Learn More</Link>
                                    </li>
                                </ul>
                            </div>
                        </li>
                        <li>
                            <div className="highlight">
                                <img src={results} alt="" />
                            </div>
                                <div className="detail">
                                <h3>Results, results, results, ...</h3>
                                <p>
                                    If all things go well, your code builds, the tests pass, the deployment works, then you do not want to see your CI system.
                                    All you want then is to know about the results it produced, e.g. the npm package, the test installation or the Docker image that was just pushed.
                                </p>
                                <p>
                                    Werft jobs register their results through their log output. These results are shown on the job page, but can also be added to the commit status on GitHub, sent to Slack, or printed as smoke signs.
                                </p>
                                <ul className="actions">
                                    <li>
                                        <Link to="/generic" className="button">Learn More</Link>
                                    </li>
                                </ul>
                            </div>
                        </li>
                        <li>
                            <div className="highlight">
                                <img src={githubIntegration} alt="" />
                            </div>
                            <div className="detail">
                                <h3>GitHub integration</h3>
                                <p>
                                    GitOps is a thing. When you push your code to GitHub you want things to happen, and werft is the tool that makes them happen.
                                    Much like any other CI system on this planet, werft can listen to events on your repository, trigger jobs and register the results of those jobs as commit status back to GitHub.
                                </p>
                                <p>
                                    Thanks to werft's results mechanism, you can tell GitHub not just that your build passed, but also the URL of that shiny new dev-environment you just deployed.
                                </p>
                                <ul className="actions">
                                    <li>
                                        <Link to="/generic" className="button">Learn More</Link>
                                    </li>
                                </ul>
                            </div>
                        </li>
                        <li>
                            <div className="highlight">
                                <SyntaxHighlighter style={solarizedLight}>{`werft is a very simple GitHub triggered and Kubernetes powered CI system

Usage:
  werft [command]

Available Commands:
  help        Help about any command
  init        Initializes configuration for werft
  job         Interacts with currently running or previously run jobs
  run         Starts the execution of a job
  version     Prints the version of this binary

Flags:
  -h, --help          help for werft
      --host string   werft host to talk to (defaults to WERFT_HOST env var) (default "localhost:7777")
      --verbose       en/disable verbose logging

Use "werft [command] --help" for more information about a command.`}</SyntaxHighlighter>
                            </div>
                            <div className="detail">
                                <h3>Excellent Command-Line Integration</h3>
                                <p>
                                    CI is not all Git. Sometimes you want to start a job for a branch without having to commit that job, for example while setting things up.
                                    Maybe you want to integrate your CI system into your terminal, e.g. see the log output of the job started by your latest commit.
                                </p>
                                <p>
                                    Werft sports a CLI that supports those use-cases. Everything you can do on the UI, you also do on the CLI (and then some).
                                </p>
                                <ul className="actions">
                                    <li>
                                        <Link to="/generic" className="button">Learn More</Link>
                                    </li>
                                </ul>
                            </div>
                        </li>
                        <li>
                            <div className="highlight">
                                <SyntaxHighlighter language="yaml" style={solarizedLight} customStyle={{width:"100%"}} showLineNumbers={false}>{`plugins:
  - name: "example"
    type:
    - integration
    config:
      emoji: 🚀`}
                                </SyntaxHighlighter>
                            </div>
                            <div className="detail">
                                <h3>Extensible Plugin System</h3>
                                <p>
                                    There are a lot of things that werft cannot do out-of-the-box; and that's intentional.
                                    Werft is the counter proposal to feature-laden and complicated systems. However, that does not mean that werft cannot do those things.
                                </p>
                                <p>
                                    Werft sports a plugin system based on <a href="https://github.com/hashicorp/go-plugin">gRPC</a>.
                                    Those plugins can provide integration with other code-hosting platforms, add additional notifications or add a different logging format.
                                </p>
                                <ul className="actions">
                                    <li>
                                        <Link to="/generic" className="button">Learn More</Link>
                                    </li>
                                </ul>
                            </div>
                        </li>
                    </ul>
                </section>

                <section id="getting-started" className="main special">
                    <header className="major">
                        <h2>Getting started</h2>
                    </header>
                    <p>
                        <SyntaxHighlighter language="bash">{`# Werft runs on Kubernetes and installs using Helm:
helm repo add ...
helm install werft

# Next, get ahold of the werft CLI and create a job in your repo using
curl https://werft.dev/get-cli.sh | sh
werft init job hello-world

# Run that job using the werft CLI
werft run local -j .werft/hello-world.yaml`}
                        </SyntaxHighlighter>
                    </p>
                    <footer className="major">
                        <ul className="actions">
                            <li>
                                <Link to="/generic" className="button special">
                                    Learn More
                            </Link>
                            </li>
                        </ul>
                    </footer>
                </section>
            </div>
        </Layout>
    }

}

export default Index
