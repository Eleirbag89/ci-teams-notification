# Microsoft Teams Notification 

Docker image / CI plugin to send pipeline notifications to Microsoft Teams using Adaptive Cards. Designed to work with Woodpecker CI, but can be used with any CI system that supports Docker images.

## Features

- Customizable notification card with:
  - Pipeline status (success/failure)
  - Author information with avatar
  - Commit/build details
  - Configurable fact sections
  - Optional variables table
  - Customizable action buttons
- Base64-encoded avatar images
- Debug mode

## Configuration

### Basic Configuration

```yaml
steps:
  - name: notify-teams
    image: mobydeck/ci-teams-notification
    settings:
      webhook_url: https://outlook.office.com/webhook/...
    when:
      - status: [success, failure]
        event: [manual, push, tag]
```

### Environment Variables

The plugin uses environment variables:

- `CI_REPO` - Repository name
- `CI_REPO_URL` - Repository URL
- ~~`CI_PIPELINE_STATUS`~~ `DRONE_BUILD_STATUS` - Pipeline status
  * `CI_PIPELINE_STATUS` is missing in 3.1.0 :(
- `CI_PIPELINE_URL` - Pipeline URL
- `CI_PIPELINE_FORGE_URL` - Forge commit URL
- `CI_COMMIT_SHA` - Commit SHA (shortened to 7 characters)
- `CI_COMMIT_TAG` - Release tag (if available)
- `CI_COMMIT_MESSAGE` - Commit message
- `CI_COMMIT_AUTHOR` - Commit author
- `CI_COMMIT_AUTHOR_AVATAR` - Author's avatar URL



### Plugin Settings

- `webhook_url` (required) - Microsoft Teams webhook URL
- `debug` (optional) - Enable debug output of the card JSON
- `facts` (optional) - Comma-separated list of facts to display:
  - `project` - Repository name
  - `message` - Commit message
  - `version` - Version/tag/commit
  - Default: all facts are shown
- `buttons` (optional) - Comma-separated list of buttons to display:
  - `pipeline` - Link to pipeline
  - `commit` - Link to commit (for non-tag builds)
  - `release` - Link to release (for tag builds)
  - Default: all buttons are shown
- `variables` (optional) - Comma-separated list of environment variables to display in a table

### Example Configuration

```yaml
steps:
  - name: notify-teams
    image: mobydeck/ci-teams-notification
    settings:
      webhook_url:
        from_secret: teams_webhook_url
      facts: project,version
      buttons: pipeline,commit
      variables: MY_VAR1,MY_VAR2
      debug: true
    when:
      - status: [success, failure]
        event: [manual, push, tag]
```

## Development

The plugin is written in Go and uses Microsoft [Teams Adaptive Cards](https://adaptivecards.io/designer/) for rich notifications. It supports customization through environment variables and plugin settings.

Inspired by [woodpecker-teams-notify-plugin](https://github.com/GECO-IT/woodpecker-plugin-teams-notify).