// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@nderjung.net>
//
// Copyright (c) 2020, Alexander Jung.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.
package api

import (
  "fmt"
  "context"
  "strconv"
  "strings"
  "net/url"
  "net/http"
  "crypto/tls"

  "golang.org/x/oauth2"
  "github.com/google/go-github/v32/github"
)

// GithubClient containing the necessary information to authenticate and perform
// actions against the REST API.
type GithubClient struct {
  Owner      string
  Repository string
  Client     *github.Client
}

// Github interface representing the desired functions for this resource.
type Github interface {
  ListPullRequests() ([]*github.PullRequest, error)
  GetPullRequest(prID int) (*github.PullRequest, error)
  ListPullRequestComments(prID int) ([]*github.IssueComment, error)
  GetPullRequestComment(commentID int64) (*github.IssueComment, error)
  SetPullRequestState(prID int, state string) error
  DeleteLastPullRequestComment(prID int) error
  AddPullRequestLabels(prID int, labels []string) error
  RemovePullRequestLabels(prID int, labels []string) error
  ReplacePullRequestLabels(prID int, labels []string) error
  CreatePullRequestComment(prID int, comment string) error
}

// NewGitHubClient for creating a new instance of the client.
func NewGithubClient(repo string, accessToken string, skipSSL bool, githubEndpoint string) (*GithubClient, error) {
  owner, repository, err := parseRepository(repo)
  if err != nil {
    return nil, err
  }

  var ctx context.Context

  if skipSSL {
    insecureClient := &http.Client{
      Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
          InsecureSkipVerify: true,
        },
      },
    }

    ctx = context.WithValue(context.TODO(), oauth2.HTTPClient, insecureClient)
  } else {
    ctx = context.TODO()
  }

  var client *github.Client
  oauth2Client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(
    &oauth2.Token{
      AccessToken: accessToken,
    },
  ))
  
  if githubEndpoint != "" {
    endpoint, err := url.Parse(githubEndpoint)
    if err != nil {
      return nil, fmt.Errorf("failed to parse v3 endpoint: %s", err)
    }
    
    client, err = github.NewEnterpriseClient(endpoint.String(), endpoint.String(), oauth2Client)
    if err != nil {
      return nil, err
    }
  } else {
    client = github.NewClient(oauth2Client)
  }

  return &GithubClient{
    Owner:      owner,
    Repository: repository,
    Client:     client,
  }, nil
}

// ListPullRequests returns the list of pull requests for the configured repo
func (c *GithubClient) ListPullRequests() ([]*github.PullRequest, error) {
  pulls, _, err := c.Client.PullRequests.List(
    context.TODO(), 
    c.Owner,
    c.Repository,
    &github.PullRequestListOptions{
      // We want all states so we can sort through them later
      State: "all",
    },
  )
  if err != nil {
    return nil, err
  }

  return pulls, nil
}

// GetPullRequest returns the specific pull request given its ID relative to the
// configured repo
func (c *GithubClient) GetPullRequest(prID int) (*github.PullRequest, error) {
  pull, _, err := c.Client.PullRequests.Get(
    context.TODO(),
    c.Owner,
    c.Repository,
    prID,
  )
  if err != nil {
    return nil, err
  }
  return pull, nil
}

// ListPullRequestComments returns the list of comments for the specific pull
// request given its ID relative to the configured repo
func (c *GithubClient) ListPullRequestComments(prID int) ([]*github.IssueComment, error) {
  comments, _, err := c.Client.Issues.ListComments(
    context.TODO(),
    c.Owner,
    c.Repository,
    prID,
    &github.IssueListCommentsOptions{},
  )
  if err != nil {
    return nil, err
  }
  
  return comments, nil
}

// GetPulLRequestComment returns the specific comment given its unique Github ID
func (c *GithubClient) GetPullRequestComment(commentID int64) (*github.IssueComment, error) {
  comment, _, err := c.Client.Issues.GetComment(
    context.TODO(),
    c.Owner,
    c.Repository,
    commentID,
  )
  if err != nil {
    return nil, err
  }
  
  return comment, nil
}

func (c *GithubClient) SetPullRequestState(prID int, state string) error {
  validState := false
  validStates := []string{"open", "closed"}
  for _, s := range validStates {
    if state == s {
      validState = true
    }
  }

  if !validState {
    return fmt.Errorf("invalid pull request state: %s", state)
  }

  _, _, err := c.Client.Issues.Edit(
    context.TODO(),
    c.Owner,
    c.Repository,
    prID, &github.IssueRequest{
      State: &state,
    },
  )

  return err
}

func (c *GithubClient) DeleteLastPullRequestComment(prID int) error {
  comments, err := c.ListPullRequestComments(prID)
  if err != nil {
    return err
  }

  // Retrieve the authenticated user provided by the access token
  user, _, err := c.Client.Users.Get(
    context.TODO(),
    "",
  )
  if err != nil {
    return err
  }

  // Only delete the last comment from the same author as the provided token
  var commentID int64
  for _, comment := range comments {
    fmt.Print(*comment.User.ID)
    if *comment.User.ID == *user.ID {
      commentID = *comment.ID
    }
  }

  if commentID > 0 {
    _, err = c.Client.Issues.DeleteComment(
      context.TODO(),
      c.Owner,
      c.Repository,
      commentID,
    )

    return err
  }

  return nil
}

// AddPullRequestLabels adds the list of labels to the existing set of labels
// given the relative pull request ID to the configure repo
func (c *GithubClient) AddPullRequestLabels(prID int, labels []string) error {
  _, _, err := c.Client.Issues.AddLabelsToIssue(
    context.TODO(),
    c.Owner,
    c.Repository,
    prID,
    labels,
  )

  return err
}

// RemovePullRequestLabels remove the list of labels from the set of existing
// labels given the relative pull request ID to the configured repo
func (c *GithubClient) RemovePullRequestLabels(prID int, labels []string) error {
  for _, l := range labels {
    _, err := c.Client.Issues.RemoveLabelForIssue(
      context.TODO(),
      c.Owner,
      c.Repository,
      prID,
      l,
    )
    
    if err != nil {
      return err
    }
  }

  return nil
}

// ReplacePullRequestLabels overrides all existing labels with the given set of
// labels for the pull request ID relative to the configured repo
func (c *GithubClient) ReplacePullRequestLabels(prID int, labels []string) error {
  _, _, err := c.Client.Issues.ReplaceLabelsForIssue(
    context.TODO(),
    c.Owner,
    c.Repository,
    prID,
    labels,
  )

  return err
}

// CreatePullRequestComment adds a new comment to the pull request given its
// ID relative to the configured repo
func (c *GithubClient) CreatePullRequestComment(prID int, comment string) error {
  _, _, err := c.Client.Issues.CreateComment(
    context.TODO(),
    c.Owner,
    c.Repository,
    prID,
    &github.IssueComment{
      Body: &comment,
    },
  )
  return err
}

func parseRepository(s string) (string, string, error) {
  parts := strings.Split(s, "/")
  if len(parts) != 2 {
    return "", "", fmt.Errorf("malformed repository")
  }
  return parts[0], parts[1], nil
}

// ParseCommentHTMLURL takes in a standard issue URL and returns the issue 
// number, e.g.:
// https://github.com/octocat/Hello-World/issues/1347#issuecomment-1
func ParseCommentHTMLURL(prUrl string) (int, error) {
  u, err := url.Parse(prUrl)
  if err != nil {
    return -1, err
  }

  parts := strings.Split(u.Path, "/")
  i, err := strconv.Atoi(parts[len(parts)-1])
  if err != nil {
    return -1, err
  }

  return i, nil
}
