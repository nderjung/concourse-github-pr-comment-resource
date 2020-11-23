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
  CommentFile string `json:"comment_file"`
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

  commentID, err := strconv.ParseInt(req.Version.Ref, 10, 64)
  if err != nil {
    return nil, err
  }
  comment, err := client.GetPullRequestComment(commentID)
  if err != nil {
    return nil, err
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

  _, err = f.WriteString(*comment.Body)
  if err != nil {
    return nil, err
  }

  // Retrieve the PR number from the given URL
  prNumber, err := api.ParseCommentHTMLURL(*comment.HTMLURL)
  if err != nil {
    return nil, err
  }

  metadata := serializeMetadata(InMetadata{
    PRID:              prNumber,
    CommentID:         *comment.ID,
    Body:              *comment.Body,
    CreatedAt:         *comment.CreatedAt,
    UpdatedAt:         *comment.UpdatedAt,
    AuthorAssociation: *comment.AuthorAssociation,
    HTMLURL:           *comment.HTMLURL,
    UserLogin:         *comment.User.Login,
    UserID:            *comment.User.ID,
    UserAvatarURL:     *comment.User.AvatarURL,
    UserHTMLURL:       *comment.User.HTMLURL,
  })

  if req.Source.MapCommentMeta {
    for _, commentStr := range req.Source.Comments {
      extraMeta := getParams(commentStr, *comment.Body)

      for k, v := range extraMeta {
        metadata.Add(k, v)
      }
    }
  }

  b, err := json.Marshal(req.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal version: %s", err)
  }

	if err := ioutil.WriteFile(filepath.Join(path, "version.json"), b, 0644); err != nil {
		return nil, fmt.Errorf("failed to write version: %s", err)
  }

	b, err = json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %s", err)
	}

  if err := ioutil.WriteFile(filepath.Join(path, "metadata.json"), b, 0644); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %s", err)
	}

  // Save the individual metadata items to seperate files
	for _, d := range metadata {
    filename := d.Name
		content := []byte(d.Value)
		if err := ioutil.WriteFile(filepath.Join(path, filename), content, 0644); err != nil {
			return nil, fmt.Errorf("failed to write metadata file %s: %s", filename, err)
		}
	}

  return &InResponse{
    Version:  req.Version,
    Metadata: metadata,
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
