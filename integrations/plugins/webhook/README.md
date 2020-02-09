This plugin HTTP POST's messages to a URL.
For example, to send messages to a Slack webhook:
```YAML
url: https://your-webhook.slack.com
contentType: application/json
filter: 
- phase==done
template: '{"text": "{{ .Name }} done"}'
```

The template receives a [JobStatus](https://godoc.org/github.com/csweichel/werft/pkg/api/v1#JobStatus) as context.