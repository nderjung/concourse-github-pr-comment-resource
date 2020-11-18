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
package cmd

import (
  "os"
  "fmt"

  "github.com/spf13/cobra"
  "github.com/nderjung/concourse-github-pr-comment-resource/actions"
)


// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
  // Use:   "",
  Use:   "github-pr-comment [OPTIONS] [COMMANDS]",
  Short: `Concourse Resouce to act on Github PR comments.`,
  TraverseChildren: true,
  Run:    doRootCmd,
}

// Execute adds all child commands to the root command and sets flags
// appropriately.
func Execute() {
  if err := rootCmd.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}

func doRootCmd(cmd *cobra.Command, args []string) {
  printVersionFlag, err := cmd.PersistentFlags().GetBool("version")
  if err == nil && printVersionFlag {
    printVersion(cmd, args)
    os.Exit(0)
  }
}


func init() {
  rootCmd.PersistentFlags().BoolP("version", "v", false, "Display version number")
  
  rootCmd.AddCommand(actions.CheckCmd)
  rootCmd.AddCommand(actions.InCmd)
  rootCmd.AddCommand(actions.OutCmd)
}
