/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"os/exec"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck,revive
	. "github.com/onsi/gomega"
)

const (
	// settleWindow is how long to observe a Ready APIShard before declaring
	// the reconciler idle. A hot loop produces hundreds of resourceVersion
	// bumps in this window; residual watches produce a handful.
	settleWindow = 30 * time.Second

	// maxResourceVersionDelta is the maximum metadata.resourceVersion increase
	// allowed during settleWindow. Every successful reconcile writes APIShard
	// status, so a tight loop far exceeds this bound.
	maxResourceVersionDelta int64 = 8
)

// WaitForAPIShardReady waits until the named APIShard reports phase Ready,
// then asserts the apishard reconciler has settled.
func WaitForAPIShardReady(name string, timeout time.Duration) {
	By("waiting for APIShard to become Ready")
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "apishard", name,
			"-o", "jsonpath={.status.phase}")
		output, err := Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		if output != "Ready" {
			condCmd := exec.Command("kubectl", "get", "apishard", name,
				"-o", "jsonpath={.status.conditions}")
			condOut, _ := Run(condCmd)
			g.Expect(output).To(Equal("Ready"),
				"phase=%s conditions=%s", output, condOut)
		}
	}, timeout, 10*time.Second).Should(Succeed())

	ExpectAPIShardSettled(name)
}

// ExpectAPIShardSettled asserts that a Ready APIShard is not being hot-looped
// by the operator. It samples metadata.resourceVersion, waits settleWindow,
// and requires the integer delta to stay below maxResourceVersionDelta.
func ExpectAPIShardSettled(name string) {
	By("asserting APIShard reconciler has settled")

	startRV := apiShardResourceVersion(name)
	startN, err := strconv.ParseInt(startRV, 10, 64)
	Expect(err).NotTo(HaveOccurred(), "parsing start resourceVersion %q", startRV)

	time.Sleep(settleWindow)

	cmd := exec.Command("kubectl", "get", "apishard", name,
		"-o", "jsonpath={.status.phase}")
	phase, err := Run(cmd)
	Expect(err).NotTo(HaveOccurred())
	Expect(phase).To(Equal("Ready"), "APIShard %s left Ready during settle window", name)

	endRV := apiShardResourceVersion(name)
	endN, err := strconv.ParseInt(endRV, 10, 64)
	Expect(err).NotTo(HaveOccurred(), "parsing end resourceVersion %q", endRV)

	delta := endN - startN
	Expect(delta).To(BeNumerically("<", maxResourceVersionDelta),
		"APIShard %s resourceVersion moved from %s to %s in %s (delta %d); reconciler is hot-looping",
		name, startRV, endRV, settleWindow, delta)
}

// apiShardResourceVersion returns metadata.resourceVersion for the named APIShard.
func apiShardResourceVersion(name string) string {
	cmd := exec.Command("kubectl", "get", "apishard", name,
		"-o", "jsonpath={.metadata.resourceVersion}")
	output, err := Run(cmd)
	Expect(err).NotTo(HaveOccurred())
	Expect(output).NotTo(BeEmpty(), "APIShard %s resourceVersion", name)
	return output
}
