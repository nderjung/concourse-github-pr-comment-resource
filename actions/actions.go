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
  "log"
  "regexp"
  "strings"
  "reflect"
  "encoding/json"

  "github.com/google/go-github/v32/github"
)

// Source parameters provided by the resource.
type Source struct {
  // Meta
  SkipSSLVerification    bool   `json:"skip_ssl"`
  GithubEndpoint         string `json:"github_endpoint"`

  // The repository to interface with
  Repository             string `json:"repository"`
  DisableGitLfs          bool   `json:"disable_git_lfs"`

  // Access methods
  AccessToken            string `json:"access_token"`
  Username               string `json:"username"`
  Password               string `json:"password"`

  // Selection criteria
  OnlyMergeable          bool   `json:"only_mergeable"`
  States               []string `json:"states"`
  Labels               []string `json:"labels"`
  Comments             []string `json:"comments"`
  CommenterAssociation []string `json:"commenter_association"`
  MapCommentMeta         bool   `json:"map_comment_meta"`
  ReviewStates         []string `json:"review_states"`
  When                   string `json:"when"` // all, latest, first

  IgnoreStates         []string `json:"ignore_states"`
  IgnoreLabels         []string `json:"ignore_labels"`
  IgnoreComments       []string `json:"ignore_comments"`
  IgnoreDrafts           bool   `json:"ignore_drafts"`
}

// Version communicated with Concourse.
type Version struct {
  CreatedAt string `json:"created_at"`
  PrID      string `json:"pr_id"`
  ReviewID  string `json:"review_id"`
  CommentID string `json:"comment_id"`
}

// Metadata has a key name and value
type MetadataField struct {
  Name  string `json:"name"`
  Value string `json:"value"`
}

// Metadata contains the serialized interface
type Metadata []*MetadataField

// Add a MetadataField to the Metadata struct
func (m *Metadata) Add(name, value string) {
  *m = append(*m, &MetadataField{
    Name: name,
    Value: value,
  })
}

// Get a MetadataField value from the Metadata struct
func (m *Metadata) Get(name string) (string, error) {
  for _, i := range *m {
    if name == i.Name {
      return i.Value, nil
    }
  }

  return "", fmt.Errorf("metadata index does not exist: %s", name)
}

func serializeMetadata(meta interface{}) Metadata {
  var res Metadata
  v := reflect.ValueOf(meta)
  typeOfS := v.Type()

  for i := 0; i< v.NumField(); i++ {
    res.Add(
      typeOfS.Field(i).Tag.Get("json"),
      fmt.Sprintf("%v", v.Field(i).Interface()),
    )
  }

  return res
}

// requestsState checks whether the source requests this particular state
func (source *Source) requestsState(state string) bool {
  ret := false

  // if there are no set states, assume only "open" states
  if len(source.States) == 0 {
    ret = state == "open"
  } else {
    for _, s := range source.States {
      if s == state {
        ret = true
        break
      }
    }
  }

  // Ensure ignored states
  for _, s := range source.IgnoreStates {
    if s == state {
      ret = false
      break
    }
  }

  return ret
}

// requestsReviewState checks whether the PR review matches the desired state
func (source *Source) requestsReviewState(state string) bool {
  state = strings.ToLower(state)
  for _, s := range source.ReviewStates {
    if state == strings.ToLower(s) {
      return true
    }
  }

  return false
}

// requestsLabels checks whether the source requests these set of labels
func (source *Source) requestsLabels(labels []*github.Label) bool {
  ret := false

  // If no set labels, assume all
  if len(source.Labels) == 0 {
    ret = true
  } else {
    includeLoop:
    for _, rl := range source.Labels {
      for _, rr := range labels {
        if rl == rr.GetName() {
          ret = true
          break includeLoop
        }
      }
    }
  }

  excludeLoop:
  for _, rl := range source.IgnoreLabels {
    for _, rr := range labels {
      if rl == rr.GetName() {
        ret = false
        break excludeLoop
      }
    }
  }

  return ret
}

// requestsCommenterAssociation checks the comment author's association
func (source *Source) requestsCommenterAssociation(assoc string) bool {
  // if no associations set, assume all
  if len(source.CommenterAssociation) == 0 || (
      len(source.CommenterAssociation) == 1 &&
      source.CommenterAssociation[0] == "all") {
    return true
  }

  assoc = strings.ToLower(assoc)
  for _, a := range source.CommenterAssociation {
    if assoc == strings.ToLower(a) {
      return true
    }
  }

  return false
}

// requestsCommentRegex determines if the source requests this comment regex
func (source *Source) requestsCommentRegex(comment string) bool {
  ret := false

  if len(source.Comments) == 0 {
    ret = true
  } else {
    for _, c := range source.Comments {
      matched, _ := regexp.Match(c, []byte(comment))
      if matched {
        ret = true
      }
    }
  }

  for _, c := range source.IgnoreComments {
    matched, _ := regexp.Match(c, []byte(comment))
    if matched {
      ret = false
    }
  }

  return ret
}

var logger = log.New(os.Stderr, "resource:", log.Lshortfile)

// doOutput ...
func doOutput(output interface{}, encoder *json.Encoder, logger *log.Logger) error {
  _, err := json.MarshalIndent(output, "", "  ")
  if err != nil {
    return err
  }

  // encode output to stdout
  return encoder.Encode(output)
}
