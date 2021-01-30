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
  
  var versions CheckResponse

  // Get all pull requests
  pulls, err := client.ListPullRequests()
  if err != nil {
    return nil, err
  }

  // Iterate over all pull requests
  for _, pull := range pulls {
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

    for _, comment := range comments {
      if !req.Source.requestsCommenterAssociation(*comment.AuthorAssociation) {
        continue
      }

      if !req.Source.requestsCommentRegex(*comment.Body) {
        continue
      }

      // Retrieve the PR number from the given URL
      prID, err := api.ParseCommentHTMLURL(*comment.HTMLURL)
      if err != nil {
        return nil, err
      }

      // Add the comment ID to the list of versions we want Concourse to see
      versions = append(versions, Version{
        PrID:      strconv.Itoa(prID),
        CommentID: strconv.FormatInt(*comment.ID, 10),
      })
    }

    // Iterate through all the reviews for this PR
    reviews, err := client.ListPullRequestReviews(int(*pull.Number))
    if err != nil {
      return nil, err
    }

    for _, review := range reviews {
      if !req.Source.requestsCommenterAssociation(*review.AuthorAssociation) {
        continue
      }

      // Retrieve the PR number from the given URL
      prID, err := api.ParseCommentHTMLURL(*review.HTMLURL)
      if err != nil {
        return nil, err
      }
      
      if !req.Source.requestsCommentRegex(*review.Body) {
        continue
      }

      // Add the comment ID to the list of versions we want Concourse to see
      versions = append(versions, Version{
        PrID:     strconv.Itoa(prID),
        ReviewID: strconv.FormatInt(*review.ID, 10),
      })
    }
  }

  return &versions, nil
}
