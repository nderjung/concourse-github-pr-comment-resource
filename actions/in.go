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
  "fmt"
  "time"
  "regexp"
  "strconv"
  "io/ioutil"
  "encoding/json"
  "path/filepath"

  "github.com/spf13/cobra"
  "github.com/nderjung/concourse-github-pr-comment-resource/api"
)

// InCmd
var InCmd = &cobra.Command{
  Use:                   "in [OPTIONS] PATH",
  Short:                 "Run the input parsing step",
  Run:                   doInCmd,
  Args:                  cobra.ExactArgs(1),
  DisableFlagsInUseLine: true,
}

// InParams are the parameters for configuring the input
type InParams struct {
  CommentFile     string `json:"comment_file"`
  SourcePath      string `json:"source_path"`
  GitDepth        int    `json:"git_depth"`
  Submodules      bool   `json:"submodules"`
  SkipDownload    bool   `json:"skip_download"`
  FetchTags       bool   `json:"fetch_tags"`
  IntegrationTool string `json:"integration_tool"`
}

// InRequest from the check stdin.
type InRequest struct {
  Source  Source   `json:"source"`
  Version Version  `json:"version"`
  Params  InParams `json:"params"`
}

// InResponse represents the structure Concourse expects on stdout
type InResponse struct {
  Version  Version  `json:"version"`
  Metadata Metadata `json:"metadata"`
}

type InMetadata struct {
  PRID              int       `json:"pr_id"`
  PRHeadRef         string    `json:"pr_head_ref"`
  PRHeadSHA         string    `json:"pr_head_sha"`
  PRBaseRef         string    `json:"pr_base_ref"`
  PRBaseSHA         string    `json:"pr_base_sha"`
  CommentID         int64     `json:"comment_id"`
  Body              string    `json:"body"`
  CreatedAt         time.Time `json:"created_at"`
  UpdatedAt         time.Time `json:"updated_at"`
  AuthorAssociation string    `json:"author_association"`
  HTMLURL           string    `json:"html_url"`
  UserLogin         string    `json:"user_login"`
  UserID            int64     `json:"user_id"`
  UserAvatarURL     string    `json:"user_avatar_url"`
  UserHTMLURL       string    `json:"user_html_url"`
}


func doInCmd(cmd *cobra.Command, args []string) {
  decoder := json.NewDecoder(os.Stdin)
  decoder.DisallowUnknownFields()
  
  // Concourse passes .json on stdin
  var req InRequest
  if err := decoder.Decode(&req); err != nil {
    logger.Fatal(err)
    return
  }
  
  // Perform the in command with the given request
  res, err := In(args[0], req)
  if err != nil {
    logger.Fatal(err)
    return
  }

  var encoder = json.NewEncoder(os.Stdout)

  // Generate a compatible Concourse output
  if err := doOutput(res, encoder, logger); err != nil {
    logger.Fatalf("Failed to encode to stdout: %s", err)
    return
  }
}

