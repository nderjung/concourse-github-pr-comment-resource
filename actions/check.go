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
package actions

import (
  "os"
  "sort"
  "strconv"
  "encoding/json"

  "github.com/spf13/cobra"
  "github.com/nderjung/concourse-github-pr-comment-resource/api"
)

// CheckCmd ...
var CheckCmd = &cobra.Command{
  Use:                   "check",
  Short:                 "Run the check step",
  Run:                   doCheckCmd,
  DisableFlagsInUseLine: true,
}

// CheckRequest from the check stdin.
type CheckRequest struct {
  Source  Source  `json:"source"`
  Version Version `json:"version"`
}

// CheckResponse represents the structure Concourse expects on stdout
type CheckResponse []Version

func doCheckCmd(cmd *cobra.Command, args []string) {
  decoder := json.NewDecoder(os.Stdin)
  decoder.DisallowUnknownFields()

  // Concourse passes .json on stdin
  var req CheckRequest
  if err := decoder.Decode(&req); err != nil {
    logger.Fatalf("Failed to decode to stdin: %s", err)
    return
  }

  // Perform the check with the given request
  res, err := Check(req)
  if err != nil {
    logger.Fatalf("Failed to connect to Github: %s", err)
    return
  }

  var encoder = json.NewEncoder(os.Stdout)

  // Generate a compatible Concourse output
  if err := doOutput(*res, encoder, logger); err != nil {
    logger.Fatalf("Failed to encode to stdout: %s", err)
    return
  }
}

func Check(req CheckRequest) (*CheckResponse, error) {
  client, err := api.NewGithubClient(
    req.Source.Repository,
    req.Source.AccessToken,
    req.Source.SkipSSLVerification,
    req.Source.GithubEndpoint,
  )
  if err != nil {
    return nil, err
  }

  if len(req.Source.When) == 0 {
    req.Source.When = "latest"
  }

  var versions CheckResponse
  var version *Version

  // Get all pull requests
  pulls, err := client.ListPullRequests()
  if err != nil {
    return nil, err
  }

  // Iterate over all pull requests
  for _, pull := range pulls {
    version = nil

    // Ignore if state not requested
    if !req.Source.requestsState(*pull.State) {
      continue
    }

    // Ignore if labels not requested
    if !req.Source.requestsLabels(pull.Labels) {
      continue
    }

    // Ignore if only mergeables requested
    if req.Source.OnlyMergeable && !*pull.Mergeable {
      continue
    }

    // Ignore drafts
    if *pull.Draft {
      continue
    }

    // Iterate through all the comments for this PR
    comments, err := client.ListPullRequestComments(int(*pull.Number))
    if err != nil {
      return nil, err
    }

    latestCommentIsMatch := false

    for _, comment := range comments {
      // Ignore comments which do not match comment author association
      if !req.Source.requestsCommenterAssociation(*comment.AuthorAssociation) {
        latestCommentIsMatch = false
        continue
      }

      // Ignore comments which do not match regex
      if !req.Source.requestsCommentRegex(*comment.Body) {
        latestCommentIsMatch = false
        continue
      }

      latestCommentIsMatch = true

      // Add the comment ID to the list of versions we want Concourse to see
      version = &Version{
        CreatedAt: strconv.FormatInt(comment.CreatedAt.Unix(), 10),
        PrID:      strconv.Itoa(*pull.Number),
        CommentID: strconv.FormatInt(*comment.ID, 10),
      }

      if req.Source.When == "all" || req.Source.When == "first" {
        versions = append(versions, *version)
      }

      // Break the loop now since we found the first match, causing the above
      // statement to be valid for only "all"
      if req.Source.When == "first" {
        break
      }
    }

    // Only save the latest
    if req.Source.When == "latest" && latestCommentIsMatch {
      versions = append(versions, *version)
    }

    // Iterate through all the reviews for this PR
    reviews, err := client.ListPullRequestReviews(int(*pull.Number))
    if err != nil {
      return nil, err
    }

    latestReviewIsMatch := false

    for _, review := range reviews {
      // Ignore reviews which do not approve the
      if !req.Source.requestsReviewState(*review.State) {
        latestReviewIsMatch = false
        continue
      }

      if !req.Source.requestsCommentRegex(*review.Body) {
        latestReviewIsMatch = false
        continue
      }

      latestReviewIsMatch = true

      // Add the comment ID to the list of versions we want Concourse to see
      version = &Version{
        CreatedAt: strconv.FormatInt(review.SubmittedAt.Unix(), 10),
        PrID:     strconv.Itoa(*pull.Number),
        ReviewID: strconv.FormatInt(*review.ID, 10),
      }

      if req.Source.When == "all" || req.Source.When == "first" {
        versions = append(versions, *version)
      }

      // Break the loop now since we found the first match, causing the above
      // statement to be valid for only "all"
      if req.Source.When == "first" {
        break
      }
    }

    // Only save the latest
    if req.Source.When == "latest" && latestReviewIsMatch {
      versions = append(versions, *version)
    }
  }

  sort.Slice(versions, func(i, j int) bool {
    return versions[i].CreatedAt < versions[j].CreatedAt
  })

  return &versions, nil
}
