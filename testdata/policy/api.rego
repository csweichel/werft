package werft

default allow = false

allow {
    input.method == "/v1.WerftService/ListJobs"
}
allow {
    input.auth.known
}
