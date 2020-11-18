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
  "strconv"
  "strings"
  "io/ioutil"
  "encoding/json"
  "path/filepath"

  "github.com/spf13/cobra"
  "github.com/nderjung/concourse-github-pr-comment-resource/api"
)

// OutCmd
var OutCmd = &cobra.Command{
  Use:                   "out [OPTIONS] PATH",
  Short:                 "Run the output processing step",
  Run:                   doOutCmd,
  Args:                  cobra.ExactArgs(1),
  DisableFlagsInUseLine: true,
}

type OutParams struct {
  Path                string `json:"path"`
  State               string `json:"state"`
  Comment             string `json:"comment"`
  CommentFile         string `json:"comment_file"`
  Labels            []string `json:"labels"`
  AddLabels         []string `json:"add_labels"`
  RemoveLabels      []string `json:"remove_labels"`
  DeleteLastComment   bool   `json:"delete_last_comment"`
}

func (p *OutParams) Validate() error {
  if p.State == "" {
    return nil
  }

  // Make sure we are setting an allowed state
  var allowedState bool

  state := strings.ToLower(p.State)
  allowed := []string{"open", "closed"}

  for _, a := range allowed {
    if state == a {
      allowedState = true
    }
  }

  if !allowedState {
    return fmt.Errorf("unknown state: %s", p.State)
  }

  return nil
}

// OutRequest from the check stdin.
type OutRequest struct {
  Source Source    `json:"source"`
  Params OutParams `json:"params"`
}

// OutResponse represents the structure Concourse expects on stdout
type OutResponse struct {
  Version  Version  `json:"version"`
  Metadata Metadata `json:"metadata"`
}

func doOutCmd(cmd *cobra.Command, args []string) {
  decoder := json.NewDecoder(os.Stdin)
  decoder.DisallowUnknownFields()
  
  // Concourse passes .json on stdin
  var req OutRequest
  if err := decoder.Decode(&req); err != nil {
    logger.Fatalf("Failed to decode to stdin: %s", err)
    return
  }
  
  // Perform the out command with the given request
  res, err := Out(args[0], req)
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

func Out(inputDir string, req OutRequest) (*OutResponse, error) {
  if err := req.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid parameters: %s", err)
  }

  path := filepath.Join(inputDir, req.Params.Path)

	// Version available after a GET step.
	var version Version
	content, err := ioutil.ReadFile(filepath.Join(path, "version.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read version from path: %s", err)
	}
	if err := json.Unmarshal(content, &version); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version from file: %s", err)
  }

	// Metadata available after a GET step.
	var metadata Metadata
	content, err = ioutil.ReadFile(filepath.Join(path, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata from path: %s", err)
	}
	if err := json.Unmarshal(content, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata from file: %s", err)
  }
  
  prNumber, err := metadata.Get("pr_id")
  if err != nil {
    return nil, err
  }

  prID, err := strconv.Atoi(prNumber)
  if err != nil {
    return nil, err
  }

  client, err := api.NewGithubClient(
    req.Source.Repository,
    req.Source.AccessToken,
    req.Source.SkipSSLVerification,
    req.Source.GithubEndpoint,
  )
  if err != nil {
    return nil, err
  }

  // Update the state?
  if req.Params.State != "" {
    err = client.SetPullRequestState(prID, req.Params.State)
    if err != nil {
      return nil, err
    }
  }

  // Delete the last comment?
  if req.Params.DeleteLastComment {
    err = client.DeleteLastPullRequestComment(prID)
    if err != nil {
      return nil, err
    }
  }

  // Add, remove or replace tags?
  if len(req.Params.Labels) > 0 {
    err = client.ReplacePullRequestLabels(prID, req.Params.Labels)
    if err != nil {
      return nil, err
    }
  } else {
    if len(req.Params.AddLabels) > 0 {
      err = client.AddPullRequestLabels(prID, req.Params.AddLabels)
      if err != nil {
        return nil, err
      }
    }
    if len(req.Params.RemoveLabels) > 0 {
      err = client.RemovePullRequestLabels(prID, req.Params.RemoveLabels)
      if err != nil {
        return nil, err
      }
    }
  }

  // Add a new comment?
  var comment string
  if len(req.Params.Comment) > 0 {
    comment = req.Params.Comment
  } else if len(req.Params.CommentFile) > 0 {
    b, err := ioutil.ReadFile(filepath.Join(path, req.Params.CommentFile))
    if err != nil {
      return nil, err
    }
    comment = string(b)
  }

  if len(comment) > 0 {
    err = client.CreatePullRequestComment(prID, safeExpandEnv(comment))
    if err != nil {
      return nil, err
    }
  }

  return &OutResponse{
    Version:  version,
    Metadata: metadata,
  }, nil
}

func safeExpandEnv(s string) string {
	return os.Expand(s, func(v string) string {
		switch v {
    case "BUILD_ID",
         "BUILD_NAME",
         "BUILD_JOB_NAME",
         "BUILD_PIPELINE_NAME",
         "BUILD_TEAM_NAME",
         "ATC_EXTERNAL_URL":
			return os.Getenv(v)
    }
    
		return "$" + v
	})
}
