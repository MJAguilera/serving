/*
Copyright 2020 The Knative Authors

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

// This file contains the load load balancing policies for Activator load balancing.

package net

import (
	"context"
	"math/rand"
)

// lbPolicy is a functor that selects a target pod from the list, or (noop, nil) if
// no such target can be currently acquired.
// Policies will presume that `targets` list is appropriately guarded by the caller,
// that is while podTrackers themselves can change during this call, the list
// and pointers therein are immutable.
type lbPolicy func(ctx context.Context, targets []*podTracker) (func(), *podTracker)

// randomLBPolicy is a load balancer policy that picks a random target.
// This approximates the LB policy done by K8s Service (IPTables based).
//
// nolint // This is currently unused but kept here for posterity.
func randomLBPolicy(_ context.Context, targets []*podTracker) (func(), *podTracker) {
	return noop, targets[rand.Intn(len(targets))]
}

// randomChoice2 implements the Power of 2 choices LB algorithm
func randomChoice2(_ context.Context, targets []*podTracker) (func(), *podTracker) {
	// Avoid random if possible.
	l := len(targets)
	// One tracker = no choice.
	if l == 1 {
		targets[0].addWeight(1)
		return func() {
			targets[0].addWeight(-1)
		}, targets[0]
	}
	r1, r2 := 0, 1
	// Two trackers - we know the both contestants,
	// otherwise pick 2 random unequal integers.
	if l > 2 {
		r1, r2 = rand.Intn(l), rand.Intn(l)
		for r1 == r2 {
			r2 = rand.Intn(l)
		}
	}

	pick, alt := targets[r1], targets[r2]
	// Possible race here, but this policy is for CC=0,
	// so fine.
	if pick.getWeight() > alt.getWeight() {
		pick = alt
	}
	pick.addWeight(1)
	return func() {
		pick.addWeight(-1)
	}, pick
}

// firstAvailableLBPolicy is a load balancer policy, that picks the first target
// that has capacity to serve the request right now.
func firstAvailableLBPolicy(ctx context.Context, targets []*podTracker) (func(), *podTracker) {
	for _, t := range targets {
		if cb, ok := t.Reserve(ctx); ok {
			return cb, t
		}
	}
	return noop, nil
}