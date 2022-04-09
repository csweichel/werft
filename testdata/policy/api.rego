package werft

default allow = false

allow {
    input.method == "/v1.WerftService/ListJobs"
}
allow {
    input.method == "/v1.WerftService/Listen"
}

# Allow running GitHub jobs with sideloading only for team members and not on main
allow {
    is_team_member

    input.method == "/v1.WerftService/StartGitHubJob"
    input.message.sideload != ""
    not startswith(input.message.metadata.repository.ref, "refs/heads/main")
}
# Allow running GitHub jobs with custom jobs only for team members and not on main
allow {
    is_team_member

    input.method == "/v1.WerftService/StartGitHubJob"
    input.message.job_yaml != ""
    not startswith(input.message.metadata.repository.ref, "refs/heads/main")
}
# Allow running GitHub jobs on all branches without sideloading/custom jobs
allow {
    is_team_member

    input.method == "/v1.WerftService/StartGitHubJob"
    not input.message.job_yaml
    not input.message.job_path
    not input.message.sideload
}

# Allow team members to run previously started jobs
allow {
    is_team_member

    input.method == "/v1.WerftService/StartFromPreviousJob"
}

is_team_member {
    input.auth.known
    endswith(input.auth.emails[_], "@gitpod.io")
}
