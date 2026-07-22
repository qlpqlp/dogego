// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package upnp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway1"
	"github.com/huin/goupnp/dcps/internetgateway2"
	natpmp "github.com/jackpal/go-nat-pmp"
)

const (
	leaseSeconds    = 3600
	mapDescription  = "DogeGo P2P"
	discoverTimeout = 4 * time.Second
)

// Result is the outcome of a port-mapping attempt.
type Result struct {
	OK         bool
	ExternalIP net.IP
	Port       int
	Method     string // "upnp-igd2", "upnp-igd1", "nat-pmp"
	Err        error

	igd2      *internetgateway2.WANIPConnection2
	igd1      *internetgateway1.WANIPConnection1
	natClient *natpmp.Client
}

type mappingState struct {
	method       string
	externalPort uint16
	internalPort uint16
	igd2         *internetgateway2.WANIPConnection2
	igd1         *internetgateway1.WANIPConnection1
	natClient    *natpmp.Client
}

var (
	stateMu sync.Mutex
	active  *mappingState
)

// Map attempts UPnP IGD2, then IGD1, then NAT-PMP. Replaces any prior mapping.
func Map(ctx context.Context, port int) Result {
	if port <= 0 || port > 65535 {
		return Result{Err: fmt.Errorf("upnp: invalid port %d", port)}
	}
	Unmap()
	localIP, err := localIPv4()
	if err != nil {
		return Result{Err: err}
	}
	internal := localIP.String()
	p := uint16(port)

	dctx, cancel := context.WithTimeout(ctx, discoverTimeout)
	defer cancel()

	if res := tryIGD2(dctx, internal, p); res.OK {
		remember(res)
		return res
	}
	if res := tryIGD1(dctx, internal, p); res.OK {
		remember(res)
		return res
	}
	if res := tryNATPMCP(dctx, p); res.OK {
		remember(res)
		return res
	}
	return Result{Err: fmt.Errorf("upnp: no IGD or NAT-PMP gateway found")}
}

func remember(res Result) {
	stateMu.Lock()
	defer stateMu.Unlock()
	active = &mappingState{
		method:       res.Method,
		externalPort: uint16(res.Port),
		internalPort: uint16(res.Port),
		igd2:         res.igd2,
		igd1:         res.igd1,
		natClient:    res.natClient,
	}
}

// Unmap removes the last successful mapping, if any.
func Unmap() {
	stateMu.Lock()
	st := active
	active = nil
	stateMu.Unlock()
	if st == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	switch st.method {
	case "upnp-igd2":
		if st.igd2 != nil {
			_ = st.igd2.DeletePortMappingCtx(ctx, "", st.externalPort, "TCP")
		}
	case "upnp-igd1":
		if st.igd1 != nil {
			_ = st.igd1.DeletePortMappingCtx(ctx, "", st.externalPort, "TCP")
		}
	case "nat-pmp":
		if st.natClient != nil {
			_, _ = st.natClient.AddPortMapping("tcp", int(st.internalPort), int(st.externalPort), 0)
		}
	}
}

func tryIGD2(ctx context.Context, internal string, port uint16) Result {
	clients, _, err := internetgateway2.NewWANIPConnection2ClientsCtx(ctx)
	if err != nil {
		return Result{Err: err}
	}
	for _, c := range clients {
		extStr, err := c.GetExternalIPAddressCtx(ctx)
		if err != nil {
			continue
		}
		extIP := net.ParseIP(extStr)
		if extIP == nil {
			continue
		}
		if err := c.AddPortMappingCtx(ctx, "", port, "TCP", port, internal, true, mapDescription, leaseSeconds); err != nil {
			continue
		}
		return Result{OK: true, ExternalIP: extIP, Port: int(port), Method: "upnp-igd2", igd2: c}
	}
	return Result{Err: fmt.Errorf("upnp-igd2: no suitable gateway")}
}

func tryIGD1(ctx context.Context, internal string, port uint16) Result {
	clients, _, err := internetgateway1.NewWANIPConnection1ClientsCtx(ctx)
	if err != nil {
		return Result{Err: err}
	}
	for _, c := range clients {
		extStr, err := c.GetExternalIPAddressCtx(ctx)
		if err != nil {
			continue
		}
		extIP := net.ParseIP(extStr)
		if extIP == nil {
			continue
		}
		if err := c.AddPortMappingCtx(ctx, "", port, "TCP", port, internal, true, mapDescription, leaseSeconds); err != nil {
			continue
		}
		return Result{OK: true, ExternalIP: extIP, Port: int(port), Method: "upnp-igd1", igd1: c}
	}
	return Result{Err: fmt.Errorf("upnp-igd1: no suitable gateway")}
}

func tryNATPMCP(ctx context.Context, port uint16) Result {
	localIP, err := localIPv4()
	if err != nil {
		return Result{Err: err}
	}
	gw := guessGateway(localIP)
	if gw == nil {
		return Result{Err: fmt.Errorf("nat-pmp: no gateway guess")}
	}
	client := natpmp.NewClientWithTimeout(gw, discoverTimeout)
	type extRes struct {
		r   *natpmp.GetExternalAddressResult
		err error
	}
	ch := make(chan extRes, 1)
	go func() {
		r, err := client.GetExternalAddress()
		ch <- extRes{r, err}
	}()
	var ext *natpmp.GetExternalAddressResult
	select {
	case <-ctx.Done():
		return Result{Err: ctx.Err()}
	case out := <-ch:
		if out.err != nil {
			return Result{Err: out.err}
		}
		ext = out.r
	}
	extIP := net.IP(ext.ExternalIPAddress[:])
	if _, err := client.AddPortMapping("tcp", int(port), int(port), leaseSeconds); err != nil {
		return Result{Err: err}
	}
	return Result{OK: true, ExternalIP: extIP, Port: int(port), Method: "nat-pmp", natClient: client}
}
