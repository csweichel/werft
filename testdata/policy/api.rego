package werft

default allow = false

allow {
    input.method == "/v1.WerftService/ListJobs"
}
allow {
    input.method == "/v1.WerftService/Listen"
}

# all other things are only allowed when the user is authenticated and a Gitpod employee
allow {
    is_team_member

    input.method == "/v1.WerftService/StartGitHubJob"
    input.message.sideload != ""
    not startswith(input.message.metadata.repository.ref, "refs/heads/main")
}
allow {
    is_team_member

    input.method == "/v1.WerftService/StartGitHubJob"
    input.message.job_yaml != ""
    not startswith(input.message.metadata.repository.ref, "refs/heads/main")
}
allow {
    is_team_member

    input.method == "/v1.WerftService/StartGitHubJob"
    input.message.job_yaml == ""
    input.message.job_path == ""
    input.message.sideload == ""
}

is_team_member[auth] {
    input.auth.known
    endswith(auth.emails[_], "@gitpod.io")
}