func In(outputDir string, req InRequest) (*InResponse, error) {
  client, err := api.NewGithubClient(
    req.Source.Repository,
    req.Source.AccessToken,
    req.Source.SkipSSLVerification,
    req.Source.GithubEndpoint,
  )
  if err != nil {
    return nil, err
  }

  prId, _ := strconv.ParseInt(req.Version.PrID, 10, 64)
  reviewId, _ := strconv.ParseInt(req.Version.ReviewID, 10, 64)
  commentId, _ := strconv.ParseInt(req.Version.CommentID, 10, 64)

  pull, err := client.GetPullRequest(int(prId))
  if err != nil {
    return nil, err
  }

  metadata := InMetadata{
    PRID:       int(prId),
    PRHeadRef: *pull.Head.Ref,
    PRHeadSHA: *pull.Head.SHA,
    PRBaseRef: *pull.Base.Ref,
    PRBaseSHA: *pull.Base.SHA,
  }

  // Write comment, version and metadata for reuse in PUT
  path := filepath.Join(outputDir)
  if err := os.MkdirAll(path, os.ModePerm); err != nil {
    return nil, fmt.Errorf("failed to create output directory: %s", err)
  }

  // Set the destination file to save the comment to
  commentFile := "comment.txt"
  if req.Params.CommentFile != "" {
    commentFile = req.Params.CommentFile
  }

  // Write the comment body to the specified path
  f, err := os.Create(filepath.Join(path, commentFile))
  if err != nil {
    return nil, fmt.Errorf("could not create comment file: %s", err)
  }

  defer f.Close()

  err = f.Truncate(0)
  if err != nil {
    return nil, err
  }

  var serialized Metadata

  if commentId > 0 {
    comment, err := client.GetPullRequestComment(commentId)
    if err != nil {
      return nil, fmt.Errorf("could not retrieve comment: %s", err)
    }

    metadata.CommentID = *comment.ID
    metadata.Body = *comment.Body
    metadata.CreatedAt = *comment.CreatedAt
    metadata.UpdatedAt = *comment.UpdatedAt
    metadata.AuthorAssociation = *comment.AuthorAssociation
    metadata.HTMLURL = *comment.HTMLURL
    metadata.UserLogin = *comment.User.Login
    metadata.UserID = *comment.User.ID
    metadata.UserAvatarURL = *comment.User.AvatarURL
    metadata.UserHTMLURL = *comment.User.HTMLURL
    
    serialized = serializeMetadata(metadata)

    if req.Source.MapCommentMeta {
      for _, commentStr := range req.Source.Comments {
        extraMeta := getParams(commentStr, *comment.Body)
  
        for k, v := range extraMeta {
          serialized.Add(k, v)
        }
      }
    }

    _, err = f.WriteString(*comment.Body)
    if err != nil {
      return nil, err
    }
  } else if reviewId > 0 && prId > 0 {
    review, err := client.GetPullRequestReview(
      int(prId),
      reviewId,
    )
    if err != nil {
      return nil, fmt.Errorf("could not retrieve review: %s", err)
    }
    
    metadata.CommentID = *review.ID
    metadata.Body = *review.Body
    metadata.CreatedAt = *review.SubmittedAt
    metadata.AuthorAssociation = *review.AuthorAssociation
    metadata.HTMLURL = *review.HTMLURL
    metadata.UserLogin = *review.User.Login
    metadata.UserID = *review.User.ID
    metadata.UserAvatarURL = *review.User.AvatarURL
    metadata.UserHTMLURL = *review.User.HTMLURL
    
    serialized = serializeMetadata(metadata)

    if req.Source.MapCommentMeta {
      for _, commentStr := range req.Source.Comments {
        extraMeta := getParams(commentStr, *review.Body)
  
        for k, v := range extraMeta {
          serialized.Add(k, v)
        }
      }
    }

    _, err = f.WriteString(*review.Body)
    if err != nil {
      return nil, err
    }
  } else {
    return nil, fmt.Errorf("cannot extrapolate version")
  }

  b, err := json.Marshal(req.Version)
  if err != nil {
    return nil, fmt.Errorf("failed to marshal version: %s", err)
  }

  if err := ioutil.WriteFile(filepath.Join(path, "version.json"), b, 0644); err != nil {
    return nil, fmt.Errorf("failed to write version: %s", err)
  }

  b, err = json.Marshal(serialized)
  if err != nil {
    return nil, fmt.Errorf("failed to marshal metadata: %s", err)
  }

  if err := ioutil.WriteFile(filepath.Join(path, "metadata.json"), b, 0644); err != nil {
    return nil, fmt.Errorf("failed to write metadata: %s", err)
  }

  // Save the individual metadata items to seperate files
  for _, d := range serialized {
    filename := d.Name
    content := []byte(d.Value)
    if err := ioutil.WriteFile(filepath.Join(path, filename), content, 0644); err != nil {
      return nil, fmt.Errorf("failed to write metadata file %s: %s", filename, err)
    }
  }

  if !req.Params.SkipDownload {
    // Set the destination path to save the HEAD of the PR
    sourcePath := "source"
    if req.Params.SourcePath != "" {
      sourcePath = req.Params.SourcePath
    }

    sourcePath = filepath.Join(path, sourcePath)
    if err := os.MkdirAll(sourcePath, os.ModePerm); err != nil {
      return nil, fmt.Errorf("failed to create source directory: %s", err)
    }

    git, err := api.NewGitClient(
      req.Source.AccessToken,
      req.Source.SkipSSLVerification,
      req.Source.DisableGitLfs,
      sourcePath,
      os.Stderr,
    )
    if err != nil {
      return nil, fmt.Errorf("failed to initialize git client: %s", err)
    }

    // Initialize and pull the base for the PR
    if err := git.Init(*pull.Base.Ref); err != nil {
      return nil, fmt.Errorf("failed to initialize git repo: %s", err)
    }

    if err := git.Pull(
      *pull.Base.Repo.GitURL,
      *pull.Base.Ref,
      req.Params.GitDepth,
      req.Params.Submodules,
      req.Params.FetchTags,
    ); err != nil {
      return nil, err
    }

    // Fetch the PR and merge the specified commit into the base
    if err := git.Fetch(
      *pull.Base.Repo.GitURL,
      *pull.Number,
      req.Params.GitDepth,
      req.Params.Submodules,
    ); err != nil {
      return nil, err
    }

    switch tool := req.Params.IntegrationTool; tool {
    case "rebase", "":
      if err := git.Rebase(
        *pull.Base.Ref,
        *pull.Head.SHA,
        req.Params.Submodules,
      ); err != nil {
        return nil, err
      }
    case "merge":
      if err := git.Merge(
        *pull.Head.SHA,
        req.Params.Submodules,
      ); err != nil {
        return nil, err
      }
    case "checkout":
      if err := git.Checkout(
        *pull.Head.Ref,
        *pull.Head.SHA,
        req.Params.Submodules,
      ); err != nil {
        return nil, err
      }
    default:
      return nil, fmt.Errorf("invalid integration tool specified: %s", tool)
    }
  }

  return &InResponse{
    Version:  req.Version,
    Metadata: serialized,
  }, nil
}

func getParams(regEx, comment string) (paramsMap map[string]string) {
  var compRegEx = regexp.MustCompile(regEx)
  match := compRegEx.FindStringSubmatch(comment)

  paramsMap = make(map[string]string)
  for i, name := range compRegEx.SubexpNames() {
    if i > 0 && i <= len(match) {
      paramsMap[name] = match[i]
    }
  }

  return
}
