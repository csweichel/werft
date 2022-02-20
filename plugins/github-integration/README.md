# GitHub integration

This plugin provides GitHub integration using a GitHub app. It can trigger builds when a branch is pushed,
pull annotations from a PR, take commands from a PR comment and update the commit status.

## Installation
First you must create a GitHub app with the following permissions:
- Deployments: Read & Write
- Issues: Read & Write
- Metadata: Read-only
- Pull Requests: Read & Write
- Commit Status: Read & Write

subscribing to the following events:
- Meta
- Issue Comment
- Push
- Pull Request

Once you have created this application, please install it on the repositories you intent to use werft with.

Then add the following to your werft config file:
```YAML
plugins:
  - name: "github-integration"
    type:
    - integration
    config:
      baseURL: https://your-werft-installation-url.com
      webhookSecret: choose-a-sensible-secret-here
      privateKeyPath: path-to-your/app-private-key.pem
      appID: 00000              # appID of your GitHub app
      installationID: 0000000   # installation ID of your GitHub app installation
      jobProtection: "default-branch"
      pullRequestComments:
        enabled: true
        updateComment: true
        requiresOrg: []
        requiresWriteAccess: true
```

### Job Protection
By default this plugin will pull the job config, werft job and `.werft/` directory from the branch it's building.
If `jobProtection` is set to `default-branch`, those files will be taken from the repository's default branch instead.

## PR Commands
This integration plugin listens for comments on PRs to trigger operations in werft.

```YAML
# start a werft job for this PR
/werft run
```

## Commit Checks
For all jobs that carry the `updateGitHubStatus` annotation, werft attempts to add a commit check on the repository pointed to in that annotation. E.g. if the job ran with `updateGitHubStatus=csweichel/werft`, upon completion of that job, this plugin would add a check indiciating job success or failure.
By default, all jobs started using this integration plugin (push events or comments) will carry this annotation.

In addition to the job success annotation, jobs can add additional checks to commits. Any result posted to the `github` or any channel starting with `github-check-` will become a check on the commit. Examples:

- ```
  werft log result -d "dev installation" -c github url http://foobar.com
  ``` 
  adds the following check (`Details` points to `http://foobar.com`)
  ![](docs/check-screenshot.png)

- ```
  werft log result -d "dev installation" -c github-check-tests conclusion success
  ```
  would add a successful check named `continuous-integration/werft/result-tests`, whereas 
  ```
  werft log result -d "dev installation" -c github-check-tests conclusion failure
  ```
  would add a failed check named `continuous-integration/werft/result-tests`.

  Valid values for `conclusion` results in this case are listed in the [GitHub API docs](https://docs.github.com/en/rest/reference/checks#update-a-check-run).