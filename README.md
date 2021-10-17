# concourse-github-pr-comment-resource

![resource-pipeline](https://github.com/nderjung/concourse-github-pr-comment-resource/workflows/resource-pipeline/badge.svg)

This Concourse resource monitors incoming comments on a Github Pull Request and
is able to monitor for comments matching regular expressions, match comment
author's association with the project, the pull request's state and any labels
it has been assigned.

Inspired by [`telia-oss/github-pr-resource`](https://github.com/telia-oss/github-pr-resource)
with the aim of providing a resource which reacts solely on the newest comment
to a particular pull request of a repository.

## Source configuration

The following parameters are used for the resource's `source` configuration:

| Parameter               | Required | Example                                     | Default                  | Description                                                                                                                                                                                                                                   |
| ----------------------- | -------- | ------------------------------------------- | ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `repository`            | Yes      | `nderjung/limp`                             |                          | The repository to listen for PR comments on.                                                                                                                                                                                                  |
| `disable_git_lfs`       | No       | `true`                                      | `false`                  | Disable Git LFS, skipping an attempt to convert pointers of files tracked into their corresponding objects when checked out into a working copy.                                                                                              |
| `access_token`          | Yes      |                                             |                          | The [personal access token](https://github.com/settings/tokens/new) of the account used to access, monitor and post comments on the repository in question.                                                                                   |
| `github_endpoint`       | No       |                                             | `https://api.github.com` | Endpoint used to connect to the Github v3 API.                                                                                                                                                                                                |
| `skip_ssl`              | No       | `true`                                      | `false`                  | Whether to skip SSL verification of the Github API.                                                                                                                                                                                           |
| `only_mergeable`        | No       | `true`                                      | `false`                  | Whether to react to (non-)mergeable pull requests.                                                                                                                                                                                            |
| `states`                | No       | `["closed"]`                                | `["open"]`               | The state of the pull request to react on.                                                                                                                                                                                                    |
| `ignore_states`         | No       | `["open"]`                                  | `[]`                     | The state of the pull request to not react on.                                                                                                                                                                                                |
| `labels`                | No       | `["bug"]`                                   | `[]`                     | The labels of the pull request to react on.                                                                                                                                                                                                   |
| `ignore_labels`         | No       | `["lifecycle/stale"]`                       | `[]`                     | The labels of the pull request not to react on.                                                                                                                                                                                               |
| `comments`              | No       | `["^ping$"]`                                | `[]`                     | The regular expressions of the latest comment to react on.                                                                                                                                                                                    |
| `commenter_association` | No       | `["first_time_contributor", "first_timer"]` | `["all"]`                | The comment author's relationship with the pull request's repository. Possible values include any of or any combination of `"collaborator"`, `"contributor"`, `"first_timer"`, `"first_time_contributor"`, `"member"`, `"owner"`, or `"all"`. |
| `ignore_comments`       | No       | `["ing$"]`                                  | `[]`                     | The regular expressions of the latest comment not to react on.                                                                                                                                                                                |
| `map_comment_meta`      | No       | `true`                                      | `false`                  | Whether to map any regular expression keys and their corresponding values to the meta object provided in `in`.                                                                                                                                |
| `review_states`         | No       | `["commented", "changes_requested"]`        | `[]`                     | The state of the review, any combination of `approved`, `changes_requeste` and/or `commented`.                                                                                                                                                | 
| `when`                  | No       | `first`                                     | `latest`                 | The comment or review to select, one of either `all`, `latest` or `first`.                                                                                                                                                                    | 

## Behaviour

### `check`

Produces new versions for all new comments to pull requests matching the
criteria set by the resource's `source` configuration.  The version provided to
Concourse is Github's unique numerical ID for the comment.

### `in`

The following parameters may be used in the `get` step of the resource:

| Parameter          | Required | Default       | Description                                                                  |
| ------------------ | -------- | ------------- | ---------------------------------------------------------------------------- |
| `comment_file`     | No       | `comment.txt` | A unique path to save the body of the comment.                               |
| `source_path`      | No       | `source`      | The path to save the source within the resource.                             |
| `git_depth`        | No       | `0`           | Git clone depth.                                                             |
| `submodules`       | No       | `false`       | Whether to clone Git submodules.                                             |
| `fetch_tags`       | No       | `false`       | Whether to fetch Git tags.                                                   |
| `integration_tool` | No       | `rebase`      | How to merge the PR source, selection between `rebase`, `merge`, `checkout`. |
| `skip_download`    | No       | `false`       | Does not clone the pull request.                                             |

The `in` procedure of this resource retrieves the following metadata about the
pull request comment and saves the key as the filename to the `path` set by the
resource.

| Key                  | Description                                                               |
| -------------------- | ------------------------------------------------------------------------- |
| `pr_id`              | The ID of the pull request relative to the repository.                    |
| `comment_id`         | The unique ID provided by Github for the comment.                         |
| `body`               | The content of the comment.                                               |
| `created_at`         | The [timestamp](https://golang.org/pkg/time/#Time.String) of the comment. |
| `updated_at`         | The timestamp of when the comment was last updated.                       |
| `author_association` | The association the author of the comment has with the repository.        |
| `html_url`           | The URL to the comment.                                                   |
| `user_id`            | The unique ID of the comment author on Github.                            |
| `user_login`         | The username of the comment author on Github.                             |
| `user_name`          | The name of the comment author on Github.                                 |
| `user_email`         | The email of the comment author on Github.                                |
| `user_avatar_url`    | The avatar URL for the comment author.                                    |
| `user_html_url`      | The URL to the comment author's profile on Github.                        |
| `pr_head_ref`        | The branch name from the HEAD of Pull Request.                            |
| `pr_head_sha`        | The commit SHA from the HEAD of the Pull Request.                         |
| `pr_base_ref`        | The branch name from the base of the Pull Request.                        |
| `pr_base_sha`        | The commit SHA from the base of the Pull Request.                         |

Additionally, the `in`/get step of this resource produces two additional JSON
formatted files which contain the information about the PR comment:

 * `version.json` which contains only contains the unique ID of the Github
   comment to the PR; and,
 * `metadata.json` which contains a serialized version of the table above,
 * Any additional attributes mapped from parsing comments using Golang's name
   grouping.  More details can be found [here](https://golang.org/pkg/regexp/syntax/).

### `out`

| Parameter             | Required | Example           | Default | Description                                                         |
| --------------------- | -------- | ----------------- | ------- | ------------------------------------------------------------------- |
| `path`                | Yes      | `pr-comment`      |         | The name given to the resource in a in/get step.                    |
| `state`               | No       | `closed`          |         | The state to set the PR.  Options include `open` and `closed`.      |
| `comment`             | No       | `pong`            |         | The string to use as a new comment on the PR.                       |
| `comment_file`        | No       | `pong.txt`        |         | The path to the file to read and post as a new comment on the PR.   |
| `labels`              | No       | `[""]`            |         | The finite set of labels to replace on the PR.                      |
| `add_labels`          | No       | `["cicd/tested"]` |         | Additional labels to add to the PR.                                 |
| `remove_labels`       | No       | `["cicd/await"]`  |         | Labels to remove from the PR.                                       |
| `delete_last_comment` | No       | `true`            | `false` | Whether or not to delete the last comment of the PR comment thread. |


Note that `comment` and `comment_file` will all expand all [Concourse environment variables](https://concourse-ci.org/implementing-resource-types.html#resource-metadata).

#### Notes

 * The author of the comment will be that of the user whose access token is used
   in the resource's `source` configuration.

## Example

The following represents a simple "ping-pong" setup, where Concourse is able to
use this resource to react to comments to a PR with the single term "ping" with
a comment "pong":

```yaml
resource_types:
  - name: github-pr-comment-resource
    type: docker-image
    source:
      repository: ndrjng/concourse-github-pr-comment-resource
      tag: latest

resources:
  - name: github-pr-comment-ping
    type: github-pr-comment-resource
    icon: magnify
    source:
      repository: nderjung/limp
      access_token: ((github.access-token))
      comments: ["^ping$"]

jobs:
  - name: pong
    serial: true
    plan:
      - get: github-pr-comment-ping
        trigger: true
        version: every

      - put: github-pr-comment-ping
        params:
          path: github-pr-comment-ping
          comment: "pong"
```

## License

BSD-3-Clause.  See [`LICENSE`](LICENSE).
