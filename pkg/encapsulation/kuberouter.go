// Copyright 2019 the Kilo authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package encapsulation

import (
	"fmt"
	"net"
	"sync"

	"github.com/squat/kilo/pkg/iptables"
	"github.com/vishvananda/netlink"
)

const kubeRouterDeviceName = "kube-bridge"

type KubeRouter struct {
	iface    int
	strategy Strategy
	ch       chan netlink.LinkUpdate
	done     chan struct{}
	// mu guards updates to the iface field.
	mu sync.Mutex
}

// NewKubeRouter returns an encapsulator that uses kube-router.
func NewKubeRouter(strategy Strategy) Encapsulator {
	return &KubeRouter{
		ch:       make(chan netlink.LinkUpdate),
		done:     make(chan struct{}),
		strategy: strategy,
	}
}

// CleanUp is a no-op.
func (f *KubeRouter) CleanUp() error {
	close(f.done)
	return nil
}

// Gw returns the correct gateway IP associated with the given node.
func (f *KubeRouter) Gw(_, _ net.IP, subnet *net.IPNet) net.IP {
	return subnet.IP
}

// Index returns the index of the kube-router interface.
func (f *KubeRouter) Index() int {
	return f.iface
}

// Init finds the kubeRouter interface index.
func (f *KubeRouter) Init(_ int) error {
	if err := netlink.LinkSubscribe(f.ch, f.done); err != nil {
		return fmt.Errorf("failed to subscribe to updates to %s: %v", kubeRouterDeviceName, err)
	}
	go func() {
		var lu netlink.LinkUpdate
		for {
			select {
			case lu = <-f.ch:
				if lu.Attrs().Name == kubeRouterDeviceName {
					f.mu.Lock()
					f.iface = lu.Attrs().Index
					f.mu.Unlock()
				}
			case <-f.done:
				return
			}
		}
	}()
	i, err := netlink.LinkByName(kubeRouterDeviceName)
	if _, ok := err.(netlink.LinkNotFoundError); ok {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to query for kube-router interface: %v", err)
	}
	f.mu.Lock()
	f.iface = i.Attrs().Index
	f.mu.Unlock()
	return nil
}

// Rules is a no-op.
func (f *KubeRouter) Rules(_ []*net.IPNet) []iptables.Rule {
	return nil
}

// Set is a no-op.
func (f *KubeRouter) Set(_ *net.IPNet) error {
	return nil
}

// Strategy returns the configured strategy for encapsulation.
func (f *KubeRouter) Strategy() Strategy {
	return f.strategy
}
