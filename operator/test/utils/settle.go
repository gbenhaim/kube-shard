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
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck,revive
	. "github.com/onsi/gomega"    //nolint:staticcheck,revive
)

const (
	// settleWindow is how long to observe a Ready APIShard before declaring
	// the reconciler idle. A hot loop produces a resourceVersion change on
	// nearly every poll; residual watches produce a handful.
	settleWindow = 30 * time.Second

	// settlePollInterval is how often to re-read this APIShard's
	// metadata.resourceVersion while waiting for the reconciler to go idle.
	settlePollInterval = time.Second

	// maxResourceVersionChanges is the maximum number of times this
	// APIShard's resourceVersion may change during settleWindow. Resource
	// versions are opaque, so we count object-specific updates rather than
	// subtracting revision numbers.
	maxResourceVersionChanges = 8
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
// by the operator. It polls this object's metadata.resourceVersion during
// settleWindow and requires the number of observed changes to stay below
// maxResourceVersionChanges.
func ExpectAPIShardSettled(name string) {
	By("asserting APIShard reconciler has settled")

	startRV := apiShardResourceVersion(name)
	lastRV := startRV
	changes := 0
	deadline := time.Now().Add(settleWindow)

	for time.Now().Before(deadline) {
		time.Sleep(settlePollInterval)
		rv := apiShardResourceVersion(name)
		if rv != lastRV {
			changes++
			lastRV = rv
		}
	}

	cmd := exec.Command("kubectl", "get", "apishard", name,
		"-o", "jsonpath={.status.phase}")
	phase, err := Run(cmd)
	Expect(err).NotTo(HaveOccurred())
	Expect(phase).To(Equal("Ready"), "APIShard %s left Ready during settle window", name)

	Expect(changes).To(BeNumerically("<", maxResourceVersionChanges),
		"APIShard %s resourceVersion changed %d times in %s (start %s, end %s); reconciler is hot-looping",
		name, changes, settleWindow, startRV, lastRV)
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
